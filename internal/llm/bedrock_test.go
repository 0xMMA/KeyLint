package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go/auth/bearer"
)

// bedrockReply is a Bedrock InvokeModel answer for an Anthropic model: the
// Messages API body, which Bedrock passes through.
const bedrockReply = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-haiku-5-5","content":[{"type":"text","text":"fixed text"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":2}}`

// isolateAWS points every AWS SDK input at nothing: empty config and
// credential files in a temp dir, a temp home (where the SSO token cache
// lives), no credential or profile variables, and no EC2 metadata lookup.
// Nothing a test does can read the developer's real ~/.aws.
func isolateAWS(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	creds := filepath.Join(dir, "credentials")
	for _, f := range []string{config, creds} {
		if err := os.WriteFile(f, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("AWS_CONFIG_FILE", config)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", creds)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, v := range []string{
		"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN", "AWS_BEARER_TOKEN_BEDROCK", "AWS_REGION", "AWS_DEFAULT_REGION",
		"AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE", "AWS_CONTAINER_CREDENTIALS_FULL_URI",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_ENDPOINT_URL", "AWS_ENDPOINT_URL_BEDROCK_RUNTIME",
	} {
		t.Setenv(v, "")
	}
	return dir
}

// writeAWSFile fills one of isolateAWS's files.
func writeAWSFile(t *testing.T, env, content string) {
	t.Helper()
	if err := os.WriteFile(os.Getenv(env), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func keyModeConfig(baseURL string) Config {
	return Config{
		APIKey:  "bedrock-key-from-keyring",
		BaseURL: baseURL,
		Bedrock: BedrockConfig{Region: "eu-central-1", Auth: BedrockAuthKey},
	}
}

func signInConfig(baseURL string) Config {
	return Config{
		BaseURL: baseURL,
		Bedrock: BedrockConfig{Region: "eu-central-1", Auth: BedrockAuthSignIn},
	}
}

func TestBedrockKeyModeRequestShape(t *testing.T) {
	isolateAWS(t)
	var got capture
	srv := newServer(t, &got, http.StatusOK, bedrockReply)

	resp, err := newBedrock(keyModeConfig(srv.URL)).Complete(context.Background(), Request{
		System:    "system prompt",
		User:      "user message",
		Model:     "anthropic.claude-haiku-5-5",
		MaxTokens: 2048,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "fixed text" {
		t.Errorf("Text = %q", resp.Text)
	}
	if resp.Model != "claude-haiku-5-5" {
		t.Errorf("Model = %q, want the model Bedrock reported", resp.Model)
	}

	// bedrock-runtime InvokeModel, through the EU profile for Frankfurt.
	if got.path != "/model/eu.anthropic.claude-haiku-5-5/invoke" {
		t.Errorf("path = %q, want /model/eu.anthropic.claude-haiku-5-5/invoke", got.path)
	}
	if auth := got.headers.Get("Authorization"); auth != "Bearer bedrock-key-from-keyring" {
		t.Errorf("Authorization = %q, want the Bedrock API key as a bearer token", auth)
	}
	if key := got.headers.Get("x-api-key"); key != "" {
		t.Errorf("x-api-key = %q: an Anthropic key header has no place on Bedrock", key)
	}
	if got.body["anthropic_version"] != "bedrock-2023-05-31" {
		t.Errorf("anthropic_version = %v", got.body["anthropic_version"])
	}
	for _, field := range []string{"model", "stream"} {
		if _, ok := got.body[field]; ok {
			t.Errorf("%s is in the body; Bedrock takes the model in the path", field)
		}
	}
	if got.body["max_tokens"] != float64(2048) {
		t.Errorf("max_tokens = %v", got.body["max_tokens"])
	}
	if text := textBlocks(t, got.body["system"]); text != "system prompt" {
		t.Errorf("system = %v", got.body["system"])
	}
	if ua := got.headers.Get("User-Agent"); ua != userAgent {
		t.Errorf("User-Agent = %q, want %q", ua, userAgent)
	}
	for _, header := range fingerprintHeaders {
		if header != "User-Agent" && got.headers.Get(header) != "" {
			t.Errorf("%s reached Bedrock", header)
		}
	}
}

// TestBedrockKeyModeNeverReadsAWSConfig: a user who chose an API key has not
// handed KeyLint their AWS sign-in, so ~/.aws is not read. The trap is a
// profile named in the environment that does not exist: loading the AWS
// config fails on it. Sign-in mode trips it, which proves it is armed; key
// mode must not.
func TestBedrockKeyModeNeverReadsAWSConfig(t *testing.T) {
	isolateAWS(t)
	t.Setenv("AWS_PROFILE", "keylint-trap-profile")
	var got capture
	srv := newServer(t, &got, http.StatusOK, bedrockReply)
	req := Request{User: "x", Model: "anthropic.claude-haiku-5-5", MaxTokens: 64}

	loads := 0
	original := loadAWSConfig
	t.Cleanup(func() { loadAWSConfig = original })
	loadAWSConfig = func(ctx context.Context, opts ...func(*config.LoadOptions) error) (aws.Config, error) {
		loads++
		return original(ctx, opts...)
	}

	if _, err := newBedrock(keyModeConfig(srv.URL)).Complete(context.Background(), req); err != nil {
		t.Fatalf("key mode: %v", err)
	}
	if loads != 0 {
		t.Errorf("key mode loaded the AWS config %d time(s)", loads)
	}

	_, err := newBedrock(signInConfig(srv.URL)).Complete(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "keylint-trap-profile") {
		t.Fatalf("sign-in mode should trip the trap profile, got %v", err)
	}
	if !errors.Is(err, ErrAWSSignIn) {
		t.Errorf("a missing profile should be an AWS sign-in problem: %v", err)
	}
	if got.hits != 1 {
		t.Errorf("hits = %d, want only the key-mode request", got.hits)
	}
}

func TestBedrockSignInModeSignsWithSigV4(t *testing.T) {
	isolateAWS(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDKEYLINTTEST")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	var got capture
	srv := newServer(t, &got, http.StatusOK, bedrockReply)

	if _, err := newBedrock(signInConfig(srv.URL)).Complete(context.Background(), Request{
		User: "x", Model: "anthropic.claude-haiku-5-5", MaxTokens: 64,
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	auth := got.headers.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=AKIDKEYLINTTEST/") ||
		!strings.Contains(auth, "/eu-central-1/bedrock/aws4_request") {
		t.Errorf("Authorization = %q, want a SigV4 signature for bedrock in eu-central-1", auth)
	}
	if got.headers.Get("X-Amz-Date") == "" {
		t.Error("X-Amz-Date missing: the request was not signed")
	}
	if strings.Contains(auth, "test-secret") {
		t.Error("the secret key itself went on the wire")
	}
}

func TestBedrockSignInModeUsesTheNamedProfile(t *testing.T) {
	isolateAWS(t)
	writeAWSFile(t, "AWS_SHARED_CREDENTIALS_FILE", "[default]\naws_access_key_id = AKIDDEFAULT\naws_secret_access_key = s1\n\n[work]\naws_access_key_id = AKIDWORKPROFILE\naws_secret_access_key = s2\n")
	var got capture
	srv := newServer(t, &got, http.StatusOK, bedrockReply)

	cfg := signInConfig(srv.URL)
	cfg.Bedrock.Profile = "work"
	if _, err := newBedrock(cfg).Complete(context.Background(), Request{User: "x", Model: "anthropic.claude-haiku-5-5", MaxTokens: 64}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if auth := got.headers.Get("Authorization"); !strings.Contains(auth, "Credential=AKIDWORKPROFILE/") {
		t.Errorf("Authorization = %q, want the work profile's key", auth)
	}
}

// TestBedrockEnvironmentTokenWins documents the decision for
// AWS_BEARER_TOKEN_BEDROCK: it wins in both modes. In key mode that is
// KeyLint's env-over-keyring rule; in sign-in mode it is what AWS's own SDKs
// do with it for Bedrock, and the card says so. Either way ~/.aws is not read.
func TestBedrockEnvironmentTokenWins(t *testing.T) {
	isolateAWS(t)
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "token-from-environment")
	t.Setenv("AWS_PROFILE", "keylint-trap-profile")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDSHOULDNOTSIGN")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "s")

	for name, cfg := range map[string]Config{"key mode": keyModeConfig(""), "sign-in mode": signInConfig("")} {
		t.Run(name, func(t *testing.T) {
			var got capture
			srv := newServer(t, &got, http.StatusOK, bedrockReply)
			cfg.BaseURL = srv.URL
			if _, err := newBedrock(cfg).Complete(context.Background(), Request{User: "x", Model: "anthropic.claude-haiku-5-5", MaxTokens: 64}); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if auth := got.headers.Get("Authorization"); auth != "Bearer token-from-environment" {
				t.Errorf("Authorization = %q, want the environment's token", auth)
			}
		})
	}
}

func TestBedrockNeverSendsAJSONSchema(t *testing.T) {
	isolateAWS(t)
	var got capture
	srv := newServer(t, &got, http.StatusOK, bedrockReply)

	if _, err := newBedrock(keyModeConfig(srv.URL)).Complete(context.Background(), Request{
		User: "x", Model: "anthropic.claude-sonnet-5-5", MaxTokens: 64,
		JSONMode: true, JSONSchema: []byte(testSchema),
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if oc, ok := got.body["output_config"]; ok {
		t.Errorf("output_config = %v: Bedrock serves these models without structured outputs", oc)
	}
	for _, field := range []string{"response_format", "format"} {
		if _, ok := got.body[field]; ok {
			t.Errorf("%s reached Bedrock", field)
		}
	}
}

func TestBedrockEffortAndThinking(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		effort       string
		noThinking   bool
		wantEffort   string
		wantDisabled bool
	}{
		{name: "effort goes as output_config.effort", model: "anthropic.claude-sonnet-5-5", effort: EffortHigh, wantEffort: EffortHigh},
		{name: "Haiku 5.5 can be asked not to reason", model: "anthropic.claude-haiku-5-5", noThinking: true, wantDisabled: true},
		{name: "a routed ID is looked up by its base", model: "eu.anthropic.claude-haiku-5-5", noThinking: true, wantDisabled: true},
		{name: "Sonnet 5.5 cannot stop reasoning, so it is not asked", model: "anthropic.claude-sonnet-5-5", noThinking: true},
		{name: "Opus 5.5 cannot stop reasoning either", model: "anthropic.claude-opus-5-5", noThinking: true},
		{name: "Haiku 4.5 takes no effort", model: "anthropic.claude-haiku-4-5-20251001-v1:0", effort: EffortLow},
		{name: "an unknown level is dropped", model: "anthropic.claude-haiku-5-5", effort: "turbo"},
		{name: "an unknown model gets the effort as asked", model: "anthropic.claude-sonnet-9", effort: EffortMedium, wantEffort: EffortMedium},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isolateAWS(t)
			var got capture
			srv := newServer(t, &got, http.StatusOK, bedrockReply)
			if _, err := newBedrock(keyModeConfig(srv.URL)).Complete(context.Background(), Request{
				User: "x", Model: tc.model, MaxTokens: 64, Effort: tc.effort, NoThinking: tc.noThinking,
			}); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			oc, _ := got.body["output_config"].(map[string]any)
			if effort, _ := oc["effort"].(string); effort != tc.wantEffort {
				t.Errorf("output_config.effort = %q, want %q", effort, tc.wantEffort)
			}
			thinking, _ := got.body["thinking"].(map[string]any)
			if disabled := thinking["type"] == "disabled"; disabled != tc.wantDisabled {
				t.Errorf("thinking = %v, want disabled=%v", got.body["thinking"], tc.wantDisabled)
			}
		})
	}
}

// TestBedrockRetriesWithoutARejectedOption: Bedrock has no model listing to
// check a level against, so a model KeyLint does not know gets the effort, and
// a refusal is retried once without it — the user's text still gets fixed.
func TestBedrockRetriesWithoutARejectedOption(t *testing.T) {
	isolateAWS(t)
	var mu sync.Mutex
	var bodies []string
	url := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(string(raw), `"effort"`) {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"message":"output_config.effort: this model does not support the effort parameter"}`)
			return
		}
		io.WriteString(w, bedrockReply)
	})

	if _, err := newBedrock(keyModeConfig(url)).Complete(context.Background(), Request{
		User: "x", Model: "anthropic.claude-sonnet-9", MaxTokens: 64, Effort: EffortMax,
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(bodies) != 2 || strings.Contains(bodies[1], "effort") {
		t.Errorf("want one retry without the effort, got %d request(s): %v", len(bodies), bodies)
	}
}

func TestBedrockInvokeID(t *testing.T) {
	tests := []struct {
		model, region, routing string
		want                   string
		wantErr                error
	}{
		// Geography by region.
		{"anthropic.claude-haiku-5-5", "eu-central-1", BedrockRoutingGeo, "eu.anthropic.claude-haiku-5-5", nil},
		{"anthropic.claude-haiku-5-5", "eu-west-2", "", "eu.anthropic.claude-haiku-5-5", nil},
		{"anthropic.claude-haiku-5-5", "us-east-1", BedrockRoutingGeo, "us.anthropic.claude-haiku-5-5", nil},
		{"anthropic.claude-haiku-5-5", "ca-central-1", BedrockRoutingGeo, "us.anthropic.claude-haiku-5-5", nil},
		{"anthropic.claude-sonnet-5-5", "ap-southeast-2", BedrockRoutingGeo, "au.anthropic.claude-sonnet-5-5", nil},
		{"anthropic.claude-haiku-5-5", "ap-southeast-4", BedrockRoutingGeo, "au.anthropic.claude-haiku-5-5", nil},
		{"anthropic.claude-haiku-5-5", "ap-northeast-1", BedrockRoutingGeo, "jp.anthropic.claude-haiku-5-5", nil},
		{"anthropic.claude-haiku-5-5", "ap-northeast-3", BedrockRoutingGeo, "jp.anthropic.claude-haiku-5-5", nil},
		// Worldwide, chosen.
		{"anthropic.claude-haiku-5-5", "eu-central-1", BedrockRoutingGlobal, "global.anthropic.claude-haiku-5-5", nil},
		// A region with no geography offers only worldwide; the card says so.
		{"anthropic.claude-haiku-5-5", "ap-south-1", BedrockRoutingGeo, "global.anthropic.claude-haiku-5-5", nil},
		{"anthropic.claude-sonnet-5-5", "sa-east-1", "", "global.anthropic.claude-sonnet-5-5", nil},
		// A known model with no route that stays: refused, never quietly global.
		{"anthropic.claude-sonnet-5-5", "ap-northeast-1", BedrockRoutingGeo, "", ErrBedrockRoute},
		{"anthropic.claude-haiku-4-5-20251001-v1:0", "ca-west-1", BedrockRoutingGeo, "", ErrBedrockRoute},
		{"anthropic.claude-sonnet-5-5", "ap-northeast-1", BedrockRoutingGlobal, "global.anthropic.claude-sonnet-5-5", nil},
		// A model KeyLint does not know gets the region's prefix; Bedrock judges it.
		{"anthropic.claude-sonnet-9", "eu-west-1", BedrockRoutingGeo, "eu.anthropic.claude-sonnet-9", nil},
		// Already routed, or an application inference profile: sent unchanged.
		{"us.anthropic.claude-haiku-5-5", "eu-central-1", BedrockRoutingGeo, "us.anthropic.claude-haiku-5-5", nil},
		{"in.anthropic.claude-sonnet-5", "ap-south-1", BedrockRoutingGeo, "in.anthropic.claude-sonnet-5", nil},
		{"global.anthropic.claude-opus-5-5", "eu-central-1", BedrockRoutingGeo, "global.anthropic.claude-opus-5-5", nil},
		{"arn:aws:bedrock:eu-central-1:123456789012:application-inference-profile/abc", "eu-central-1", BedrockRoutingGlobal,
			"arn:aws:bedrock:eu-central-1:123456789012:application-inference-profile/abc", nil},
		// No region, no call.
		{"anthropic.claude-haiku-5-5", "", BedrockRoutingGeo, "", ErrBedrockNoRegion},
		{" anthropic.claude-haiku-5-5 ", "eu-central-1", BedrockRoutingGeo, "eu.anthropic.claude-haiku-5-5", nil},
	}
	for _, tc := range tests {
		got, err := BedrockInvokeID(tc.model, tc.region, tc.routing)
		if tc.wantErr != nil {
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("BedrockInvokeID(%q, %q, %q) error = %v, want %v", tc.model, tc.region, tc.routing, err, tc.wantErr)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("BedrockInvokeID(%q, %q, %q) = %q, %v; want %q", tc.model, tc.region, tc.routing, got, err, tc.want)
		}
	}
}

func TestBedrockRouteErrorNamesModelAndPlace(t *testing.T) {
	_, err := BedrockInvokeID("anthropic.claude-sonnet-5-5", "ap-northeast-1", BedrockRoutingGeo)
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"Claude Sonnet 5.5", "Japan", "Worldwide"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// TestBedrockRouteRefusalSendsNothing: the text does not leave the machine
// when the route the user chose does not exist for the model.
func TestBedrockRouteRefusalSendsNothing(t *testing.T) {
	isolateAWS(t)
	var got capture
	srv := newServer(t, &got, http.StatusOK, bedrockReply)
	cfg := keyModeConfig(srv.URL)
	cfg.Bedrock.Region = "ap-northeast-1"
	_, err := newBedrock(cfg).Complete(context.Background(), Request{User: "x", Model: "anthropic.claude-sonnet-5-5", MaxTokens: 64})
	if !errors.Is(err, ErrBedrockRoute) {
		t.Fatalf("err = %v, want ErrBedrockRoute", err)
	}
	if got.hits != 0 {
		t.Errorf("hits = %d, want none", got.hits)
	}
}

func TestBedrockNoRegionSendsNothing(t *testing.T) {
	isolateAWS(t)
	var got capture
	srv := newServer(t, &got, http.StatusOK, bedrockReply)
	cfg := keyModeConfig(srv.URL)
	cfg.Bedrock.Region = ""
	_, err := newBedrock(cfg).Complete(context.Background(), Request{User: "x", Model: "anthropic.claude-haiku-5-5", MaxTokens: 64})
	if !errors.Is(err, ErrBedrockNoRegion) || !strings.Contains(err.Error(), "region") {
		t.Fatalf("err = %v, want ErrBedrockNoRegion", err)
	}
	if got.hits != 0 {
		t.Errorf("hits = %d, want none", got.hits)
	}
}

func TestBedrockErrorWording(t *testing.T) {
	const marker = "BODYMARKER-user-text"
	tests := []struct {
		name      string
		cfg       func(string) Config
		status    int
		errorType string
		want      []string
	}{
		{
			name: "403 without access names model, region and the console", cfg: keyModeConfig,
			status: http.StatusForbidden, errorType: "AccessDeniedException:http://internal.amazon.com/coral/com.amazon.bedrock/",
			want: []string{"AWS Bedrock error 403", "eu.anthropic.claude-haiku-5-5", "eu-central-1", "Model access", "bedrock:InvokeModel"},
		},
		{
			name: "a 403 of unknown type reads as access", cfg: keyModeConfig,
			status: http.StatusForbidden,
			want:   []string{"AWS Bedrock error 403", "eu.anthropic.claude-haiku-5-5", "Model access"},
		},
		{
			name: "a rejected key in key mode", cfg: keyModeConfig,
			status: http.StatusForbidden, errorType: "UnrecognizedClientException",
			want: []string{"AWS Bedrock error 403", "API key was not accepted"},
		},
		{
			name: "rejected credentials in sign-in mode", cfg: signInConfig,
			status: http.StatusForbidden, errorType: "UnrecognizedClientException",
			want: []string{"AWS Bedrock error 403", "AWS did not accept", "aws sts get-caller-identity"},
		},
		{
			name: "expired credentials in sign-in mode", cfg: signInConfig,
			status: http.StatusForbidden, errorType: "ExpiredTokenException",
			want: []string{"AWS Bedrock error 403", "expired", "aws sso login"},
		},
		{
			name: "401 is the key", cfg: keyModeConfig,
			status: http.StatusUnauthorized,
			want:   []string{"AWS Bedrock error 401", "API key was not accepted"},
		},
		{
			name: "404 names the model and region", cfg: keyModeConfig,
			status: http.StatusNotFound,
			want:   []string{"AWS Bedrock error 404", "eu.anthropic.claude-haiku-5-5", "eu-central-1"},
		},
		{
			name: "throttling goes through the shared wording", cfg: keyModeConfig,
			status: http.StatusTooManyRequests, errorType: "ThrottlingException",
			want: []string{"AWS Bedrock error 429", "rate limited"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isolateAWS(t)
			t.Setenv("AWS_ACCESS_KEY_ID", "AKIDTEST")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "s")
			url := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if tc.errorType != "" {
					w.Header().Set("X-Amzn-Errortype", tc.errorType)
				}
				w.WriteHeader(tc.status)
				io.WriteString(w, `{"message":"`+marker+`"}`)
			})
			_, err := newBedrock(tc.cfg(url)).Complete(context.Background(), Request{User: "x", Model: "anthropic.claude-haiku-5-5", MaxTokens: 64})
			if err == nil {
				t.Fatal("want an error")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
			if strings.Contains(err.Error(), marker) {
				t.Errorf("the provider's body reached the error: %v", err)
			}
			var status *StatusError
			if !errors.As(err, &status) || status.Status != tc.status || status.Provider != "AWS Bedrock" {
				t.Errorf("want a StatusError for AWS Bedrock %d, got %#v", tc.status, err)
			}
		})
	}
}

func TestBedrockSignInWithNoCredentials(t *testing.T) {
	isolateAWS(t)
	var got capture
	srv := newServer(t, &got, http.StatusOK, bedrockReply)

	_, err := newBedrock(signInConfig(srv.URL)).Complete(context.Background(), Request{User: "x", Model: "anthropic.claude-haiku-5-5", MaxTokens: 64})
	if !errors.Is(err, ErrAWSSignIn) {
		t.Fatalf("err = %v, want ErrAWSSignIn", err)
	}
	for _, want := range []string{"AWS Bedrock", "aws sts get-caller-identity", "API key"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if got.hits != 0 {
		t.Errorf("hits = %d: nothing should be sent without credentials", got.hits)
	}
}

func TestBedrockSignInWithAnExpiredSSOSession(t *testing.T) {
	isolateAWS(t)
	// An SSO profile whose token cache is empty: what `aws sso login` fills.
	writeAWSFile(t, "AWS_CONFIG_FILE", "[profile corp]\nsso_start_url = https://example.awsapps.com/start\nsso_region = eu-central-1\nsso_account_id = 123456789012\nsso_role_name = Writer\n")
	var got capture
	srv := newServer(t, &got, http.StatusOK, bedrockReply)

	cfg := signInConfig(srv.URL)
	cfg.Bedrock.Profile = "corp"
	_, err := newBedrock(cfg).Complete(context.Background(), Request{User: "x", Model: "anthropic.claude-haiku-5-5", MaxTokens: 64})
	if !errors.Is(err, ErrAWSSignIn) {
		t.Fatalf("err = %v, want ErrAWSSignIn", err)
	}
	if !strings.Contains(err.Error(), "aws sso login --profile corp") {
		t.Errorf("error %q should say how to sign in again", err)
	}
	if got.hits != 0 {
		t.Errorf("hits = %d, want none", got.hits)
	}
}

// TestBedrockProductionHost covers the shape with no BaseURL: the request goes
// to bedrock-runtime in the chosen region, and no environment variable — the
// Anthropic SDK's or AWS's — can send it elsewhere.
func TestBedrockProductionHost(t *testing.T) {
	isolateAWS(t)
	t.Setenv("ANTHROPIC_BASE_URL", "http://redirected.invalid")
	t.Setenv("AWS_ENDPOINT_URL", "http://redirected.invalid")
	t.Setenv("AWS_ENDPOINT_URL_BEDROCK_RUNTIME", "http://redirected.invalid")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDTEST")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "s")

	for name, cfg := range map[string]Config{"key mode": keyModeConfig(""), "sign-in mode": signInConfig("")} {
		t.Run(name, func(t *testing.T) {
			transport := &recordingTransport{body: bedrockReply}
			cfg.HTTPClient = &http.Client{Transport: transport}
			if _, err := newBedrock(cfg).Complete(context.Background(), Request{User: "x", Model: "anthropic.claude-haiku-5-5", MaxTokens: 64}); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if !slices.Equal(transport.hosts, []string{"bedrock-runtime.eu-central-1.amazonaws.com"}) {
				t.Errorf("hosts = %v, want bedrock-runtime.eu-central-1.amazonaws.com once", transport.hosts)
			}
		})
	}
}

func TestBedrockListModelsAsksNobody(t *testing.T) {
	cfg := Config{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Errorf("listing Bedrock's models made a request to %s", r.URL)
		return nil, errors.New("no requests")
	})}}
	list, err := ListModels(context.Background(), ProviderBedrock, cfg)
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if list.Source != ModelSourceKnown {
		t.Errorf("Source = %q, want %q", list.Source, ModelSourceKnown)
	}
	var ids []string
	for _, m := range list.Models {
		ids = append(ids, m.ID)
	}
	want := []string{
		"anthropic.claude-haiku-5-5", "anthropic.claude-sonnet-5-5", "anthropic.claude-opus-5-5",
		"anthropic.claude-sonnet-5", "anthropic.claude-haiku-4-5-20251001-v1:0", "anthropic.claude-opus-4-8",
	}
	if !slices.Equal(ids, want) {
		t.Errorf("models = %v, want %v", ids, want)
	}
	for _, feature := range []string{FeatureFix, FeaturePyramidize} {
		if def := DefaultModel(ProviderBedrock, feature); !slices.Contains(ids, def) {
			t.Errorf("default %s model %q is not in the list", feature, def)
		}
	}
}

func TestBedrockDefaults(t *testing.T) {
	if got := DefaultModel(ProviderBedrock, FeatureFix); got != "anthropic.claude-haiku-5-5" {
		t.Errorf("Fix default = %q", got)
	}
	if got := DefaultModel(ProviderBedrock, FeaturePyramidize); got != "anthropic.claude-sonnet-5-5" {
		t.Errorf("Pyramidize default = %q", got)
	}
}

func TestBedrockRegions(t *testing.T) {
	regions := BedrockRegions()
	byID := map[string]BedrockRegion{}
	for _, r := range regions {
		if _, dup := byID[r.ID]; dup {
			t.Errorf("region %s listed twice", r.ID)
		}
		byID[r.ID] = r
		if strings.HasPrefix(r.ID, "us-gov") {
			t.Errorf("%s is out of scope", r.ID)
		}
		if r.Label == "" {
			t.Errorf("%s has no label", r.ID)
		}
		if (r.Geography == "") != (r.GeographyName == "") {
			t.Errorf("%s: geography %q named %q", r.ID, r.Geography, r.GeographyName)
		}
	}
	checks := map[string]struct{ geo, name string }{
		"eu-central-1":   {"eu", "the EU"},
		"eu-central-2":   {"eu", "the EU and Switzerland"},
		"eu-west-2":      {"eu", "the EU and the UK"},
		"us-east-1":      {"us", "the US"},
		"ca-central-1":   {"us", "the US and Canada"},
		"ap-southeast-2": {"au", "Australia"},
		"ap-northeast-1": {"jp", "Japan"},
		"ap-south-1":     {"", ""},
		"sa-east-1":      {"", ""},
	}
	for id, want := range checks {
		r, ok := byID[id]
		if !ok {
			t.Errorf("%s missing", id)
			continue
		}
		if r.Geography != want.geo || r.GeographyName != want.name {
			t.Errorf("%s = %q/%q, want %q/%q", id, r.Geography, r.GeographyName, want.geo, want.name)
		}
	}
	// Tokyo keeps Haiku 5.5 in Japan; Sonnet 5.5 has no Japan profile.
	tokyo := byID["ap-northeast-1"]
	if !slices.Contains(tokyo.GeoModels, "anthropic.claude-haiku-5-5") || slices.Contains(tokyo.GeoModels, "anthropic.claude-sonnet-5-5") {
		t.Errorf("Tokyo geo models = %v", tokyo.GeoModels)
	}
	if len(byID["ap-south-1"].GeoModels) != 0 {
		t.Errorf("Mumbai has no geography, yet lists %v", byID["ap-south-1"].GeoModels)
	}
}

func TestCheckBedrock(t *testing.T) {
	tests := []struct {
		name   string
		status int
		reply  string
		ok     bool
	}{
		{name: "an answer", status: http.StatusOK, reply: bedrockReply, ok: true},
		// Cut off at the tiny limit still proves the connection works.
		{name: "cut off", status: http.StatusOK, ok: true,
			reply: `{"content":[{"type":"thinking","thinking":""}],"stop_reason":"max_tokens","usage":{"output_tokens":32}}`},
		{name: "refused", status: http.StatusForbidden, reply: `{"message":"denied"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isolateAWS(t)
			var got capture
			srv := newServer(t, &got, tc.status, tc.reply)
			id, err := CheckBedrock(context.Background(), keyModeConfig(srv.URL), "anthropic.claude-haiku-5-5")
			if tc.ok != (err == nil) {
				t.Fatalf("err = %v, want ok=%v", err, tc.ok)
			}
			if tc.ok && id != "eu.anthropic.claude-haiku-5-5" {
				t.Errorf("id = %q", id)
			}
			if got.hits != 1 {
				t.Errorf("hits = %d, want one tiny call", got.hits)
			}
			if mt, _ := got.body["max_tokens"].(float64); mt <= 0 || mt > 64 {
				t.Errorf("max_tokens = %v, want a tiny limit", got.body["max_tokens"])
			}
			if thinking, _ := got.body["thinking"].(map[string]any); thinking["type"] != "disabled" {
				t.Errorf("thinking = %v: Haiku is asked not to reason on a check", got.body["thinking"])
			}
		})
	}
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestBedrockRegionMustBeARegion: the region becomes part of the host name
// (bedrock-runtime.<region>.amazonaws.com), so a hand-edited value such as
// "x.evil.example/" must never reach a URL — the text would go to that host.
func TestBedrockRegionMustBeARegion(t *testing.T) {
	isolateAWS(t)
	for _, region := range []string{"x.evil.example/", "eu-central-1.evil.example", "eu-central-1/../x", "EU-CENTRAL-1", "eu central 1", "eu-central-1#"} {
		transport := &recordingTransport{body: bedrockReply}
		cfg := keyModeConfig("")
		cfg.Bedrock.Region = region
		cfg.HTTPClient = &http.Client{Transport: transport}
		_, err := newBedrock(cfg).Complete(context.Background(), Request{User: "x", Model: "anthropic.claude-haiku-5-5", MaxTokens: 8})
		if err == nil || !strings.Contains(err.Error(), "region") {
			t.Errorf("region %q: err = %v, want a refusal", region, err)
		}
		if len(transport.hosts) != 0 {
			t.Errorf("region %q reached %v", region, transport.hosts)
		}
		if _, err := BedrockInvokeID("anthropic.claude-haiku-5-5", region, BedrockRoutingGlobal); err == nil {
			t.Errorf("BedrockInvokeID accepted region %q", region)
		}
	}
	if !ValidBedrockRegion("eu-central-3") || !ValidBedrockRegion("ap-southeast-7") {
		t.Error("a well-formed region this build does not list is still a region")
	}
}

// TestBedrockSignInAlwaysSigns: an AWS config can prefer bearer auth
// (auth_scheme_preference) and carry an SSO token provider; the adapter would
// then send that SSO token to Bedrock. Sign-in mode means the computer's
// credentials, signed.
func TestBedrockSignInAlwaysSigns(t *testing.T) {
	isolateAWS(t)
	original := loadAWSConfig
	t.Cleanup(func() { loadAWSConfig = original })
	loadAWSConfig = func(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error) {
		return aws.Config{
			Region:                  "eu-central-1",
			Credentials:             aws.NewCredentialsCache(staticCreds{"AKIDSIGNED"}),
			BearerAuthTokenProvider: bedrockSDKStaticToken("sso-token-must-not-be-sent"),
			AuthSchemePreference:    []string{"httpBearerAuth"},
		}, nil
	}
	var got capture
	srv := newServer(t, &got, http.StatusOK, bedrockReply)
	if _, err := newBedrock(signInConfig(srv.URL)).Complete(context.Background(), Request{User: "x", Model: "anthropic.claude-haiku-5-5", MaxTokens: 8}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if auth := got.headers.Get("Authorization"); !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=AKIDSIGNED/") {
		t.Errorf("Authorization = %q, want SigV4 with the computer's credentials", auth)
	}
}

type staticCreds struct{ id string }

func (s staticCreds) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{AccessKeyID: s.id, SecretAccessKey: "s", Source: "test"}, nil
}

func bedrockSDKStaticToken(v string) *bearer.StaticTokenProvider {
	return &bearer.StaticTokenProvider{Token: bearer.Token{Value: v}}
}

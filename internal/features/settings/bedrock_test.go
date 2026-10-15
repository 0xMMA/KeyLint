package settings

import (
	"context"
	"errors"
	"strings"
	"testing"

	"keylint/internal/llm"
)

func bedrockService(t *testing.T) *Service {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestBedrockCanBeTheProviderInUse(t *testing.T) {
	svc := bedrockService(t)
	if err := svc.SetActiveProvider(llm.ProviderBedrock); err != nil {
		t.Fatalf("SetActiveProvider(bedrock): %v", err)
	}
	if got := svc.Get().ActiveProvider; got != llm.ProviderBedrock {
		t.Errorf("active_provider = %q", got)
	}
}

func TestSetBedrockSavesTheConnectionAndNothingElse(t *testing.T) {
	svc := bedrockService(t)
	before := svc.Get()
	before.Providers.OllamaURL = "http://gpu-box:11434"
	before.LogLevel = "debug"
	if err := svc.Save(before); err != nil {
		t.Fatal(err)
	}

	if err := svc.SetBedrock(BedrockSettings{
		Region: " eu-central-1 ", Auth: llm.BedrockAuthSignIn, Profile: " work ", Routing: llm.BedrockRoutingGlobal,
	}); err != nil {
		t.Fatalf("SetBedrock: %v", err)
	}
	got := svc.Get()
	p := got.Providers
	if p.AWSRegion != "eu-central-1" || p.BedrockAuth != llm.BedrockAuthSignIn || p.AWSProfile != "work" || p.BedrockRouting != llm.BedrockRoutingGlobal {
		t.Errorf("providers = %+v", p)
	}
	if p.OllamaURL != "http://gpu-box:11434" || got.LogLevel != "debug" {
		t.Errorf("SetBedrock changed other settings: %+v", got)
	}

	// Persisted: a fresh service reads it back.
	again, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	if again.Get().Providers != p {
		t.Errorf("on disk = %+v, want %+v", again.Get().Providers, p)
	}
}

func TestSetBedrockRefusesWhatItCannotUse(t *testing.T) {
	svc := bedrockService(t)
	valid := BedrockSettings{Region: "eu-central-1", Auth: llm.BedrockAuthKey, Routing: llm.BedrockRoutingGeo}
	if err := svc.SetBedrock(valid); err != nil {
		t.Fatalf("valid: %v", err)
	}
	for name, b := range map[string]BedrockSettings{
		"region with a path":   {Region: "eu-central-1/../x", Auth: llm.BedrockAuthKey, Routing: llm.BedrockRoutingGeo},
		"region as a host":     {Region: "evil.example.com", Auth: llm.BedrockAuthKey, Routing: llm.BedrockRoutingGeo},
		"unknown mode":         {Region: "eu-central-1", Auth: "password", Routing: llm.BedrockRoutingGeo},
		"unknown routing":      {Region: "eu-central-1", Auth: llm.BedrockAuthKey, Routing: "anywhere"},
		"profile with newline": {Region: "eu-central-1", Auth: llm.BedrockAuthSignIn, Profile: "a\nb", Routing: llm.BedrockRoutingGeo},
		"profile far too long": {Region: "eu-central-1", Auth: llm.BedrockAuthSignIn, Profile: strings.Repeat("p", 200), Routing: llm.BedrockRoutingGeo},
	} {
		if err := svc.SetBedrock(b); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if got := svc.Get().Providers; got.AWSRegion != "eu-central-1" || got.BedrockAuth != llm.BedrockAuthKey {
		t.Errorf("a refused SetBedrock changed settings: %+v", got)
	}
	// No region yet is a state the card shows, not an error.
	if err := svc.SetBedrock(BedrockSettings{Auth: llm.BedrockAuthKey, Routing: llm.BedrockRoutingGeo}); err != nil {
		t.Errorf("empty region: %v", err)
	}
	// A region this build does not list is still a region (AWS adds them).
	if err := svc.SetBedrock(BedrockSettings{Region: "eu-central-3", Auth: llm.BedrockAuthKey, Routing: llm.BedrockRoutingGeo}); err != nil {
		t.Errorf("unlisted region: %v", err)
	}
}

func TestSettingsBedrockDefaults(t *testing.T) {
	// A settings file that names nothing: key mode, keep in the geography.
	b := Default().Bedrock()
	if b.Auth != llm.BedrockAuthKey || b.Routing != llm.BedrockRoutingGeo || b.Region != "" {
		t.Errorf("defaults = %+v", b)
	}
	s := Default()
	s.Providers = Provider{AWSRegion: "us-west-2", BedrockAuth: llm.BedrockAuthSignIn, AWSProfile: "dev", BedrockRouting: llm.BedrockRoutingGlobal}
	if got := s.Bedrock(); got != (llm.BedrockConfig{Region: "us-west-2", Auth: llm.BedrockAuthSignIn, Profile: "dev", Routing: llm.BedrockRoutingGlobal}) {
		t.Errorf("Bedrock() = %+v", got)
	}
}

// TestResetKeepsTheBedrockConnection: the region and how KeyLint connects
// are what make Bedrock work, like the keys and the provider in use that a
// reset already keeps. Losing the region would leave the hotkey failing.
func TestResetKeepsTheBedrockConnection(t *testing.T) {
	svc := bedrockService(t)
	conn := BedrockSettings{Region: "eu-west-1", Auth: llm.BedrockAuthSignIn, Profile: "work", Routing: llm.BedrockRoutingGlobal}
	if err := svc.SetBedrock(conn); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetToDefaults(); err != nil {
		t.Fatal(err)
	}
	p := svc.Get().Providers
	if p.AWSRegion != conn.Region || p.BedrockAuth != conn.Auth || p.AWSProfile != conn.Profile || p.BedrockRouting != conn.Routing {
		t.Errorf("after reset: %+v", p)
	}
}

// TestTheBedrockKeyComesFromAWSBearerTokenBedrock: AWS's own variable for a
// Bedrock API key. AWS_SECRET_ACCESS_KEY is half of an IAM key pair, on many
// machines for other tools, and never a Bedrock key.
func TestTheBedrockKeyComesFromAWSBearerTokenBedrock(t *testing.T) {
	t.Setenv("AWS_SECRET_ACCESS_KEY", "not-a-bedrock-key")
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "")
	if got := EnvOnlyKeys(llm.ProviderBedrock); got != "" {
		t.Errorf("EnvOnlyKeys = %q: AWS_SECRET_ACCESS_KEY is not a Bedrock key", got)
	}
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "bedrock-token")
	if got := EnvOnlyKeys(llm.ProviderBedrock); got != "bedrock-token" {
		t.Errorf("EnvOnlyKeys = %q, want the token", got)
	}
	svc := bedrockService(t)
	if st := svc.GetKeyStatus(llm.ProviderBedrock); !st.IsSet || st.Source != "env" {
		t.Errorf("GetKeyStatus = %+v, want set from env", st)
	}
	if got := svc.GetKey(llm.ProviderBedrock); got != "bedrock-token" {
		t.Errorf("GetKey = %q", got)
	}
}

func TestBedrockModelsAreListedWithoutARequest(t *testing.T) {
	svc := bedrockService(t)
	list := svc.ListModels(llm.ProviderBedrock)
	if list.Source != llm.ModelSourceKnown || len(list.Models) == 0 {
		t.Errorf("ListModels(bedrock) = %+v", list)
	}
}

func TestBedrockRegionsComeFromTheBuild(t *testing.T) {
	svc := bedrockService(t)
	if got, want := len(svc.BedrockRegions()), len(llm.BedrockRegions()); got != want || got == 0 {
		t.Errorf("BedrockRegions() = %d regions, want %d", got, want)
	}
}

// TestTestBedrockChecksTheSavedConnection: the card's button tests what is
// saved — region, mode, routing, the key in key mode, and the Fix model — with
// one call, and refuses before calling when key mode has no key.
func TestTestBedrockChecksTheSavedConnection(t *testing.T) {
	cfg := Default()
	cfg.Providers = Provider{AWSRegion: "eu-central-1", BedrockAuth: llm.BedrockAuthKey, BedrockRouting: llm.BedrockRoutingGlobal}
	cfg.Models = map[string]FeatureModels{llm.ProviderBedrock: {Fix: "anthropic.claude-haiku-4-5-20251001-v1:0"}}

	var calls []llm.Config
	var models []string
	check := func(_ context.Context, c llm.Config, model string) (string, error) {
		calls = append(calls, c)
		models = append(models, model)
		return "global." + model, nil
	}

	withKey := NewServiceFrom(cfg, func(p string) string {
		if p == llm.ProviderBedrock {
			return "the-key"
		}
		return ""
	})
	withKey.checkBedrock = check
	id, err := withKey.TestBedrock()
	if err != nil || id != "global.anthropic.claude-haiku-4-5-20251001-v1:0" {
		t.Fatalf("TestBedrock = %q, %v", id, err)
	}
	if len(calls) != 1 || calls[0].APIKey != "the-key" || calls[0].Bedrock.Region != "eu-central-1" ||
		calls[0].Bedrock.Routing != llm.BedrockRoutingGlobal || models[0] != "anthropic.claude-haiku-4-5-20251001-v1:0" {
		t.Errorf("checked with %+v / %v", calls, models)
	}

	calls = nil
	noKey := NewServiceFrom(cfg, nil)
	noKey.checkBedrock = check
	if _, err := noKey.TestBedrock(); err == nil || !strings.Contains(err.Error(), "key") {
		t.Errorf("key mode without a key: err = %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("checked %d time(s) without a key", len(calls))
	}

	// Sign-in mode needs no key, and never reads one.
	signIn := cfg
	signIn.Providers.BedrockAuth = llm.BedrockAuthSignIn
	read := false
	svc := NewServiceFrom(signIn, func(string) string { read = true; return "" })
	svc.checkBedrock = check
	if _, err := svc.TestBedrock(); err != nil {
		t.Errorf("sign-in mode: %v", err)
	}
	if read {
		t.Error("sign-in mode read a key")
	}

	// Its failure is the provider's own wording.
	failing := NewServiceFrom(signIn, nil)
	failing.checkBedrock = func(context.Context, llm.Config, string) (string, error) {
		return "", errors.New("AWS Bedrock error 403: access denied")
	}
	if _, err := failing.TestBedrock(); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v", err)
	}
}

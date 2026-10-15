package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"

	"keylint/internal/logger"
)

// AWS Bedrock (roadmap E6) serves Anthropic's models from the user's own AWS
// account. KeyLint calls bedrock-runtime's InvokeModel through the Anthropic
// SDK's bedrock adapter, which turns a Messages request into
// POST /model/{id}/invoke, adds anthropic_version, and signs it (SigV4) or
// sends a Bedrock API key as a bearer token.
//
// Not bedrock-mantle: per the AWS model cards (read 2026-10-15) Haiku 5.5 and
// Sonnet 5.5 are in-region on mantle only in us-gov-west-1. Everywhere else
// they are served on bedrock-runtime, and only through a cross-region
// inference profile — no in-region option at all.
//
// The request is the Anthropic API's (messagesCall), with three differences:
//   - the model ID carries a routing prefix chosen from region and routing
//     (BedrockInvokeID), so a model is stored once and a new region or routing
//     needs no re-pick;
//   - there is no model listing (Anthropic's Models API is not offered on
//     Bedrock), so what a model takes comes from its AWS model card
//     (bedrockModels), and a model KeyLint does not know gets the effort as
//     asked and one retry without it on a refusal;
//   - structured outputs are never sent: the cards list them as not supported
//     on bedrock-runtime for every current model.

// Bedrock connection modes, as settings store them (providers.bedrock_auth).
const (
	// BedrockAuthKey is a Bedrock API key, kept in the OS keyring like every
	// other key. The default: a settings file that names no mode asks for a
	// key rather than reading the computer's AWS sign-in.
	BedrockAuthKey = "api-key"
	// BedrockAuthSignIn is this computer's AWS sign-in: a profile, SSO, or the
	// default credential chain. KeyLint stores nothing for it.
	BedrockAuthSignIn = "aws-signin"
)

// Bedrock routing, as settings store it (providers.bedrock_routing). The
// user's explicit choice (owner decision 2026-10-15).
const (
	// BedrockRoutingGeo keeps the text in the region's geography through a
	// geographic inference profile (eu., us., au., jp.). The default.
	BedrockRoutingGeo = "geo"
	// BedrockRoutingGlobal lets AWS route the text to any region worldwide
	// (global. profile). About 10% cheaper: Anthropic documents regional
	// endpoints at a 10% premium over global ones.
	BedrockRoutingGlobal = "global"
)

// BedrockConfig is what a Bedrock call needs beyond the key: where, how to
// sign in, and how far the text may travel.
type BedrockConfig struct {
	Region string
	// Auth is BedrockAuthKey or BedrockAuthSignIn; anything else reads as the key.
	Auth string
	// Profile names an AWS profile for sign-in mode; "" is the default chain.
	Profile string
	// Routing is BedrockRoutingGeo or BedrockRoutingGlobal; anything else
	// reads as geo — the one that keeps the text closer.
	Routing string
}

func (b BedrockConfig) signIn() bool { return b.Auth == BedrockAuthSignIn }

func (b BedrockConfig) global() bool { return b.Routing == BedrockRoutingGlobal }

var bedrockProvider = provider{id: ProviderBedrock, name: "AWS Bedrock"}

// Typed Bedrock failures, for a caller that acts on the reason (the hotkey
// toast) rather than reading wording.
var (
	// ErrBedrockNoRegion: no region is chosen, so there is nowhere to call.
	ErrBedrockNoRegion = errors.New("no AWS region is set — choose one on the AWS Bedrock card in Settings › AI providers")
	// ErrBedrockRoute: the model has no route that keeps the text where the
	// user chose. Nothing is sent; see bedrockRouteError.
	ErrBedrockRoute = errors.New("no route keeps the text where it was chosen to stay")
	// ErrAWSSignIn: this computer's AWS sign-in could not be used.
	ErrAWSSignIn = errors.New("this computer's AWS sign-in could not be used")
)

// bedrockRegion is one region KeyLint offers, and the geography a geographic
// profile keeps a call in from there.
type bedrockRegion struct {
	id, label string
	// geo is the profile prefix that keeps a call in the geography, or ""
	// where none exists and only worldwide routing is offered.
	geo string
	// geoName says where a geo-routed call can be processed, in the user's
	// words. Per region, not per prefix, because the profiles' destinations
	// depend on where the call starts: eu. from Zurich or London may also stay
	// in that region, us. from Canada may stay in Canada (model cards,
	// destination tables, 2026-10-15).
	geoName string
}

// bedrockRegions are the regions where Haiku 5.5 and Sonnet 5.5 can be called
// on bedrock-runtime — the model cards' runtime tables, read 2026-10-15.
// GovCloud is out of scope (its cards mark Geo "yes" but list no profile).
// AWS warns not to rely on a profile's prefix alone for where it routes; the
// destinations behind each geoName are the cards' destination tables.
var bedrockRegions = []bedrockRegion{
	{"us-east-1", "US East (N. Virginia)", "us", "the US"},
	{"us-east-2", "US East (Ohio)", "us", "the US"},
	{"us-west-1", "US West (N. California)", "us", "the US"},
	{"us-west-2", "US West (Oregon)", "us", "the US"},
	{"ca-central-1", "Canada (Central)", "us", "the US and Canada"},
	{"ca-west-1", "Canada West (Calgary)", "us", "the US and Canada"},
	{"eu-central-1", "Europe (Frankfurt)", "eu", "the EU"},
	{"eu-central-2", "Europe (Zurich)", "eu", "the EU and Switzerland"},
	{"eu-north-1", "Europe (Stockholm)", "eu", "the EU"},
	{"eu-south-1", "Europe (Milan)", "eu", "the EU"},
	{"eu-south-2", "Europe (Spain)", "eu", "the EU"},
	{"eu-west-1", "Europe (Ireland)", "eu", "the EU"},
	{"eu-west-2", "Europe (London)", "eu", "the EU and the UK"},
	{"eu-west-3", "Europe (Paris)", "eu", "the EU"},
	{"ap-southeast-2", "Asia Pacific (Sydney)", "au", "Australia"},
	{"ap-southeast-4", "Asia Pacific (Melbourne)", "au", "Australia"},
	{"ap-northeast-1", "Asia Pacific (Tokyo)", "jp", "Japan"},
	{"ap-northeast-3", "Asia Pacific (Osaka)", "jp", "Japan"},
	{"ap-east-2", "Asia Pacific (Taipei)", "", ""},
	{"ap-northeast-2", "Asia Pacific (Seoul)", "", ""},
	{"ap-south-1", "Asia Pacific (Mumbai)", "", ""},
	{"ap-south-2", "Asia Pacific (Hyderabad)", "", ""},
	{"ap-southeast-1", "Asia Pacific (Singapore)", "", ""},
	{"ap-southeast-3", "Asia Pacific (Jakarta)", "", ""},
	{"ap-southeast-5", "Asia Pacific (Malaysia)", "", ""},
	{"ap-southeast-6", "Asia Pacific (New Zealand)", "", ""},
	{"ap-southeast-7", "Asia Pacific (Thailand)", "", ""},
	{"il-central-1", "Israel (Tel Aviv)", "", ""},
	{"me-central-1", "Middle East (UAE)", "", ""},
	{"me-south-1", "Middle East (Bahrain)", "", ""},
	{"af-south-1", "Africa (Cape Town)", "", ""},
	{"sa-east-1", "South America (São Paulo)", "", ""},
	{"mx-central-1", "Mexico (Central)", "", ""},
}

// geoNames names a geography for a region KeyLint does not list — a region
// AWS added after this build, typed into a settings file.
var geoNames = map[string]string{"us": "the US", "eu": "Europe", "au": "Australia", "jp": "Japan"}

// bedrockModel is one model KeyLint knows on Bedrock, with what its AWS model
// card says (read 2026-10-15).
type bedrockModel struct {
	id, label string
	// geos are the geographic profile prefixes this model has.
	geos []string
	// noGeoFrom are regions whose geography the model's profile does not take
	// calls from (Haiku 4.5's us. profile lists ca-central-1 but not ca-west-1).
	noGeoFrom []string
	// effort: the model takes output_config.effort.
	effort bool
	// thinkingOff: the model takes thinking: {type: "disabled"}.
	thinkingOff bool
}

// bedrockModels is the picker's list, in its order, and what a request to
// each may carry. Every one has a global. profile. Bedrock has no model list
// to ask, so this stands in for the Anthropic listing's capabilities.
var bedrockModels = []bedrockModel{
	// "adaptive thinking is on by default and can be turned off; effort
	// low–max; default medium".
	{id: "anthropic.claude-haiku-5-5", label: "Claude Haiku 5.5", geos: []string{"us", "eu", "au", "jp"}, effort: true, thinkingOff: true},
	// "adaptive thinking is on by default; effort low–max; default high". The
	// card does not say it can be turned off, and the Anthropic API refuses
	// thinking: disabled for it with a 400. No jp. profile.
	{id: "anthropic.claude-sonnet-5-5", label: "Claude Sonnet 5.5", geos: []string{"us", "eu", "au"}, effort: true},
	// "adaptive thinking is always on and cannot be disabled".
	{id: "anthropic.claude-opus-5-5", label: "Claude Opus 5.5", geos: []string{"us", "eu", "au", "jp"}, effort: true},
	// "thinking can be turned off"; effort configurable. Also an in. profile
	// (Mumbai, Hyderabad), which KeyLint's geography map does not use — the
	// 5.5 models have none; type in.anthropic.claude-sonnet-5 to use it.
	{id: "anthropic.claude-sonnet-5", label: "Claude Sonnet 5", geos: []string{"us", "eu", "au"}, effort: true, thinkingOff: true},
	// Not on the effort list; thinking is off unless asked for. Also an in.
	// profile, and Bedrock's one model here with structured outputs — which
	// KeyLint still does not send (see the package comment above).
	{id: "anthropic.claude-haiku-4-5-20251001-v1:0", label: "Claude Haiku 4.5", geos: []string{"us", "eu", "au", "jp"}, noGeoFrom: []string{"ca-west-1"}, thinkingOff: true},
	// Adaptive and disabled thinking; effort low–max per Anthropic's docs (the
	// card does not list levels). Its us-gov. profile is out of scope.
	{id: "anthropic.claude-opus-4-8", label: "Claude Opus 4.8", geos: []string{"us", "eu", "au", "jp"}, effort: true, thinkingOff: true},
}

// bedrockRoutingPrefixes are the leading segments that already route an ID:
// the ones in use and the ones AWS has used before. An ID starting with one is
// sent as it is.
var bedrockRoutingPrefixes = []string{"global", "us", "eu", "au", "jp", "apac", "in", "ca", "us-gov"}

// splitBedrockPrefix returns the routing prefix of an ID and the rest, or ""
// and the ID when it has none.
func splitBedrockPrefix(id string) (prefix, base string) {
	if head, rest, ok := strings.Cut(id, "."); ok && slices.Contains(bedrockRoutingPrefixes, head) {
		return head, rest
	}
	return "", id
}

func findBedrockModel(id string) (bedrockModel, bool) {
	_, base := splitBedrockPrefix(id)
	for _, m := range bedrockModels {
		if m.id == base {
			return m, true
		}
	}
	return bedrockModel{}, false
}

// staysIn reports whether a geo-routed call from region can use this model.
func (m bedrockModel) staysIn(geo, region string) bool {
	return slices.Contains(m.geos, geo) && !slices.Contains(m.noGeoFrom, region)
}

// regionPattern is the shape of an AWS region code: eu-central-1, ap-southeast-7.
var regionPattern = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-[0-9]+$`)

// ValidBedrockRegion reports whether region has the shape of an AWS region.
// The region becomes part of the host name (bedrock-runtime.<region>.amazonaws.com),
// so anything else — a dot, a slash, a hand-edited host — must never reach a
// URL. A well-formed region this build does not list is accepted: AWS adds them.
func ValidBedrockRegion(region string) bool {
	return regionPattern.MatchString(region)
}

// bedrockGeography is the profile prefix that keeps a call from region in its
// geography, and the geography's name — "" for a region with only worldwide
// routing. A listed region answers from the table; one KeyLint does not list
// is read from its name, by the rule the table follows.
func bedrockGeography(region string) (geo, name string) {
	for _, r := range bedrockRegions {
		if r.id == region {
			return r.geo, r.geoName
		}
	}
	switch {
	case strings.HasPrefix(region, "us-gov-"):
		return "", ""
	case strings.HasPrefix(region, "eu-"):
		geo = "eu"
	case strings.HasPrefix(region, "us-"), strings.HasPrefix(region, "ca-"):
		geo = "us"
	default:
		return "", ""
	}
	return geo, geoNames[geo]
}

// BedrockInvokeID is the model ID a request goes to: the stored ID with the
// routing prefix for region and routing.
//
// An ID that already carries a routing prefix, or is an ARN (an application
// inference profile), is the user's own choice of route and is sent
// unchanged. Worldwide adds global.; geo adds the region's geography. A region
// with no geography offers only worldwide routing, and the AWS Bedrock card
// says so where the region is chosen. A model KeyLint knows that has no
// profile for the geography is refused rather than quietly sent worldwide: the
// user chose to keep the text there.
func BedrockInvokeID(model, region, routing string) (string, error) {
	model = strings.TrimSpace(model)
	region = strings.TrimSpace(region)
	if region == "" {
		return "", fmt.Errorf("%s: %w", bedrockProvider.name, ErrBedrockNoRegion)
	}
	if !ValidBedrockRegion(region) {
		return "", fmt.Errorf("%s: %q is not an AWS region — choose one on the AWS Bedrock card in Settings › AI providers", bedrockProvider.name, region)
	}
	if strings.HasPrefix(model, "arn:") {
		return model, nil
	}
	if prefix, _ := splitBedrockPrefix(model); prefix != "" {
		return model, nil
	}
	if routing == BedrockRoutingGlobal {
		return "global." + model, nil
	}
	geo, name := bedrockGeography(region)
	if geo == "" {
		return "global." + model, nil
	}
	if m, known := findBedrockModel(model); known && !m.staysIn(geo, region) {
		return "", &bedrockRouteError{model: m.label, where: name}
	}
	return geo + "." + model, nil
}

// bedrockRouteError is ErrBedrockRoute with the model and the place named.
type bedrockRouteError struct{ model, where string }

func (e *bedrockRouteError) Error() string {
	return fmt.Sprintf("%s: %s cannot be kept in %s on Bedrock, so nothing was sent — choose Worldwide on the AWS Bedrock card in Settings › AI providers, or another model",
		bedrockProvider.name, e.model, e.where)
}

func (e *bedrockRouteError) Is(target error) bool { return target == ErrBedrockRoute }

// BedrockRegion is one region the AWS Bedrock card offers.
type BedrockRegion struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Geography is the profile prefix that keeps the text in GeographyName,
	// or "" where only worldwide routing exists.
	Geography     string `json:"geography"`
	GeographyName string `json:"geography_name"`
	// GeoModels are the models KeyLint knows that can stay in the geography
	// from this region — empty where there is no geography.
	GeoModels []string `json:"geo_models"`
}

// BedrockRegions lists the regions the AWS Bedrock card offers, from this
// build's table. No request: Settings must not call anyone on opening.
func BedrockRegions() []BedrockRegion {
	out := make([]BedrockRegion, 0, len(bedrockRegions))
	for _, r := range bedrockRegions {
		region := BedrockRegion{ID: r.id, Label: r.label, Geography: r.geo, GeographyName: r.geoName, GeoModels: []string{}}
		if r.geo != "" {
			for _, m := range bedrockModels {
				if m.staysIn(r.geo, r.id) {
					region.GeoModels = append(region.GeoModels, m.id)
				}
			}
		}
		out = append(out, region)
	}
	return out
}

// bedrockCuratedModels is the picker's list: the models above, labelled.
func bedrockCuratedModels() []ModelInfo {
	models := make([]ModelInfo, 0, len(bedrockModels))
	for _, m := range bedrockModels {
		models = append(models, ModelInfo{ID: m.id, Label: m.label})
	}
	return models
}

type bedrockClient struct {
	cfg Config
}

func newBedrock(cfg Config) Client { return &bedrockClient{cfg: cfg} }

func (c *bedrockClient) Complete(ctx context.Context, req Request) (Response, error) {
	if req.Model == "" {
		return Response{}, fmt.Errorf("%s: model is required", bedrockProvider.name)
	}
	if req.MaxTokens <= 0 {
		return Response{}, fmt.Errorf("%s: max_tokens is required", bedrockProvider.name)
	}
	b := c.cfg.Bedrock
	region := strings.TrimSpace(b.Region)
	// Before anything else: a route the user did not choose, or no region,
	// means the text does not leave the machine.
	model, err := BedrockInvokeID(req.Model, region, b.Routing)
	if err != nil {
		return Response{}, err
	}
	if geo, _ := bedrockGeography(region); geo == "" && !b.global() && model != strings.TrimSpace(req.Model) {
		logger.Info("llm: this region offers only worldwide routing", "feature", c.cfg.Feature,
			"provider", bedrockProvider.id, "region", region, "model", model)
	}

	effort := req.Effort
	if effort != "" && !IsEffortLevel(effort) {
		// A hand-edited settings file is the only way here; see the Anthropic
		// client.
		logger.Warn("llm: unknown effort level ignored", "feature", c.cfg.Feature, "effort", effort)
		effort = ""
	}
	noThinking := req.NoThinking
	if known, ok := findBedrockModel(model); ok {
		if effort != "" && !known.effort {
			logger.Info("llm: effort not supported by this model, sent without it",
				"feature", c.cfg.Feature, "provider", bedrockProvider.id, "model", model, "effort", effort)
			effort = ""
		}
		if noThinking && !known.thinkingOff {
			noThinking = false
		}
	}
	if len(req.JSONSchema) > 0 {
		// KEYLINT_PYRAMIDIZE_SCHEMA=1 is the only way here. The caller parses
		// the reply defensively either way.
		logger.Info("llm: Bedrock takes no JSON schema for these models, sent without it",
			"feature", c.cfg.Feature, "provider", bedrockProvider.id, "model", model)
	}

	awsCfg, err := c.awsConfig(ctx, region)
	if err != nil {
		return Response{}, err
	}

	call := messagesCall{
		p:          bedrockProvider,
		model:      model,
		effort:     effort,
		noThinking: noThinking,
		client:     func(a *httpAttempts) anthropic.Client { return c.client(awsCfg, a) },
		mapError: func(attempts *httpAttempts, err error) error {
			return mapBedrockError(attempts, model, region, b.signIn(), err)
		},
	}
	return call.run(ctx, c.cfg, req)
}

// loadAWSConfig reads this computer's AWS configuration. A variable so a test
// can see whether it was called at all.
var loadAWSConfig = config.LoadDefaultConfig

// bedrockTokenEnv is the variable AWS's own SDKs read a Bedrock API key from.
const bedrockTokenEnv = "AWS_BEARER_TOKEN_BEDROCK"

// awsConfig is how this call authenticates.
//
// Key mode never touches ~/.aws: a user who pasted a key has not handed
// KeyLint their AWS sign-in. Sign-in mode loads the AWS configuration the way
// the AWS CLI does — the named profile or the default chain — and resolves the
// credentials before anything is sent, so a missing or expired sign-in is
// reported as that, not as a failed request.
//
// AWS_BEARER_TOKEN_BEDROCK wins in both modes. The Anthropic SDK's adapter
// makes it win whatever KeyLint passes, and that is also what AWS's own SDKs
// do with it for Bedrock: in key mode it is KeyLint's env-over-keyring rule,
// in sign-in mode it is part of "this computer's AWS setup", and the AWS
// Bedrock card says so when it is set. So sign-in mode then uses it directly
// and does not read ~/.aws for nothing.
func (c *bedrockClient) awsConfig(ctx context.Context, region string) (aws.Config, error) {
	b := c.cfg.Bedrock
	if !b.signIn() {
		key := strings.TrimSpace(c.cfg.APIKey)
		if key == "" {
			key = os.Getenv(bedrockTokenEnv)
		}
		if key == "" {
			return aws.Config{}, fmt.Errorf("%s: no API key is set — add one on the AWS Bedrock card in Settings › AI providers, or choose this computer's AWS sign-in", bedrockProvider.name)
		}
		return bearerConfig(region, key), nil
	}

	if token := os.Getenv(bedrockTokenEnv); token != "" {
		logger.Info("llm: using AWS_BEARER_TOKEN_BEDROCK from the environment, which takes priority over the AWS sign-in",
			"feature", c.cfg.Feature, "provider", bedrockProvider.id)
		return bearerConfig(region, token), nil
	}

	profile := strings.TrimSpace(b.Profile)
	opts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	// Not KeyLint's HTTP client: the EC2 metadata lookup at the end of the
	// default chain brings its own sub-second timeouts, which a 90 s client
	// would replace, and a desktop has no metadata service to wait for.
	awsCfg, err := loadAWSConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, awsSignInError(ctx, err, profile)
	}
	if awsCfg.Credentials == nil {
		return aws.Config{}, awsSignInError(ctx, errors.New("no credential provider"), profile)
	}
	// Signed with the computer's credentials, always. An AWS config that
	// prefers bearer auth (auth_scheme_preference) and carries an SSO token
	// provider would otherwise have the adapter send that SSO token to
	// Bedrock as if it were a Bedrock API key.
	awsCfg.BearerAuthTokenProvider = nil
	awsCfg.AuthSchemePreference = nil
	// The credentials are cached in awsCfg, so the signing step reuses them.
	if _, err := awsCfg.Credentials.Retrieve(ctx); err != nil {
		return aws.Config{}, awsSignInError(ctx, err, profile)
	}
	return awsCfg, nil
}

// bearerConfig authenticates with a Bedrock API key.
func bearerConfig(region, key string) aws.Config {
	return aws.Config{
		Region:                  region,
		BearerAuthTokenProvider: bedrock.NewStaticBearerTokenProvider(key),
		AuthSchemePreference:    []string{"httpBearerAuth"},
	}
}

// awsSignInError words a sign-in that did not work.
//
// The AWS SDK's own error is logged at Debug and not repeated: it can carry
// account IDs, role ARNs and an STS message, and the user's terminal is the
// better place to read it — the wording names the command that shows it.
func awsSignInError(ctx context.Context, err error, profile string) error {
	if ctx.Err() != nil {
		// The caller gave up (or the deadline passed): not a sign-in problem.
		return transportError(bedrockProvider, err)
	}
	logger.Debug("llm: AWS sign-in failed", "provider", bedrockProvider.id, "profile", profile, "err", err)
	flag, whose := "", "this computer's AWS sign-in"
	if profile != "" {
		flag = " --profile " + profile
		whose = fmt.Sprintf("the AWS profile %q", profile)
	}
	var missing config.SharedConfigProfileNotExistError
	var sso *ssocreds.InvalidTokenError
	var msg string
	switch {
	case errors.As(err, &missing):
		msg = fmt.Sprintf("%s: the AWS profile %q was not found in your AWS configuration — check the profile name on the AWS Bedrock card",
			bedrockProvider.name, missing.Profile)
	case errors.As(err, &sso):
		msg = fmt.Sprintf("%s: your AWS SSO session has expired or you are not signed in — run aws sso login%s, then try again",
			bedrockProvider.name, flag)
	default:
		msg = fmt.Sprintf("%s: %s did not work — check it with aws sts get-caller-identity%s (sign in with aws configure or aws sso login), or use a Bedrock API key on the AWS Bedrock card",
			bedrockProvider.name, whose, flag)
	}
	return &signInError{msg: msg, err: err}
}

// signInError is ErrAWSSignIn with KeyLint's wording. It unwraps to the SDK's
// error for errors.As, without printing it.
type signInError struct {
	msg string
	err error
}

func (e *signInError) Error() string        { return e.msg }
func (e *signInError) Unwrap() error        { return e.err }
func (e *signInError) Is(target error) bool { return target == ErrAWSSignIn }

// client builds the SDK client per call, as the Anthropic client does: no
// environment defaults, KeyLint's HTTP client and retry budget, no machine
// fingerprint headers.
func (c *bedrockClient) client(awsCfg aws.Config, attempts *httpAttempts) anthropic.Client {
	opts := []option.RequestOption{
		option.WithoutEnvironmentDefaults(),
		option.WithHTTPClient(c.cfg.httpClient()),
		option.WithMaxRetries(sdkMaxRetries),
	}
	if timeout := c.cfg.requestTimeout(); timeout > 0 {
		opts = append(opts, option.WithRequestTimeout(timeout))
	}
	for _, header := range fingerprintHeaders {
		opts = append(opts, option.WithHeaderDel(header))
	}
	opts = append(opts, option.WithHeader("User-Agent", userAgent))
	// The adapter rewrites the request and signs it. Its base URL is
	// bedrock-runtime in the region, built from the region alone: neither
	// ANTHROPIC_BASE_URL nor AWS_ENDPOINT_URL can move it.
	opts = append(opts, bedrock.WithConfig(awsCfg))
	// Registered after the adapter, so it runs closest to the wire and logs
	// the URL that is really called (/model/<id>/invoke). It only reads the
	// request; changing one after it is signed would break the signature.
	opts = append(opts, option.WithMiddleware(attempts.middleware))
	// Tests point it at an httptest server; production leaves it empty.
	if base := resolveBaseURL(c.cfg.BaseURL, ""); base != "" {
		opts = append(opts, option.WithBaseURL(base+"/"))
	}
	return anthropic.NewClient(opts...)
}

// awsErrorType is the error class AWS puts in the X-Amzn-Errortype header —
// "AccessDeniedException:http://internal.amazon.com/coral/…" reads as
// AccessDeniedException. The class names no user text, unlike the body.
func awsErrorType(apiErr *anthropic.Error) string {
	if apiErr.Response == nil {
		return ""
	}
	t := apiErr.Response.Header.Get("X-Amzn-Errortype")
	if i := strings.IndexByte(t, ':'); i >= 0 {
		t = t[:i]
	}
	if i := strings.LastIndexByte(t, '#'); i >= 0 {
		t = t[i+1:]
	}
	return t
}

// mapBedrockError words a failed call. The raw body stays out of it, as for
// every provider (#41); the error class header and the status decide.
func mapBedrockError(attempts *httpAttempts, model, region string, signIn bool, err error) error {
	if limited, ok := rateLimitedPastDeadline(bedrockProvider, attempts, model, err); ok {
		return limited
	}
	if isContextError(err) {
		return transportError(bedrockProvider, err)
	}
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		if status := attempts.statusOrZero(); status >= 400 {
			return apiErrorForModel(bedrockProvider, status, model, "")
		}
		return transportError(bedrockProvider, err)
	}
	status := apiErr.StatusCode
	logger.Debug("llm: error body", "provider", bedrockProvider.id, "status", status,
		"type", awsErrorType(apiErr), "body", logger.Redact(apiErr.RawJSON()))

	var reason string
	switch {
	case status == http.StatusUnauthorized:
		reason = credentialsRejected(signIn)
	case status == http.StatusForbidden:
		switch awsErrorType(apiErr) {
		case "UnrecognizedClientException", "InvalidSignatureException", "IncompleteSignatureException", "MissingAuthenticationTokenException":
			reason = credentialsRejected(signIn)
		case "ExpiredTokenException":
			reason = "the AWS credentials have expired — sign in again (for SSO: aws sso login), then try again"
		default:
			// AccessDeniedException, or no class at all: by far the likeliest
			// cause is model access, which a new account does not have.
			reason = fmt.Sprintf("access to %s in %s was denied — check that model access is enabled for it in the Bedrock console (Model access) and that your key or role may call bedrock:InvokeModel", model, region)
		}
	case status == http.StatusNotFound:
		reason = fmt.Sprintf("the model %s was not found in %s — check the model on the AWS Bedrock card, and that Anthropic's first-time use form is submitted (Bedrock console › Model access)", model, region)
	case status == http.StatusBadRequest && strings.Contains(strings.ToLower(apiErr.RawJSON()), "model identifier"):
		reason = fmt.Sprintf("Bedrock does not know the model %s in %s — check the model ID on the AWS Bedrock card", model, region)
	default:
		return apiErrorForModel(bedrockProvider, status, model, "")
	}
	return &StatusError{Provider: bedrockProvider.name, Status: status,
		msg: fmt.Sprintf("%s error %d: %s", bedrockProvider.name, status, reason)}
}

func credentialsRejected(signIn bool) string {
	if signIn {
		return "AWS did not accept this computer's AWS credentials — check them with aws sts get-caller-identity"
	}
	return "the Bedrock API key was not accepted — check it on the AWS Bedrock card"
}

// bedrockCheckTokens bounds the connection check's answer. It only has to
// prove that a call gets through.
const bedrockCheckTokens = 32

// CheckBedrock sends the smallest request that proves a Bedrock connection
// works — the user's click on the card's Test connection, never automatic —
// and returns the model ID it went to. The text is fixed ("Reply with OK."),
// never the user's. A reply cut off at the tiny limit still proves the
// connection, so it counts as success.
func CheckBedrock(ctx context.Context, cfg Config, model string) (string, error) {
	id, err := BedrockInvokeID(model, cfg.Bedrock.Region, cfg.Bedrock.Routing)
	if err != nil {
		return "", err
	}
	_, err = newBedrock(cfg).Complete(ctx, Request{
		User:       "Reply with OK.",
		Model:      model,
		MaxTokens:  bedrockCheckTokens,
		NoThinking: IsFastTier(model),
	})
	if err != nil && !errors.Is(err, ErrOutputLimit) && !errors.Is(err, ErrOutputLimitThinking) {
		return "", err
	}
	return id, nil
}

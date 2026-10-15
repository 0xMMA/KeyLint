import type { KeyStatus, Settings } from './wails.service';

/**
 * AWS Bedrock's connection as the settings page and the setup wizard edit it.
 * The values and defaults are internal/llm/bedrock.go's (BedrockAuth*,
 * BedrockRouting*) as Settings.Bedrock() in internal/features/settings fills
 * them in: keep the two in sync.
 */
export const BEDROCK_AUTH_KEY = 'api-key';
export const BEDROCK_AUTH_SIGNIN = 'aws-signin';
export const BEDROCK_ROUTING_GEO = 'geo';
export const BEDROCK_ROUTING_GLOBAL = 'global';

/** What SetBedrock saves (settings.BedrockSettings). The key is apart, in the keyring. */
export interface BedrockConnection {
  region: string;
  auth: string;
  profile: string;
  routing: string;
}

/** The saved connection, defaults filled in: a key, kept in the geography. */
export function bedrockConnectionOf(settings: Settings | null | undefined): BedrockConnection {
  const p = settings?.providers;
  return {
    region: p?.aws_region ?? '',
    auth: p?.bedrock_auth === BEDROCK_AUTH_SIGNIN ? BEDROCK_AUTH_SIGNIN : BEDROCK_AUTH_KEY,
    profile: p?.aws_profile ?? '',
    routing: p?.bedrock_routing === BEDROCK_ROUTING_GLOBAL ? BEDROCK_ROUTING_GLOBAL : BEDROCK_ROUTING_GEO,
  };
}

/** `settings` with the connection written into it, everything else as it was. */
export function withBedrockConnection(settings: Settings, c: BedrockConnection): Settings {
  return {
    ...settings,
    providers: {
      ...settings.providers,
      aws_region: c.region,
      bedrock_auth: c.auth,
      aws_profile: c.profile,
      bedrock_routing: c.routing,
    },
  };
}

/** Whether this connection takes a Bedrock API key; see llm.UsesAPIKey. */
export function usesBedrockKey(c: BedrockConnection): boolean {
  return c.auth !== BEDROCK_AUTH_SIGNIN;
}

/**
 * Set up enough to try: a region, and a key unless this computer's AWS
 * sign-in is used — whether that works can only be known by trying (the
 * card's Test connection).
 */
export function bedrockReady(c: BedrockConnection, key: KeyStatus | null | undefined): boolean {
  return !!c.region && (!usesBedrockKey(c) || !!key?.is_set);
}

/** Why Bedrock cannot work yet, or null — including while the key status is loading. */
export function bedrockProblem(c: BedrockConnection, key: KeyStatus | null | undefined): string | null {
  if (!c.region) return "Won't work yet: choose a region.";
  if (usesBedrockKey(c) && key && !key.is_set) return "Won't work yet: no API key is set.";
  return null;
}

/** Routing prefixes an ID can already carry; bedrockRoutingPrefixes in internal/llm/bedrock.go. */
const ROUTING_PREFIXES = new Set(['global', 'us', 'eu', 'au', 'jp', 'apac', 'in', 'ca', 'us-gov']);

/** The model behind a routing prefix ("eu.anthropic.claude-haiku-5-5" → "anthropic.claude-haiku-5-5"). */
export function bedrockBaseModel(id: string): string {
  const dot = id.indexOf('.');
  return dot > 0 && ROUTING_PREFIXES.has(id.slice(0, dot)) ? id.slice(dot + 1) : id;
}

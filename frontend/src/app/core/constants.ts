/**
 * The forms Pyramidize writes (ADR-003). The user always picks one; nothing
 * detects it. 'powerpoint' keeps its old value because saved app presets use
 * it; the form is called Presentation everywhere the user reads it.
 */
export const DOCUMENT_TYPE_OPTIONS: Array<{ label: string; value: string }> = [
  { label: 'Email', value: 'email' },
  { label: 'Chat', value: 'chat' },
  { label: 'Memo', value: 'memo' },
  { label: 'Wiki', value: 'wiki' },
  { label: 'Presentation', value: 'powerpoint' },
];

/**
 * Providers a settings file can name but no dropdown offers yet, with the name
 * to show the user. AWS Bedrock is a backend stub that only returns an error
 * (#22) until #23 implements it. A saved value is explained, never switched:
 * switching on the user's behalf would send their text to a service they
 * never chose.
 */
export const UNAVAILABLE_PROVIDERS: Readonly<Record<string, string>> = {
  bedrock: 'AWS Bedrock',
};

/** The display name when `provider` is known but not usable yet, else null. */
export function unavailableProviderName(provider: string | null | undefined): string | null {
  // hasOwn, so a value like "toString" cannot match an inherited property.
  return provider && Object.hasOwn(UNAVAILABLE_PROVIDERS, provider) ? UNAVAILABLE_PROVIDERS[provider] : null;
}

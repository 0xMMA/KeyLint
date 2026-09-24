/** Base document types supported by the Pyramidize feature. */
export const DOCUMENT_TYPE_OPTIONS: Array<{ label: string; value: string }> = [
  { label: 'Email', value: 'email' },
  { label: 'Wiki', value: 'wiki' },
  { label: 'Memo', value: 'memo' },
  { label: 'PowerPoint', value: 'powerpoint' },
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
  return UNAVAILABLE_PROVIDERS[provider ?? ''] ?? null;
}

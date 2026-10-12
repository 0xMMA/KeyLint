import type { IconName } from '../shared/ui';

/**
 * The five forms Pyramidize writes (ADR-003), in the order the choice page
 * shows them, on the keys 1–5. The user always picks one; nothing detects it.
 * `value` is what the request and the app presets carry; Presentation keeps
 * its old value, 'powerpoint', because saved presets and the CLI already use
 * it. The one list both the choice page and Settings › App defaults read.
 */
export interface PyramidizeForm {
  value: string;
  label: string;
  /** One line under the label on the choice page: what the form is for. */
  description: string;
  icon: IconName;
}

export const PYRAMIDIZE_FORMS: readonly PyramidizeForm[] = [
  { value: 'email', label: 'Email', description: 'A subject that says it all, then headers and bullets', icon: 'mail' },
  { value: 'chat', label: 'Chat', description: 'Teams, Slack: main point first, short', icon: 'message-circle' },
  { value: 'memo', label: 'Memo', description: 'To, from, subject, then the facts', icon: 'file-text' },
  { value: 'wiki', label: 'Wiki', description: 'A page to look things up later', icon: 'book-open' },
  { value: 'powerpoint', label: 'Presentation', description: 'One message per slide, few bullets', icon: 'presentation' },
];

/** The forms as select options (Settings › App defaults). */
export const DOCUMENT_TYPE_OPTIONS: Array<{ label: string; value: string }> =
  PYRAMIDIZE_FORMS.map(({ label, value }) => ({ label, value }));

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

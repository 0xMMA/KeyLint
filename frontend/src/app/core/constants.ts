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

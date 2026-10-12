import type { IconName } from '../../shared/ui';

/**
 * The five forms Pyramidize writes (ADR-003), as the choice page shows them:
 * in this order, on the keys 1–5. `value` is what the request and the app
 * presets carry; Presentation keeps its old value, 'powerpoint', because
 * saved presets and the CLI already use it.
 */
export interface PyramidizeForm {
  value: string;
  label: string;
  /** One line under the label: what the form is for. */
  description: string;
  icon: IconName;
}

export const FORMS: readonly PyramidizeForm[] = [
  { value: 'email', label: 'Email', description: 'A subject that says it all, then headers and bullets', icon: 'mail' },
  { value: 'chat', label: 'Chat', description: 'Teams, Slack: main point first, short', icon: 'message-circle' },
  { value: 'memo', label: 'Memo', description: 'To, from, subject, then the facts', icon: 'file-text' },
  { value: 'wiki', label: 'Wiki', description: 'A page to look things up later', icon: 'book-open' },
  { value: 'powerpoint', label: 'Presentation', description: 'One message per slide, few bullets', icon: 'presentation' },
];

/** The form every session starts on when no app preset says otherwise. */
export const DEFAULT_FORM = 'email';

/**
 * A form's name as the user reads it ("Email"). Accepts the upper-case form a
 * result reports ("EMAIL") and the "presentation" alias; anything unknown is
 * shown as it came.
 */
export function formLabel(value: string): string {
  const v = value.toLowerCase() === 'presentation' ? 'powerpoint' : value.toLowerCase();
  return FORMS.find(f => f.value === v)?.label ?? value;
}

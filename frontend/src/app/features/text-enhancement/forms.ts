import { PYRAMIDIZE_FORMS } from '../../core/constants';

export type { PyramidizeForm } from '../../core/constants';

/** The five forms, as the choice page shows them (one list with Settings › App defaults). */
export const FORMS = PYRAMIDIZE_FORMS;

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

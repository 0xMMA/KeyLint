/**
 * How a stored shortcut ("ctrl+shift+g") reads on screen. One place, so the
 * shortcut recorder and the setup wizard name keys the same way.
 */
export function comboKeys(combo: string): string[] {
  if (!combo) return [];
  return combo.split('+').filter(Boolean).map(p => {
    if (p === 'ctrl') return 'Ctrl';
    if (p === 'shift') return 'Shift';
    if (p === 'alt') return 'Alt';
    if (p === 'win') return 'Win';
    return p.length === 1 ? p.toUpperCase() : p.charAt(0).toUpperCase() + p.slice(1).toUpperCase();
  });
}

/** "ctrl+g" → "Ctrl + G". */
export function formatCombo(combo: string): string {
  return comboKeys(combo).join(' + ');
}

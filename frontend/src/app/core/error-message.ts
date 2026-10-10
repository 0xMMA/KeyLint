/**
 * The text to show for an error from a Go binding call.
 *
 * When a bound Go method returns an error, Wails (v3 alpha) answers the call
 * with its CallError as JSON — `{"message":"…","kind":"RuntimeError"}` — and
 * the runtime rejects with an `Error` whose message is that JSON text, not the
 * Go error's text. This unwraps it; anything else comes through unchanged.
 */
export function describeError(e: unknown): string {
  const raw = e instanceof Error ? e.message : String(e);
  if (raw.startsWith('{')) {
    try {
      const parsed: unknown = JSON.parse(raw);
      const message = (parsed as { message?: unknown } | null)?.message;
      if (typeof message === 'string') {
        return message;
      }
    } catch {
      // Not JSON after all: show it as it is.
    }
  }
  return raw;
}

/**
 * Wording for where a model picker's list came from.
 *
 * Shared by Settings and the Pyramidize panel so the two cannot tell a user
 * different stories about the same provider. The source values are the ones
 * `internal/llm/models.go` sets.
 */

/** A provider that answered but listed nothing needs its own sentence. */
function emptyNote(provider: string): string {
  // Ollama is the case this exists for: `ollama serve` with nothing pulled is
  // a running daemon, so "could not be reached" would send the user hunting
  // for a problem that is not there.
  return provider === 'ollama'
    ? 'No models pulled yet — pull one with `ollama pull`.'
    : 'This account lists no models.';
}

/**
 * What to tell the user about this list, or "" when it needs no explaining.
 *
 * `hasList` says whether there is anything to show. It changes the wording
 * rather than silencing the note: a provider with no curated entries still owes
 * the user an explanation for the empty picker, and promising a "built-in list"
 * that is not on screen would be its own small lie.
 */
export function noteForModelSource(source: string, provider: string, hasList: boolean): string {
  switch (source) {
    case 'unreachable':
      return hasList
        ? "Showing KeyLint's built-in list — the provider could not be reached."
        : 'The provider could not be reached.';
    case 'no-credentials':
      return hasList
        ? "Showing KeyLint's built-in list — add a key to load the provider's own."
        : 'Add a key to load this provider\'s models.';
    case 'unusable':
      // The provider did list models; this app just cannot call any of them.
      // Saying "lists no models" here would be false.
      return hasList
        ? "Showing KeyLint's built-in list — none of this account's models can be used here."
        : "None of this account's models can be used here.";
    case 'empty':
      return emptyNote(provider);
    default:
      // "live" needs no note, and "fixed" — the Claude Code CLI, which has no
      // model endpoint — would be an alarm nobody can clear.
      return '';
  }
}

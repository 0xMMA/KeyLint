/**
 * Wording for where a model picker's list came from.
 *
 * Settings' wording for every provider (the Pyramidize page has no model
 * picker since its choice page), kept in one place so a user is never told
 * different stories about the same provider. The source values are the ones
 * `internal/llm/models.go` sets.
 */

/** A provider that answered but listed nothing needs its own sentence. */
function emptyNote(provider: string): string {
  // Ollama is the case this exists for: `ollama serve` with nothing pulled is
  // a running daemon, so "could not be reached" would send the user hunting
  // for a problem that is not there.
  switch (provider) {
    case 'ollama': return 'No models pulled yet — pull one with `ollama pull`.';
    case 'openrouter': return 'OpenRouter lists no models right now.';
    default: return 'This account lists no models.';
  }
}

/**
 * Whose list it is. OpenRouter's catalogue is public — the same for everyone
 * and listed without a key — so calling it "this account's" would be false.
 */
function listOwner(provider: string): string {
  return provider === 'openrouter' ? "OpenRouter's" : "this account's";
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
    case 'unusable': {
      // The provider did list models; this app just cannot call any of them.
      // Saying "lists no models" here would be false.
      const owner = listOwner(provider);
      return hasList
        ? `Showing KeyLint's built-in list — none of ${owner} models can be used here.`
        : `None of ${owner} models can be used here.`;
    }
    case 'empty':
      return emptyNote(provider);
    case 'live':
      // OpenRouter's catalogue is listed without a key, so a live list says
      // nothing about whether the user's key can call a model — and at ~450
      // entries the picker needs saying that it searches.
      return provider === 'openrouter'
        ? "OpenRouter's whole catalogue — type in a model field to search it. Which models your key can call depends on your OpenRouter account and credit."
        : '';
    default:
      // "fixed" — the Claude Code CLI, which has no model endpoint — would be
      // an alarm nobody can clear.
      return '';
  }
}

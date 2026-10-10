import type { ModelInfo } from '../../core/wails.service';

/** The two features whose model and effort are chosen per provider. */
export type Feature = 'fix' | 'pyramidize';

/**
 * One entry in a model picker.
 *
 * `label` is deliberately the model ID, not the display name: the editable
 * kl-select (like PrimeNG's p-select before it) writes optionLabel into the input and submits whatever stands there as the
 * value, so a display name there would be saved as a model ID that no provider
 * knows. The readable name lives in `display` and is rendered by the option
 * template instead; `secondary` is the muted line under it — the ID a family
 * alias resolves to today, or a pinned model's ID when its name differs.
 */
export interface ModelOption {
  /** null for the "KeyLint default" entry; see defaultOption. */
  id: string | null;
  label: string;
  display: string;
  secondary: string;
}

/**
 * KeyLint's built-in default per provider and feature, so the picker can say
 * what "default" means today. The source of truth is `defaultModels` in
 * internal/llm/models.go: keep the two in sync, or the picker names a model the
 * backend does not use.
 */
export const DEFAULT_MODELS: Readonly<Record<string, Readonly<Record<Feature, string>>>> = {
  claude: { fix: 'haiku', pyramidize: 'sonnet' },
  'claude-code': { fix: 'haiku', pyramidize: 'sonnet' },
  openai: { fix: 'gpt-6-luna', pyramidize: 'gpt-6-astra' },
  ollama: { fix: 'llama3.2', pyramidize: 'llama3.2' },
};

/** Picker entry for one model the provider reported. */
export function toModelOption(model: ModelInfo): ModelOption {
  const display = model.label || model.id;
  // An alias's own ID ("sonnet") says nothing its name does not; what is worth
  // the second line is the model it stands for, and only where that is known.
  const secondary = FAMILY_ALIASES.has(model.id)
    ? model.resolved
    : model.resolved || (model.id !== display ? model.id : '');
  return { id: model.id, label: model.id, display, secondary };
}

/** The family aliases; see modelFamilies in internal/llm/families.go. */
const FAMILY_ALIASES = new Set(['opus', 'sonnet', 'haiku', 'fable']);

/**
 * An alias as the backend labels it ("Haiku (latest)", familyLabel in
 * internal/llm/families.go), for when no list has arrived to say so; any other
 * ID as it is.
 */
function aliasLabel(id: string): string {
  return FAMILY_ALIASES.has(id) ? `${id[0].toUpperCase()}${id.slice(1)} (latest)` : id;
}

/**
 * A model's readable name: the provider's own label where its listing has the
 * ID, the alias label otherwise ("Haiku (latest)"), or the ID as it is. For
 * places that name the model in use, like the Fix page's "Sent to" chip.
 */
export function modelName(id: string, listed: ModelOption[]): string {
  return listed.find(o => o.id === id)?.display ?? aliasLabel(id);
}

/** The model a feature runs on a provider: the stored choice, else KeyLint's default. */
export function effectiveModel(provider: string, feature: Feature, stored: string | undefined): string {
  return stored || DEFAULT_MODELS[provider]?.[feature] || '';
}

/**
 * The leading "KeyLint default" entry for one feature, naming what the default
 * is today.
 *
 * Its value is null, not "": the editable select shows the placeholder only
 * while the value is null (p-select's rule, kept by kl-select), and the picker passes null for a stored "" — so the empty editable
 * field reads "Default: Haiku (latest)" in placeholder grey, and typing
 * still starts from nothing rather than being appended to a word. The entry is
 * also a real option, so the default can be chosen back from the list. The
 * picker turns a null choice back into "" before it is stored.
 */
export function defaultOption(provider: string, feature: Feature, listed: ModelOption[]): ModelOption {
  const id = DEFAULT_MODELS[provider]?.[feature] ?? '';
  const match = listed.find(o => o.id === id);
  const name = match?.display ?? aliasLabel(id);
  return {
    id: null,
    label: '',
    display: name ? `Default: ${name}` : 'Default',
    secondary: match?.secondary ?? '',
  };
}

/** One effort choice. "" sends nothing, which leaves the model on its default. */
export interface EffortOption {
  value: string;
  label: string;
}

/**
 * The levels the Anthropic API and the Claude Code CLI accept (EffortLow …
 * EffortMax in internal/llm/llm.go). A model that does not take a level is not
 * sent it — the backend checks — so the list does not change per model.
 */
export const EFFORT_OPTIONS: readonly EffortOption[] = [
  { value: '', label: 'Model default' },
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
  { value: 'xhigh', label: 'Extra high' },
  { value: 'max', label: 'Max' },
];

/**
 * What effort does on each provider, or null where it does nothing (Ollama).
 * Kept to what the backend actually does with it.
 */
export function effortNote(provider: string): string | null {
  switch (provider) {
    case 'claude':
      return 'More effort means more reasoning: better on hard text, but slower. A level a model does not take is left out.';
    case 'claude-code':
      return 'More effort means more reasoning: better on hard text, but slower. The CLI applies it to the model behind the alias.';
    case 'openai':
      return 'More effort means more reasoning: better on hard text, but slower. Only GPT-6 and GPT-5.6 models take it; others ignore it.';
    default:
      return null;
  }
}

/**
 * Whether Fix asks this model to answer without reasoning first when no effort
 * is chosen. Mirrors `IsFastTier` in internal/llm/families.go, and only the
 * providers that have a switch for it — the Claude Code CLI has none.
 */
export function fixSkipsReasoning(provider: string, model: string): boolean {
  if (provider !== 'claude' && provider !== 'openai') return false;
  return model === 'haiku' || model.startsWith('claude-haiku-')
    || model.startsWith('gpt-6-luna') || model.startsWith('gpt-5.6-luna');
}

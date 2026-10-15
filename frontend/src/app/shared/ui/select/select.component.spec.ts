import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { Component, signal, ChangeDetectionStrategy } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { SelectComponent } from './select.component';
import { defaultOption, toModelOption, type ModelOption } from '../../../features/settings/model-options';

/**
 * kl-select replaces PrimeNG's p-select, including its [editable] mode, whose
 * null-vs-"" behaviour features/settings/model-options.ts is written against:
 *
 * - the "KeyLint default" entry has the value null and the label "", so an
 *   empty editable field shows the placeholder ("Default: Haiku (latest)");
 * - the picker passes `stored || null` in and turns a null choice back into
 *   "" (`$event ?? ''`) before storing;
 * - optionLabel is the model ID, because the editable field submits whatever
 *   stands in it as the value.
 *
 * The first describe pins those down; the rest is the plain select.
 */

const LISTED: ModelOption[] = [
  toModelOption({ id: 'sonnet', label: 'Sonnet (latest)', resolved: 'claude-sonnet-5-5' }),
  toModelOption({ id: 'claude-sonnet-4-6', label: 'Claude Sonnet 4.6', resolved: '' }),
];
const MODEL_OPTIONS: ModelOption[] = [defaultOption('claude', 'fix', LISTED), ...LISTED];

/** The settings page's model picker, wired exactly like active-provider does. */
@Component({
  standalone: true,
  imports: [SelectComponent, FormsModule],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <kl-select
      data-testid="picker"
      [editable]="editable()"
      [options]="options"
      optionLabel="label"
      optionValue="id"
      optionDisplay="display"
      optionDescription="secondary"
      [placeholder]="options[0].display"
      ariaLabel="Fix model"
      [ngModel]="stored() || null"
      (ngModelChange)="emitted.push($event); stored.set($event ?? '')"
    />
  `,
})
class ModelPickerHost {
  readonly options = MODEL_OPTIONS;
  readonly editable = signal(true);
  readonly stored = signal('');
  readonly emitted: Array<string | null> = [];
}

/** A long catalogue in OpenRouter's shape: a name, and the vendor/model ID under it. */
const CATALOGUE: ModelOption[] = [
  { id: null, label: '', display: 'Default: Anthropic: Claude Haiku 5.5', secondary: 'anthropic/claude-haiku-5.5' },
  ...[
    ['anthropic/claude-haiku-5.5', 'Anthropic: Claude Haiku 5.5'],
    ['anthropic/claude-sonnet-5.5', 'Anthropic: Claude Sonnet 5.5'],
    ['openai/gpt-6-luna', 'OpenAI: GPT-6 Luna'],
    ['google/gemini-3.8-flash', 'Google: Gemini 3.8 Flash'],
    ['anthropic/claude-haiku-4.5', 'Anthropic: Claude Haiku 4.5'],
  ].map(([id, label]) => toModelOption({ id, label, resolved: '' })),
];

/** A model picker that searches as you type: OpenRouter's, with its ~450 entries. */
@Component({
  standalone: true,
  imports: [SelectComponent, FormsModule],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <kl-select
      data-testid="search"
      editable
      filter
      [options]="options"
      optionLabel="label"
      optionValue="id"
      optionDisplay="display"
      optionDescription="secondary"
      [placeholder]="options[0].display"
      ariaLabel="Fix model"
      [ngModel]="stored() || null"
      (ngModelChange)="emitted.push($event); stored.set($event ?? '')"
    />
    <button data-testid="elsewhere">elsewhere</button>
  `,
})
class SearchHost {
  readonly options = CATALOGUE;
  readonly stored = signal('');
  readonly emitted: Array<string | null> = [];
}

const LEVELS = [
  { label: 'Off', value: 'off' },
  { label: 'Debug', value: 'debug' },
  { label: 'Info', value: 'info' },
  { label: 'Warn', value: 'warn', disabled: true },
  { label: 'Error', value: 'error' },
];

/** A fixed list, as the log level and Pyramidize pickers use it. */
@Component({
  standalone: true,
  imports: [SelectComponent, FormsModule],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <kl-select
      data-testid="level"
      [options]="levels"
      optionLabel="label"
      optionValue="value"
      placeholder="Choose a level"
      ariaLabel="Log level"
      [disabled]="disabled()"
      [(ngModel)]="level"
      (changed)="changes.push($event)"
    />
    <button data-testid="elsewhere">elsewhere</button>
  `,
})
class LevelHost {
  readonly levels = LEVELS;
  readonly disabled = signal(false);
  readonly level = signal<string | null>('debug');
  readonly changes: unknown[] = [];
}

/** Named only by an outside <label for>, as the Pyramidize pickers are. */
@Component({
  standalone: true,
  imports: [SelectComponent, FormsModule],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <label data-testid="label" for="level-input"><span>Log level</span></label>
    <kl-select data-testid="level" inputId="level-input" [options]="levels" optionLabel="label" optionValue="value"
      [(ngModel)]="level" (changed)="changes.push($event)" />
  `,
})
class LabelledHost {
  readonly levels = LEVELS;
  readonly level = signal<string | null>('debug');
  readonly changes: unknown[] = [];
}

async function settle<T>(fixture: ComponentFixture<T>): Promise<void> {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

function listbox(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[role="listbox"]');
}

function options(): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>('[role="option"]'));
}

function key(el: HTMLElement, k: string): void {
  el.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }));
}

function type(input: HTMLInputElement, text: string): void {
  input.value = text;
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

afterEach(() => {
  // The overlay container lives on <body>, outside the fixture; spec files
  // share a document (isolate: false), so leave nothing behind.
  document.querySelectorAll('.cdk-overlay-container').forEach(c => (c.innerHTML = ''));
});

describe('kl-select, editable: the model picker semantics', () => {
  let fixture: ComponentFixture<ModelPickerHost>;
  let host: ModelPickerHost;
  let input: HTMLInputElement;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [ModelPickerHost] }).compileComponents();
    fixture = TestBed.createComponent(ModelPickerHost);
    host = fixture.componentInstance;
    await settle(fixture);
    input = fixture.nativeElement.querySelector('[data-testid="picker"] input[role="combobox"]');
  });

  it('renders a text combobox with the placeholder naming the default', () => {
    expect(input).toBeTruthy();
    expect(input.getAttribute('aria-autocomplete')).toBe('list');
    expect(input.getAttribute('aria-label')).toBe('Fix model');
    expect(input.placeholder).toBe('Default: Haiku (latest)');
  });

  it('shows an empty field (placeholder) for a stored "", which arrives as null', async () => {
    host.stored.set('');
    await settle(fixture);
    expect(input.value).toBe('');
  });

  it('shows the model ID (optionLabel) for a stored model that is listed', async () => {
    host.stored.set('sonnet');
    await settle(fixture);
    expect(input.value).toBe('sonnet');
  });

  it('shows a stored model that is not listed as it is', async () => {
    host.stored.set('my-finetune-7');
    await settle(fixture);
    expect(input.value).toBe('my-finetune-7');
  });

  it('submits typed text as the value, keystroke by keystroke', async () => {
    type(input, 'gpt');
    type(input, 'gpt-x');
    await settle(fixture);
    expect(host.emitted).toEqual(['gpt', 'gpt-x']);
    expect(host.stored()).toBe('gpt-x');
    expect(input.value).toBe('gpt-x');
  });

  it('reports a cleared field as "", never null, and shows the placeholder again', async () => {
    host.stored.set('sonnet');
    await settle(fixture);
    type(input, '');
    await settle(fixture);
    expect(host.emitted).toEqual(['']);
    expect(host.stored()).toBe('');
    expect(input.value).toBe('');
  });

  it('reports the default entry as null, which the picker stores as ""', async () => {
    host.stored.set('sonnet');
    await settle(fixture);
    key(input, 'ArrowDown');
    await settle(fixture);
    const def = options().find(o => o.textContent?.includes('Default: Haiku (latest)'))!;
    def.click();
    await settle(fixture);
    expect(host.emitted).toEqual([null]);
    expect(host.stored()).toBe('');
    expect(input.value).toBe('');
    expect(listbox()).toBeNull();
  });

  it('writes the chosen option’s ID into the field', async () => {
    key(input, 'ArrowDown');
    await settle(fixture);
    options().find(o => o.textContent?.includes('Claude Sonnet 4.6'))!.click();
    await settle(fixture);
    expect(host.emitted).toEqual(['claude-sonnet-4-6']);
    expect(input.value).toBe('claude-sonnet-4-6');
  });

  it('lists every option with its name and the muted second line, not filtered by the typed text', async () => {
    type(input, 'zzz');
    key(input, 'ArrowDown');
    await settle(fixture);
    const rows = options();
    expect(rows.length).toBe(3);
    const sonnet = rows[1];
    expect(sonnet.querySelector('.kl-option-label')?.textContent?.trim()).toBe('Sonnet (latest)');
    expect(sonnet.querySelector('.kl-option-description')?.textContent?.trim()).toBe('claude-sonnet-5-5');
  });

  it('marks the default entry as selected while the stored value is ""', async () => {
    key(input, 'ArrowDown');
    await settle(fixture);
    expect(options()[0].getAttribute('aria-selected')).toBe('true');
    expect(options()[1].getAttribute('aria-selected')).toBe('false');
  });

  it('chooses with the keyboard: ArrowDown opens, arrows move, Enter takes it', async () => {
    key(input, 'ArrowDown');
    await settle(fixture);
    expect(input.getAttribute('aria-expanded')).toBe('true');
    key(input, 'ArrowDown');
    await settle(fixture);
    const active = input.getAttribute('aria-activedescendant');
    expect(active).toBe(options()[1].id);
    key(input, 'Enter');
    await settle(fixture);
    expect(host.emitted).toEqual(['sonnet']);
    expect(input.getAttribute('aria-expanded')).toBe('false');
  });

  it('Enter keeps typed text that matches no option and closes the list', async () => {
    host.stored.set('sonnet');
    await settle(fixture);
    key(input, 'ArrowDown');
    await settle(fixture);
    expect(input.getAttribute('aria-activedescendant')).toBe(options()[1].id);
    type(input, 'my-finetune-7');
    await settle(fixture);
    expect(input.getAttribute('aria-activedescendant'), 'no option is active').toBeNull();
    key(input, 'Enter');
    await settle(fixture);
    expect(host.stored()).toBe('my-finetune-7');
    expect(host.emitted).toEqual(['my-finetune-7']);
    expect(input.value).toBe('my-finetune-7');
    expect(listbox()).toBeNull();
  });

  it('Enter keeps typed text from the default entry too (never stores "")', async () => {
    key(input, 'ArrowDown');
    await settle(fixture);
    expect(input.getAttribute('aria-activedescendant')).toBe(options()[0].id);
    type(input, 'gpt-x');
    key(input, 'Enter');
    await settle(fixture);
    expect(host.stored()).toBe('gpt-x');
    expect(host.emitted).toEqual(['gpt-x']);
    expect(listbox()).toBeNull();
  });

  it('opening on a stored free-text model activates nothing, so Enter keeps it', async () => {
    host.stored.set('my-finetune-7');
    await settle(fixture);
    key(input, 'ArrowDown');
    await settle(fixture);
    expect(listbox()).toBeTruthy();
    expect(input.getAttribute('aria-activedescendant')).toBeNull();
    key(input, 'Enter');
    await settle(fixture);
    expect(host.stored()).toBe('my-finetune-7');
    expect(host.emitted).toEqual([]);
    expect(listbox()).toBeNull();
    // ArrowDown from nothing active goes to the first option.
    key(input, 'ArrowDown');
    key(input, 'ArrowDown');
    await settle(fixture);
    expect(input.getAttribute('aria-activedescendant')).toBe(options()[0].id);
  });

  it('typed text that is an option’s ID activates that option, and Enter takes it', async () => {
    key(input, 'ArrowDown');
    await settle(fixture);
    type(input, 'claude-sonnet-4-6');
    await settle(fixture);
    expect(input.getAttribute('aria-activedescendant')).toBe(options()[2].id);
    key(input, 'Enter');
    await settle(fixture);
    expect(host.stored()).toBe('claude-sonnet-4-6');
    expect(input.value).toBe('claude-sonnet-4-6');
    expect(listbox()).toBeNull();
  });

  it('Escape closes the list and keeps the typed text', async () => {
    type(input, 'custom');
    key(input, 'ArrowDown');
    await settle(fixture);
    key(input, 'Escape');
    await settle(fixture);
    expect(listbox()).toBeNull();
    expect(host.stored()).toBe('custom');
    expect(input.value).toBe('custom');
  });

  it('as a fixed list (not editable) shows the default entry’s name, not an empty field', async () => {
    host.editable.set(false);
    await settle(fixture);
    const trigger = fixture.nativeElement.querySelector('[data-testid="picker"] [role="combobox"]') as HTMLElement;
    expect(trigger.tagName).toBe('BUTTON');
    expect(trigger.textContent).toContain('Default: Haiku (latest)');
  });
});

describe('kl-select, editable with filter: searching a long list', () => {
  let fixture: ComponentFixture<SearchHost>;
  let host: SearchHost;
  let input: HTMLInputElement;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [SearchHost] }).compileComponents();
    fixture = TestBed.createComponent(SearchHost);
    host = fixture.componentInstance;
    await settle(fixture);
    input = fixture.nativeElement.querySelector('[data-testid="search"] input[role="combobox"]');
  });

  const names = () => options().map(o => o.querySelector('.kl-option-label')?.textContent?.trim());

  it('opens on typing and keeps the options whose name or ID holds every typed word, in any case', async () => {
    type(input, 'HAIKU');
    await settle(fixture);
    expect(listbox()).toBeTruthy();
    expect(names()).toEqual(['Default: Anthropic: Claude Haiku 5.5', 'Anthropic: Claude Haiku 5.5', 'Anthropic: Claude Haiku 4.5']);

    type(input, 'haiku 4.5');
    await settle(fixture);
    expect(names()).toEqual(['Anthropic: Claude Haiku 4.5']);

    // The ID, shown on the second line, is searched too.
    type(input, 'openai/');
    await settle(fixture);
    expect(names()).toEqual(['OpenAI: GPT-6 Luna']);
  });

  it('shows the whole list when opened on a stored model, so the choice can be browsed', async () => {
    host.stored.set('openai/gpt-6-luna');
    await settle(fixture);
    key(input, 'ArrowDown');
    await settle(fixture);
    expect(options().length).toBe(CATALOGUE.length);
    expect(input.getAttribute('aria-activedescendant')).toBe(options()[3].id);
  });

  it('takes the first match with ArrowDown and Enter, writing its ID into the field', async () => {
    type(input, 'gemini');
    key(input, 'ArrowDown');
    await settle(fixture);
    key(input, 'Enter');
    await settle(fixture);
    expect(host.stored()).toBe('google/gemini-3.8-flash');
    expect(input.value).toBe('google/gemini-3.8-flash');
    expect(listbox()).toBeNull();
  });

  it('takes the clicked match, not the option that was at that position before filtering', async () => {
    type(input, 'sonnet');
    await settle(fixture);
    options()[0].click();
    await settle(fixture);
    expect(host.stored()).toBe('anthropic/claude-sonnet-5.5');
  });

  it('still keeps free text: Enter on a search that names no option keeps what was typed', async () => {
    type(input, 'vendor/brand-new-model');
    await settle(fixture);
    expect(names()).toEqual([]);
    expect(document.querySelector('.kl-option-empty')?.textContent?.trim()).toBe('No matches');
    expect(host.emitted, 'typing alone commits nothing').toEqual([]);
    key(input, 'Enter');
    await settle(fixture);
    expect(host.stored()).toBe('vendor/brand-new-model');
    expect(host.emitted).toEqual(['vendor/brand-new-model']);
    expect(listbox()).toBeNull();
  });

  it('makes the option the typed ID names active, so Enter takes it', async () => {
    type(input, 'anthropic/claude-haiku-4.5');
    await settle(fixture);
    expect(names()).toEqual(['Anthropic: Claude Haiku 4.5']);
    expect(input.getAttribute('aria-activedescendant')).toBe(options()[0].id);
    // Selected means stored: nothing is, until it is taken.
    expect(options()[0].getAttribute('aria-selected')).toBe('false');
    key(input, 'Enter');
    await settle(fixture);
    expect(host.emitted).toEqual(['anthropic/claude-haiku-4.5']);
  });

  // Review of #134: with every keystroke committed, a search the user gave up
  // on ("gpt lu", then a click elsewhere) became the saved model — an ID no
  // provider knows. Typing here is a search; only a choice commits.
  describe('typing is a search, not a choice', () => {
    beforeEach(async () => {
      host.stored.set('openai/gpt-6-luna');
      await settle(fixture);
    });

    it('commits nothing while typing', async () => {
      type(input, 'gem');
      type(input, 'gemini');
      await settle(fixture);
      expect(host.emitted).toEqual([]);
      expect(host.stored()).toBe('openai/gpt-6-luna');
      expect(input.value).toBe('gemini');
    });

    it('Escape puts the stored model back in the field', async () => {
      type(input, 'gem');
      await settle(fixture);
      key(input, 'Escape');
      await settle(fixture);
      expect(listbox()).toBeNull();
      expect(input.value).toBe('openai/gpt-6-luna');
      expect(host.emitted).toEqual([]);
    });

    it('leaving the field puts the stored model back', async () => {
      type(input, 'gem');
      await settle(fixture);
      input.dispatchEvent(new FocusEvent('blur'));
      await settle(fixture);
      expect(listbox()).toBeNull();
      expect(input.value).toBe('openai/gpt-6-luna');
      expect(host.emitted).toEqual([]);
    });

    it('a click elsewhere puts the stored model back', async () => {
      type(input, 'gem');
      await settle(fixture);
      expect(listbox()).toBeTruthy();
      const elsewhere = fixture.nativeElement.querySelector('[data-testid="elsewhere"]') as HTMLElement;
      elsewhere.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }));
      elsewhere.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      await settle(fixture);
      expect(listbox()).toBeNull();
      expect(input.value).toBe('openai/gpt-6-luna');
      expect(host.emitted).toEqual([]);
    });

    it('a click on a match commits it', async () => {
      type(input, 'gem');
      await settle(fixture);
      options()[0].click();
      await settle(fixture);
      expect(host.emitted).toEqual(['google/gemini-3.8-flash']);
      expect(input.value).toBe('google/gemini-3.8-flash');
    });
  });

});

describe('kl-select, fixed list', () => {
  let fixture: ComponentFixture<LevelHost>;
  let host: LevelHost;
  let trigger: HTMLButtonElement;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [LevelHost] }).compileComponents();
    fixture = TestBed.createComponent(LevelHost);
    host = fixture.componentInstance;
    await settle(fixture);
    trigger = fixture.nativeElement.querySelector('[data-testid="level"] [role="combobox"]');
  });

  it('is a button that says it opens a listbox, labelled and showing the value', () => {
    expect(trigger.tagName).toBe('BUTTON');
    expect(trigger.getAttribute('aria-haspopup')).toBe('listbox');
    expect(trigger.getAttribute('aria-expanded')).toBe('false');
    expect(trigger.getAttribute('aria-label')).toBe('Log level');
    expect(trigger.textContent).toContain('Debug');
  });

  it('shows the placeholder when the value matches no option', async () => {
    host.level.set(null);
    await settle(fixture);
    expect(trigger.textContent).toContain('Choose a level');
  });

  it('opens the list on <body> (outside the component), wired to the trigger', async () => {
    trigger.click();
    await settle(fixture);
    const list = listbox()!;
    expect(list).toBeTruthy();
    expect(fixture.nativeElement.contains(list)).toBe(false);
    expect(trigger.getAttribute('aria-expanded')).toBe('true');
    expect(trigger.getAttribute('aria-controls')).toBe(list.id);
    expect(options().map(o => o.textContent?.trim())).toEqual(['Off', 'Debug', 'Info', 'Warn', 'Error']);
  });

  it('takes a clicked option, closes, emits once and returns focus to the trigger', async () => {
    trigger.click();
    await settle(fixture);
    options()[2].click();
    await settle(fixture);
    expect(host.level()).toBe('info');
    expect(host.changes).toEqual(['info']);
    expect(listbox()).toBeNull();
    expect(document.activeElement).toBe(trigger);
    expect(trigger.textContent).toContain('Info');
  });

  it('does not take a disabled option', async () => {
    trigger.click();
    await settle(fixture);
    const warn = options()[3];
    expect(warn.getAttribute('aria-disabled')).toBe('true');
    warn.click();
    await settle(fixture);
    expect(host.level()).toBe('debug');
  });

  it('starts on the selected option and skips disabled ones with the arrows', async () => {
    key(trigger, 'ArrowDown');
    await settle(fixture);
    expect(trigger.getAttribute('aria-activedescendant')).toBe(options()[1].id);
    key(trigger, 'ArrowDown');
    key(trigger, 'ArrowDown');
    await settle(fixture);
    expect(trigger.getAttribute('aria-activedescendant')).toBe(options()[4].id);
    key(trigger, 'Home');
    await settle(fixture);
    expect(trigger.getAttribute('aria-activedescendant')).toBe(options()[0].id);
    key(trigger, 'End');
    key(trigger, 'Enter');
    await settle(fixture);
    expect(host.level()).toBe('error');
  });

  it('Escape closes without a change', async () => {
    key(trigger, 'ArrowDown');
    key(trigger, 'ArrowDown');
    key(trigger, 'Escape');
    await settle(fixture);
    expect(listbox()).toBeNull();
    expect(host.level()).toBe('debug');
    expect(host.changes).toEqual([]);
  });

  it('closes on a click outside', async () => {
    trigger.click();
    await settle(fixture);
    expect(listbox()).toBeTruthy();
    const elsewhere = fixture.nativeElement.querySelector('[data-testid="elsewhere"]') as HTMLElement;
    elsewhere.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }));
    elsewhere.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await settle(fixture);
    expect(listbox()).toBeNull();
  });

  it('picking the option that is already selected changes nothing and emits nothing', async () => {
    const writes: unknown[] = [];
    fixture.debugElement.children[0].injector.get(SelectComponent).registerOnChange(v => writes.push(v));
    trigger.click();
    await settle(fixture);
    options()[1].click();
    await settle(fixture);
    expect(host.level()).toBe('debug');
    expect(host.changes).toEqual([]);
    expect(writes).toEqual([]);
    expect(listbox()).toBeNull();
    expect(document.activeElement).toBe(trigger);

    key(trigger, 'Enter');
    await settle(fixture);
    key(trigger, 'Enter');
    await settle(fixture);
    expect(host.changes).toEqual([]);
    expect(listbox()).toBeNull();
  });

  it('does not open while disabled', async () => {
    host.disabled.set(true);
    await settle(fixture);
    expect(trigger.disabled).toBe(true);
    trigger.click();
    key(trigger, 'ArrowDown');
    await settle(fixture);
    expect(listbox()).toBeNull();
  });
});

describe('kl-select, the open panel', () => {
  let fixture: ComponentFixture<LevelHost>;
  let trigger: HTMLButtonElement;
  const original = Element.prototype.scrollIntoView;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [LevelHost] }).compileComponents();
    fixture = TestBed.createComponent(LevelHost);
    fixture.componentInstance.level.set('error');
    await settle(fixture);
    trigger = fixture.nativeElement.querySelector('[data-testid="level"] [role="combobox"]');
  });

  afterEach(() => {
    Element.prototype.scrollIntoView = original;
  });

  it('scrolls the selected option into view once the options have rendered', async () => {
    const scrolled: string[] = [];
    Element.prototype.scrollIntoView = function (this: Element) { scrolled.push(this.textContent?.trim() ?? ''); };
    trigger.click();
    await settle(fixture);
    expect(options().length).toBe(5);
    expect(scrolled).toContain('Error');
  });

  it('follows the trigger’s width when the window is resized', async () => {
    const host = fixture.nativeElement.querySelector('[data-testid="level"]') as HTMLElement;
    let width = 240;
    host.getBoundingClientRect = () => ({ width, height: 40, top: 0, left: 0, right: width, bottom: 40, x: 0, y: 0, toJSON() {} }) as DOMRect;
    trigger.click();
    await settle(fixture);
    const pane = () => document.querySelector<HTMLElement>('.kl-select-overlay')!;
    expect(pane().style.width).toBe('240px');
    width = 400;
    window.dispatchEvent(new Event('resize'));
    await new Promise(r => setTimeout(r, 150));
    await settle(fixture);
    expect(pane().style.width).toBe('400px');
  });
});

describe('kl-select, named by a <label for>', () => {
  let fixture: ComponentFixture<LabelledHost>;
  let host: LabelledHost;
  let trigger: HTMLButtonElement;
  let label: HTMLLabelElement;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [LabelledHost] }).compileComponents();
    fixture = TestBed.createComponent(LabelledHost);
    host = fixture.componentInstance;
    document.body.appendChild(fixture.nativeElement);
    await settle(fixture);
    trigger = fixture.nativeElement.querySelector('[data-testid="level"] [role="combobox"]');
    label = fixture.nativeElement.querySelector('[data-testid="label"]');
  });

  afterEach(() => fixture.nativeElement.remove());

  it('is the label’s control', () => {
    expect(trigger.id).toBe('level-input');
    expect(Array.from(trigger.labels)).toEqual([label]);
  });

  it('clicking the label focuses the trigger without opening the list', async () => {
    (label.querySelector('span') as HTMLElement).click();
    await settle(fixture);
    expect(document.activeElement).toBe(trigger);
    expect(listbox()).toBeNull();
    expect(trigger.getAttribute('aria-expanded')).toBe('false');
    expect(host.changes).toEqual([]);
  });

  it('still opens on a click on the trigger itself', async () => {
    trigger.click();
    await settle(fixture);
    expect(listbox()).toBeTruthy();
  });

  it('names the listbox with the label’s text when no aria name is given', async () => {
    trigger.click();
    await settle(fixture);
    expect(listbox()!.getAttribute('aria-label')).toBe('Log level');
    expect(listbox()!.hasAttribute('aria-labelledby')).toBe(false);
  });
});

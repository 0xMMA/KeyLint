import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { ActiveProviderComponent, FeatureChoice } from './active-provider.component';
import { ModelOption } from '../model-options';

const OPTIONS: ModelOption[] = [
  { id: null, label: '', display: 'Default: Haiku (latest)', secondary: 'claude-haiku-5-5' },
  { id: 'sonnet', label: 'sonnet', display: 'Sonnet (latest)', secondary: 'claude-sonnet-5-5' },
  { id: 'claude-sonnet-4-6', label: 'claude-sonnet-4-6', display: 'Claude Sonnet 4.6', secondary: 'claude-sonnet-4-6' },
];

describe('ActiveProviderComponent', () => {
  let fixture: ComponentFixture<ActiveProviderComponent>;
  let el: HTMLElement;

  async function render(inputs: Record<string, unknown>): Promise<void> {
    fixture = TestBed.createComponent(ActiveProviderComponent);
    fixture.componentRef.setInput('providerId', 'claude');
    fixture.componentRef.setInput('label', 'Anthropic API');
    fixture.componentRef.setInput('fixOptions', OPTIONS);
    fixture.componentRef.setInput('pyramidizeOptions', OPTIONS);
    for (const [name, value] of Object.entries(inputs)) {
      fixture.componentRef.setInput(name, value);
    }
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  const q = (testid: string) => el.querySelector<HTMLElement>(`[data-testid="${testid}"]`);

  // The options open in the CDK overlay container on <body>, which spec files share.
  afterEach(() => document.querySelector('.cdk-overlay-container')?.replaceChildren());

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ActiveProviderComponent],
    }).compileComponents();
  });

  it('names the provider and marks it in use', async () => {
    await render({});
    expect(q('active-provider-name')?.textContent).toBe('Anthropic API');
    expect(q('active-provider')?.textContent).toContain('In use');
  });

  // Since E3 phase 3 the Pyramidize page has no provider switch: Fix and
  // Pyramidize both send to the provider in use. The lede still promised a
  // per-session switch after that (review of #126, 2026-10-14).
  it('says Fix and Pyramidize both use it, with no per-session switch', async () => {
    await render({});
    const lede = q('active-provider-lede')!.textContent!;
    expect(lede).not.toMatch(/session|another provider|per run|one run|switch/i);
    expect(lede).toContain('Fix');
    expect(lede).toContain('Pyramidize');
  });

  it('names each picker by its feature and column, for a screen reader', async () => {
    await render({ showEffort: true, effortNote: 'More effort means more reasoning.' });
    const model = q('model-fix-claude')!.querySelector('[role="combobox"]')!;
    const names = model.getAttribute('aria-labelledby')!.split(' ').map(id => el.querySelector(`#${id}`)?.textContent?.trim());
    expect(names).toEqual(['Fix', 'Model']);
    expect(q('effort-pyramidize-claude')!.querySelector('[role="combobox"]')!.getAttribute('aria-label')).toBe('Pyramidize effort');
  });

  it('says which model an alias runs today, and nothing for a pinned ID', async () => {
    await render({ fixModel: '', pyramidizeModel: 'claude-sonnet-4-6' });
    expect(q('model-effective-fix')?.textContent?.trim()).toBe('Now claude-haiku-5-5');
    expect(q('model-effective-pyramidize')).toBeNull();
  });

  // "Now …" says which model an alias stands for today. OpenRouter's default
  // is an explicit ID that never moves, so the line would suggest an alias
  // the owner decided against (2026-10-15).
  it('says nothing about "now" for a default that is an explicit ID, as on OpenRouter', async () => {
    const catalogue: ModelOption[] = [
      { id: null, label: '', display: 'Default: Anthropic: Claude Haiku 5.5', secondary: 'anthropic/claude-haiku-5.5' },
      { id: 'anthropic/claude-haiku-5.5', label: 'anthropic/claude-haiku-5.5', display: 'Anthropic: Claude Haiku 5.5', secondary: 'anthropic/claude-haiku-5.5' },
    ];
    await render({ providerId: 'openrouter', fixOptions: catalogue, pyramidizeOptions: catalogue });
    expect(q('model-effective-fix')).toBeNull();
  });

  it('hides effort where the provider has nothing to send it to', async () => {
    await render({ showEffort: false });
    expect(q('effort-fix-claude')).toBeNull();
    await render({ showEffort: true, effortNote: 'More effort means more reasoning.' });
    expect(q('effort-fix-claude')).not.toBeNull();
    expect(q('effort-note')?.textContent).toContain('More effort');
  });

  it('names the Fix model in the no-reasoning note without the "Default:" prefix', async () => {
    await render({ fixFastNote: true });
    expect(q('fix-fast-note')?.textContent).toContain('Fix asks Haiku (latest) to answer');
  });

  /** The labels of the open list's options, in the overlay on <body>. */
  const listed = () => Array.from(document.querySelectorAll('.cdk-overlay-container [role="option"] .kl-option-label'))
    .map(l => l.textContent?.trim());

  function typeInto(testid: string, text: string): void {
    const input = q(testid)!.querySelector('input')!;
    input.value = text;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  }

  // OpenRouter lists ~450 models; finding one means typing part of its name.
  it('searches the model list as you type where the provider asks for that', async () => {
    await render({ searchable: true });
    typeInto('model-fix-claude', 'sonnet 4.6');
    expect(listed()).toEqual(['Claude Sonnet 4.6']);
  });

  it('leaves the other pickers as they were: typing does not narrow their list', async () => {
    await render({});
    typeInto('model-fix-claude', 'sonnet 4.6');
    expect(listed()).toEqual([]);
    q('model-fix-claude')!.querySelector('button')!.click();
    fixture.detectChanges();
    expect(listed().length).toBe(OPTIONS.length);
  });

  it('reports a chosen default as an empty value, never as null', async () => {
    await render({ fixModel: 'sonnet' });
    const changes: FeatureChoice[] = [];
    fixture.componentInstance.modelChange.subscribe(c => changes.push(c));

    // The chevron: the editable picker's only button.
    q('model-fix-claude')!.querySelector('button')!.click();
    fixture.detectChanges();
    document.querySelector<HTMLElement>('.cdk-overlay-container [data-testid="model-option-default"]')!.click();

    expect(changes).toEqual([{ feature: 'fix', value: '' }]);
  });
});

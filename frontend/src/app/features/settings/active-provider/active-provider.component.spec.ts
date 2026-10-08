import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { ActiveProviderComponent, FeatureChoice } from './active-provider.component';
import { ModelOption } from '../model-options';

// PrimeNG's select overlay asks matchMedia whether to go modal; jsdom has none.
// Test-only — see .claude/rules/testing.md.
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false, media: query, onchange: null,
    addListener() {}, removeListener() {}, addEventListener() {}, removeEventListener() {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

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

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ActiveProviderComponent],
      providers: [provideAnimationsAsync()],
    }).compileComponents();
  });

  it('names the provider and marks it in use', async () => {
    await render({});
    expect(q('active-provider-name')?.textContent).toBe('Anthropic API');
    expect(q('active-provider')?.textContent).toContain('In use');
  });

  it('says which model an alias runs today, and nothing for a pinned ID', async () => {
    await render({ fixModel: '', pyramidizeModel: 'claude-sonnet-4-6' });
    expect(q('model-effective-fix')?.textContent?.trim()).toBe('Now claude-haiku-5-5');
    expect(q('model-effective-pyramidize')).toBeNull();
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

  it('reports a chosen default as an empty value, never as null', async () => {
    await render({ fixModel: 'sonnet' });
    const changes: FeatureChoice[] = [];
    fixture.componentInstance.modelChange.subscribe(c => changes.push(c));

    q('model-fix-claude')!.click();
    fixture.detectChanges();
    Array.from(document.querySelectorAll<HTMLElement>('.p-select-option'))
      .find(o => o.textContent?.includes('Default: Haiku (latest)'))!.click();

    expect(changes).toEqual([{ feature: 'fix', value: '' }]);
  });
});

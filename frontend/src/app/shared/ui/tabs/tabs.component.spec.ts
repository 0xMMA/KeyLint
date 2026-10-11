import { describe, it, expect, beforeEach } from 'vitest';
import { Component, signal, ChangeDetectionStrategy } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { TabsComponent, TabComponent, TabPanelComponent } from './tabs.component';

@Component({
  standalone: true,
  imports: [TabsComponent, TabComponent, TabPanelComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <kl-tabs [(value)]="tab" ariaLabel="Settings sections">
      <kl-tab value="general">General</kl-tab>
      <kl-tab value="providers">AI providers</kl-tab>
      <kl-tab value="defaults" disabled>App defaults</kl-tab>
      <kl-tab value="about">About</kl-tab>
      <button klTabsEnd data-testid="end">History</button>
      <kl-tab-panel value="general"><input data-testid="general-field" /></kl-tab-panel>
      <kl-tab-panel value="providers">Providers content</kl-tab-panel>
      <kl-tab-panel value="defaults">Defaults content</kl-tab-panel>
      <kl-tab-panel value="about">About content</kl-tab-panel>
    </kl-tabs>
  `,
})
class Host {
  readonly tab = signal('providers');
}

describe('kl-tabs', () => {
  let fixture: ComponentFixture<Host>;
  let el: HTMLElement;

  const tabs = () => Array.from(el.querySelectorAll<HTMLElement>('[role="tab"]'));
  const tab = (name: string) => tabs().find(t => t.textContent?.trim() === name)!;
  const panels = () => Array.from(el.querySelectorAll<HTMLElement>('[role="tabpanel"]'));
  const key = (target: HTMLElement, k: string) =>
    target.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }));

  async function render(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [Host] }).compileComponents();
    fixture = TestBed.createComponent(Host);
    el = fixture.nativeElement;
    await render();
  });

  it('is a labelled tablist with the chosen tab selected and in the Tab order', () => {
    const list = el.querySelector('[role="tablist"]')!;
    expect(list.getAttribute('aria-label')).toBe('Settings sections');
    expect(tab('AI providers').getAttribute('aria-selected')).toBe('true');
    expect(tab('AI providers').getAttribute('tabindex')).toBe('0');
    expect(tab('General').getAttribute('tabindex')).toBe('-1');
    expect(el.querySelector('[data-testid="end"]')).toBeTruthy();
  });

  it('ties every tab to its panel and shows only the selected one', () => {
    for (const t of tabs()) {
      const panel = el.querySelector(`#${t.getAttribute('aria-controls')}`)!;
      expect(panel.getAttribute('role')).toBe('tabpanel');
      expect(panel.getAttribute('aria-labelledby')).toBe(t.id);
    }
    expect(panels().filter(p => !p.hidden).map(p => p.textContent?.trim())).toEqual(['Providers content']);
  });

  it('selects on click and writes the value back', async () => {
    tab('About').click();
    await render();
    expect(fixture.componentInstance.tab()).toBe('about');
    expect(tab('About').getAttribute('aria-selected')).toBe('true');
    expect(panels().find(p => !p.hidden)?.textContent?.trim()).toBe('About content');
  });

  it('ignores a disabled tab', async () => {
    tab('App defaults').click();
    await render();
    expect(fixture.componentInstance.tab()).toBe('providers');
  });

  it('moves with the arrow keys (skipping disabled, wrapping) and Home/End, focus following', async () => {
    key(tab('AI providers'), 'ArrowRight');
    await render();
    expect(fixture.componentInstance.tab()).toBe('about');
    expect(document.activeElement).toBe(tab('About'));
    key(tab('About'), 'ArrowRight');
    await render();
    expect(fixture.componentInstance.tab()).toBe('general');
    key(tab('General'), 'End');
    await render();
    expect(fixture.componentInstance.tab()).toBe('about');
    key(tab('About'), 'Home');
    key(tab('General'), 'ArrowLeft');
    await render();
    expect(fixture.componentInstance.tab()).toBe('about');
  });

  it('keeps the content of hidden panels (form state survives a tab switch)', async () => {
    tab('General').click();
    await render();
    const field = el.querySelector<HTMLInputElement>('[data-testid="general-field"]')!;
    field.value = 'typed';
    tab('About').click();
    await render();
    tab('General').click();
    await render();
    expect(el.querySelector<HTMLInputElement>('[data-testid="general-field"]')!.value).toBe('typed');
  });

  it('falls back to the first tab when the value names none', async () => {
    fixture.componentInstance.tab.set('nope');
    await render();
    expect(tab('General').getAttribute('aria-selected')).toBe('true');
  });

  it('makes a panel a Tab stop only when nothing inside it can take the focus', async () => {
    const panel = (v: string) => el.querySelector<HTMLElement>(`[role="tabpanel"][data-value="${v}"]`)!;
    expect(panel('general').hasAttribute('tabindex')).toBe(false);
    expect(panel('providers').getAttribute('tabindex')).toBe('0');
  });
});

@Component({
  standalone: true,
  imports: [TabsComponent, TabComponent, TabPanelComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <kl-tabs value="one">
      <kl-tab value="one">One</kl-tab>
      <kl-tab-panel value="one">
        @if (withButton()) { <button data-testid="inner">Do</button> } @else { <p>Just text</p> }
      </kl-tab-panel>
    </kl-tabs>
  `,
})
class ChangingHost {
  readonly withButton = signal(false);
}

describe('kl-tab-panel Tab stop', () => {
  it('follows its content as it changes', async () => {
    await TestBed.configureTestingModule({ imports: [ChangingHost] }).compileComponents();
    const fixture = TestBed.createComponent(ChangingHost);
    const settle = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
    await settle();
    const panel = fixture.nativeElement.querySelector('[role="tabpanel"]') as HTMLElement;
    expect(panel.getAttribute('tabindex')).toBe('0');
    fixture.componentInstance.withButton.set(true);
    await settle();
    expect(panel.hasAttribute('tabindex')).toBe(false);
    fixture.componentInstance.withButton.set(false);
    await settle();
    expect(panel.getAttribute('tabindex')).toBe('0');
  });
});

import {
  ChangeDetectionStrategy, Component, ElementRef, booleanAttribute, computed, contentChildren, inject, input, model,
} from '@angular/core';

let nextId = 0;

/**
 * Accessible tabs (WAI-ARIA tabs pattern, automatic activation):
 *
 *   <kl-tabs [(value)]="tab" ariaLabel="Settings sections">
 *     <kl-tab value="general">General</kl-tab>
 *     <kl-tab value="providers">AI providers</kl-tab>
 *     <button klTabsEnd …>History</button>          (optional, right of the tabs)
 *     <kl-tab-panel value="general">…</kl-tab-panel>
 *     <kl-tab-panel value="providers">…</kl-tab-panel>
 *   </kl-tabs>
 *
 * Only the selected tab is in the Tab order (roving tabindex); Left/Right
 * move between tabs and select, Home/End jump to the ends. Panels stay
 * rendered and are hidden when inactive, so a form in a hidden panel keeps
 * its state; `role="tabpanel"` with aria-labelledby ties each to its tab.
 */
@Component({
  selector: 'kl-tabs',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { class: 'kl-tabs' },
  template: `
    <div class="kl-tabs-bar">
      <div role="tablist" class="kl-tablist" [attr.aria-label]="ariaLabel() || null" (keydown)="onKeydown($event)">
        <ng-content select="kl-tab" />
      </div>
      <ng-content select="[klTabsEnd]" />
    </div>
    <ng-content />
  `,
  styles: [`
    :host { display: block; }
    .kl-tabs-bar {
      display: flex; align-items: center; justify-content: space-between; gap: 12px;
      border-bottom: 1px solid var(--kl-line);
    }
    .kl-tablist { display: flex; flex-wrap: wrap; gap: 24px; }
    :host(.kl-tabs-compact) .kl-tablist { gap: 20px; }
  `],
})
export class TabsComponent {
  readonly value = model<string>('');
  readonly ariaLabel = input('');

  readonly baseId = `kl-tabs-${nextId++}`;
  private readonly tabs = contentChildren(TabComponent);

  /** The value shown: the chosen one, or the first tab if none matches. */
  readonly active = computed(() => {
    const all = this.tabs();
    const v = this.value();
    return all.some(t => t.value() === v) ? v : (all[0]?.value() ?? '');
  });

  tabId(value: string): string { return `${this.baseId}-tab-${value}`; }
  panelId(value: string): string { return `${this.baseId}-panel-${value}`; }

  select(value: string): void { this.value.set(value); }

  protected onKeydown(event: KeyboardEvent): void {
    const enabled = this.tabs().filter(t => !t.disabled());
    if (!enabled.length) return;
    const current = enabled.findIndex(t => t.value() === this.active());
    let next: number;
    switch (event.key) {
      case 'ArrowRight': next = (current + 1) % enabled.length; break;
      case 'ArrowLeft': next = (current - 1 + enabled.length) % enabled.length; break;
      case 'Home': next = 0; break;
      case 'End': next = enabled.length - 1; break;
      default: return;
    }
    event.preventDefault();
    const tab = enabled[next];
    this.select(tab.value());
    tab.focus();
  }
}

/** One tab; see TabsComponent. The host is the role="tab" element. */
@Component({
  selector: 'kl-tab',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    role: 'tab',
    class: 'kl-tab',
    '[id]': 'tabs.tabId(value())',
    '[attr.aria-selected]': 'selected()',
    '[attr.aria-controls]': 'tabs.panelId(value())',
    '[attr.aria-disabled]': 'disabled() ? "true" : null',
    '[attr.tabindex]': 'selected() ? 0 : -1',
    '[attr.data-value]': 'value()',
    '(click)': 'onClick()',
    '(keydown.enter)': 'onClick()',
    '(keydown.space)': '$event.preventDefault(); onClick()',
  },
  template: `<ng-content />`,
  styles: [`
    :host {
      display: inline-flex; align-items: center; gap: 6px;
      height: 46px; padding: 0 2px; margin-bottom: -1px;
      font-size: 14px; font-weight: 600; color: var(--kl-muted);
      border-bottom: 2px solid transparent; cursor: pointer; user-select: none;
      transition: color 0.12s, border-color 0.12s;
    }
    :host(:hover) { color: var(--kl-text); }
    :host([aria-selected='true']) { color: var(--kl-text-strong); border-bottom-color: var(--kl-primary); }
    :host([aria-disabled='true']) { opacity: 0.5; cursor: not-allowed; }
    :host(:focus-visible) { outline: 2px solid var(--kl-focus); outline-offset: 2px; border-radius: 2px; }
  `],
})
export class TabComponent {
  readonly value = input.required<string>();
  readonly disabled = input(false, { transform: booleanAttribute });

  protected readonly tabs = inject(TabsComponent);
  private readonly el = inject<ElementRef<HTMLElement>>(ElementRef);
  readonly selected = computed(() => this.tabs.active() === this.value());

  protected onClick(): void {
    if (!this.disabled()) this.tabs.select(this.value());
  }

  focus(): void { this.el.nativeElement.focus(); }
}

/** One tab's content; hidden while its tab is not selected. */
@Component({
  selector: 'kl-tab-panel',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    role: 'tabpanel',
    class: 'kl-tab-panel',
    '[id]': 'tabs.panelId(value())',
    '[attr.aria-labelledby]': 'tabs.tabId(value())',
    '[attr.data-value]': 'value()',
    '[attr.data-active]': 'active() ? "true" : null',
    '[hidden]': '!active()',
    tabindex: '0',
  },
  template: `<ng-content />`,
  styles: [`
    :host { display: block; }
    :host([hidden]) { display: none; }
    :host(:focus-visible) { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
  `],
})
export class TabPanelComponent {
  readonly value = input.required<string>();
  protected readonly tabs = inject(TabsComponent);
  readonly active = computed(() => this.tabs.active() === this.value());
}

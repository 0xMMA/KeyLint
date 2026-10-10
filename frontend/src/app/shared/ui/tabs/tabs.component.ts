import {
  ChangeDetectionStrategy, Component, DestroyRef, ElementRef, afterNextRender, afterRenderEffect, booleanAttribute,
  computed, contentChildren, inject, input, model, signal, untracked, viewChild,
} from '@angular/core';

let nextId = 0;

/** What can take the focus by Tab, roughly: enough to tell an empty panel from a form. */
const FOCUSABLE = [
  'a[href]', 'area[href]', 'button:not([disabled])', 'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])', 'textarea:not([disabled])', 'iframe', 'summary',
  '[contenteditable]:not([contenteditable="false"])', '[tabindex]:not([tabindex="-1"])',
].join(',');

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
 * A panel is a Tab stop of its own only when nothing inside it can take the
 * focus (WAI-ARIA APG); otherwise Tab goes from the tab straight to its first
 * control.
 *
 * On a narrow window the tab row scrolls sideways instead of wrapping onto a
 * second line: the selected tab is kept in view, and the edge with more tabs
 * behind it fades out so the row reads as scrollable. The klTabsEnd content
 * gives way first: when tabs and end content do not fit side by side, the end
 * content moves onto a line above the tabs (wrap-reverse keeps the tabs on the
 * divider), so it never squeezes the tab row.
 *
 * The row is measured when it scrolls or changes size (a ResizeObserver on
 * the row and its tabs), not on every render of the app.
 */
@Component({
  selector: 'kl-tabs',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { class: 'kl-tabs' },
  template: `
    <div class="kl-tabs-bar">
      <div #row role="tablist" class="kl-tablist" [attr.aria-label]="ariaLabel() || null"
        [class.more-start]="moreStart()" [class.more-end]="moreEnd()"
        (keydown)="onKeydown($event)" (scroll)="measureRow()">
        <ng-content select="kl-tab" />
      </div>
      <div class="kl-tabs-end"><ng-content select="[klTabsEnd]" /></div>
    </div>
    <ng-content />
  `,
  styles: [`
    :host { display: block; }
    /* The divider is an inset shadow, not a border, so the tabs' 2px marker can
       sit inside the scrolling tab row and still cover it. */
    /* wrap-reverse: end content that does not fit beside the tabs goes onto a
       line above them, and the tabs stay on the divider. The row keeps its
       natural width (flex-basis auto) unless the bar is narrower than it. */
    .kl-tabs-bar {
      display: flex; flex-wrap: wrap-reverse; align-items: center; column-gap: 12px;
      box-shadow: inset 0 -1px 0 var(--kl-line);
    }
    .kl-tablist {
      display: flex; flex-wrap: nowrap; gap: 24px; flex: 0 1 auto; min-width: 0; max-width: 100%;
      overflow-x: auto; overflow-y: hidden; scrollbar-width: none;
    }
    .kl-tabs-end { display: flex; justify-content: flex-end; min-width: 0; max-width: 100%; margin-left: auto; }
    .kl-tabs-end:empty { display: none; }
    .kl-tablist::-webkit-scrollbar { display: none; }
    .kl-tablist.more-end { mask-image: linear-gradient(to right, #000 calc(100% - 32px), transparent); }
    .kl-tablist.more-start { mask-image: linear-gradient(to right, transparent, #000 32px); }
    .kl-tablist.more-start.more-end {
      mask-image: linear-gradient(to right, transparent, #000 32px, #000 calc(100% - 32px), transparent);
    }
    :host(.kl-tabs-compact) .kl-tablist { gap: 20px; }
  `],
})
export class TabsComponent {
  readonly value = model<string>('');
  readonly ariaLabel = input('');

  readonly baseId = `kl-tabs-${nextId++}`;
  private readonly tabs = contentChildren(TabComponent);
  private readonly row = viewChild.required<ElementRef<HTMLElement>>('row');
  /** Tabs hidden behind the row's left / right edge. */
  protected readonly moreStart = signal(false);
  protected readonly moreEnd = signal(false);

  private resizeObserver: ResizeObserver | null = null;

  constructor() {
    afterNextRender(() => {
      this.measureRow();
      if (typeof ResizeObserver === 'undefined') return;
      // The row's width (window, end content) and its content's width (a tab
      // label that changes) both move the fades; nothing else does.
      this.resizeObserver = new ResizeObserver(() => this.measureRow());
      this.resizeObserver.observe(this.row().nativeElement);
      for (const tab of this.row().nativeElement.children) this.resizeObserver.observe(tab);
    });
    inject(DestroyRef).onDestroy(() => this.resizeObserver?.disconnect());
  }

  /**
   * Brings a tab into the row's view, scrolling only the row, never the page.
   * A tab wider than the row shows its start: aligning both edges in turn
   * would scroll back and forth.
   */
  reveal(tab: HTMLElement): void {
    const row = this.row().nativeElement;
    const r = row.getBoundingClientRect();
    const t = tab.getBoundingClientRect();
    if (t.left < r.left || t.width > r.width) row.scrollLeft += t.left - r.left;
    else if (t.right > r.right) row.scrollLeft += t.right - r.right;
    this.measureRow();
  }

  protected measureRow(): void {
    const row = this.row().nativeElement;
    const start = row.scrollLeft > 1;
    const end = row.scrollLeft + row.clientWidth < row.scrollWidth - 1;
    if (start !== this.moreStart()) this.moreStart.set(start);
    if (end !== this.moreEnd()) this.moreEnd.set(end);
  }

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
      display: inline-flex; align-items: center; gap: 6px; flex-shrink: 0; white-space: nowrap;
      height: 46px; padding: 0 2px;
      font-size: 14px; font-weight: 600; color: var(--kl-muted);
      border-bottom: 2px solid transparent; cursor: pointer; user-select: none;
      transition: color 0.12s, border-color 0.12s;
    }
    :host(:hover) { color: var(--kl-text); }
    :host([aria-selected='true']) { color: var(--kl-text-strong); border-bottom-color: var(--kl-primary); }
    :host([aria-disabled='true']) { opacity: 0.5; cursor: not-allowed; }
    /* Inside the box: the tab row scrolls, so an outer ring would be clipped. */
    :host(:focus-visible) { outline: 2px solid var(--kl-focus); outline-offset: -2px; border-radius: 4px; }
  `],
})
export class TabComponent {
  readonly value = input.required<string>();
  readonly disabled = input(false, { transform: booleanAttribute });

  protected readonly tabs = inject(TabsComponent);
  private readonly el = inject<ElementRef<HTMLElement>>(ElementRef);
  readonly selected = computed(() => this.tabs.active() === this.value());

  constructor() {
    // Also on the first render: a page opened on its last tab (?tab=about)
    // must not start with that tab scrolled out of a narrow row.
    // Only `selected` is tracked: reveal reads the row's fade signals, and an
    // effect that tracked them would re-run on every fade it causes (NG0103).
    afterRenderEffect(() => {
      if (this.selected()) untracked(() => this.tabs.reveal(this.el.nativeElement));
    });
  }

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
    '[attr.tabindex]': 'hasFocusable() ? null : 0',
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
  private readonly el = inject<ElementRef<HTMLElement>>(ElementRef);
  readonly active = computed(() => this.tabs.active() === this.value());
  /** Something inside can take the focus, so the panel itself needs no Tab stop. */
  protected readonly hasFocusable = signal(false);
  private readonly destroyRef = inject(DestroyRef);

  constructor() {
    // Content is projected and changes with state (a message appears, a form
    // is replaced), so look again when the panel's DOM changes rather than on
    // every render of the app; the signal only moves when the answer does.
    const check = () => {
      const found = this.el.nativeElement.querySelector(FOCUSABLE) !== null;
      if (found !== this.hasFocusable()) this.hasFocusable.set(found);
    };
    afterNextRender(() => {
      check();
      const observer = new MutationObserver(check);
      observer.observe(this.el.nativeElement, {
        childList: true, subtree: true,
        attributes: true, attributeFilter: ['disabled', 'tabindex', 'href', 'contenteditable', 'type'],
      });
      this.destroyRef.onDestroy(() => observer.disconnect());
    });
  }
}

import {
  ChangeDetectionStrategy, Component, ComponentRef, Directive, ElementRef, Injector, OnDestroy, effect, inject, input,
  signal,
} from '@angular/core';
import { ComponentPortal } from '@angular/cdk/portal';
import {
  type ConnectedPosition, type FlexibleConnectedPositionStrategy, type OverlayRef,
  createFlexibleConnectedPositionStrategy, createOverlayRef, createRepositionScrollStrategy,
} from '@angular/cdk/overlay';
import { ScrollDispatcher } from '@angular/cdk/scrolling';
import { Subscription } from 'rxjs';

export type TooltipPosition = 'right' | 'left' | 'top' | 'bottom';

let nextId = 0;

const FOCUSABLE = 'a[href], button, input, select, textarea, [tabindex]:not([tabindex="-1"])';

/** The bubble itself, rendered into the CDK overlay container on <body>. */
@Component({
  selector: 'kl-tooltip-panel',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { role: 'tooltip', '[id]': 'id()', class: 'kl-tooltip', 'data-testid': 'kl-tooltip' },
  template: `{{ text() }}`,
  styles: [`
    :host {
      display: block;
      max-width: 280px;
      padding: 6px 10px;
      font-family: var(--kl-font-ui);
      font-size: 13px;
      line-height: 1.35;
      color: var(--kl-text);
      background: var(--kl-raised);
      border: 1px solid var(--kl-line-control);
      border-radius: var(--kl-radius-lg);
      box-shadow: var(--kl-shadow-overlay);
      pointer-events: none;
      animation: kl-tooltip-in 0.12s ease-out;
    }
    @keyframes kl-tooltip-in { from { opacity: 0; } }
  `],
})
export class TooltipPanelComponent {
  readonly text = signal('');
  readonly id = signal('');
}

const POSITIONS: Record<TooltipPosition, ConnectedPosition[]> = {
  right: [
    { originX: 'end', originY: 'center', overlayX: 'start', overlayY: 'center', offsetX: 8 },
    { originX: 'start', originY: 'center', overlayX: 'end', overlayY: 'center', offsetX: -8 },
  ],
  left: [
    { originX: 'start', originY: 'center', overlayX: 'end', overlayY: 'center', offsetX: -8 },
    { originX: 'end', originY: 'center', overlayX: 'start', overlayY: 'center', offsetX: 8 },
  ],
  top: [
    { originX: 'center', originY: 'top', overlayX: 'center', overlayY: 'bottom', offsetY: -8 },
    { originX: 'center', originY: 'bottom', overlayX: 'center', overlayY: 'top', offsetY: 8 },
  ],
  bottom: [
    { originX: 'center', originY: 'bottom', overlayX: 'center', overlayY: 'top', offsetY: 8 },
    { originX: 'center', originY: 'top', overlayX: 'center', overlayY: 'bottom', offsetY: -8 },
  ],
};

/**
 * A text tooltip on the CDK overlay (appended to <body>, so no parent's
 * overflow clips it).
 *
 *   <a routerLink="/fix" [klTooltip]="collapsed ? 'Fix' : ''" klTooltipPosition="right">…</a>
 *
 * Shows on hover (after a short delay) and on keyboard focus, hides on leave,
 * blur and Escape (pressed anywhere, WCAG 1.4.13). While shown, the element
 * that takes the focus is described by it (aria-describedby): the host, or
 * for a wrapper like kl-button the <button> inside it. An empty text means no
 * tooltip, which is how a caller switches it off.
 */
@Directive({
  selector: '[klTooltip]',
  standalone: true,
  host: {
    '(mouseenter)': 'scheduleShow()',
    '(mouseleave)': 'hide()',
    '(focusin)': 'onFocus()',
    '(focusout)': 'onBlur()',
    '(pointerdown)': 'pointerDown = true',
    // After the focus a press causes, so the flag never outlives the press.
    '(pointerup)': 'pointerDown = false',
    '(pointercancel)': 'pointerDown = false',
    '(document:keydown.escape)': 'onEscape()',
    '(click)': 'hide()',
  },
})
export class TooltipDirective implements OnDestroy {
  readonly klTooltip = input('');
  readonly klTooltipPosition = input<TooltipPosition>('right');
  /** Hover delay in ms; focus shows at once. */
  readonly klTooltipDelay = input(300);

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);
  private readonly scrollDispatcher = inject(ScrollDispatcher);
  private overlayRef: OverlayRef | null = null;
  private positionStrategy: FlexibleConnectedPositionStrategy | null = null;
  private positionSub: Subscription | null = null;
  private panel: ComponentRef<TooltipPanelComponent> | null = null;
  private timer: ReturnType<typeof setTimeout> | null = null;
  protected pointerDown = false;
  private readonly id = `kl-tooltip-${nextId++}`;
  /** The element carrying aria-describedby while shown. */
  private described: HTMLElement | null = null;

  constructor() {
    // A text that changes while shown (a collapse button that now expands)
    // updates in place; one that becomes empty takes the tooltip away.
    effect(() => {
      const text = this.klTooltip();
      if (!this.panel) return;
      if (text) this.panel.instance.text.set(text);
      else this.hide();
    });
  }

  scheduleShow(): void {
    this.clearTimer();
    if (!this.klTooltip()) return;
    this.timer = setTimeout(() => this.show(), this.klTooltipDelay());
  }

  onFocus(): void {
    // Not for a focus that a click caused: the tooltip would stay up after
    // the pointer has gone. Keyboard (and programmatic) focus shows it.
    if (!this.pointerDown) this.show();
  }

  onBlur(): void {
    this.pointerDown = false;
    this.hide();
  }

  show(): void {
    this.clearTimer();
    const text = this.klTooltip();
    if (!text) return;
    if (!this.overlayRef) {
      this.positionStrategy = createFlexibleConnectedPositionStrategy(this.injector, this.host)
        .withPositions(POSITIONS[this.klTooltipPosition()])
        .withPush(true);
      // Follows the host when a cdkScrollable (or the document) scrolls, and
      // goes once the host has scrolled out of view.
      this.overlayRef = createOverlayRef(this.injector, {
        positionStrategy: this.positionStrategy,
        scrollStrategy: createRepositionScrollStrategy(this.injector),
        usePopover: false,
      });
      this.positionSub = this.positionStrategy.positionChanges.subscribe(change => {
        if (change.scrollableViewProperties.isOriginOutsideView) queueMicrotask(() => this.hide());
      });
    }
    this.positionStrategy?.withScrollableContainers(this.scrollDispatcher.getAncestorScrollContainers(this.host));
    if (!this.overlayRef.hasAttached()) {
      this.panel = this.overlayRef.attach(new ComponentPortal(TooltipPanelComponent));
      this.panel.instance.id.set(this.id);
    }
    this.panel?.instance.text.set(text);
    this.describe(true);
  }

  hide(): void {
    this.clearTimer();
    if (this.overlayRef?.hasAttached()) this.overlayRef.detach();
    this.panel = null;
    this.describe(false);
  }

  protected onEscape(): void {
    if (this.panel || this.timer !== null) this.hide();
  }

  /**
   * Adds or removes this tooltip's id in aria-describedby of the element that
   * takes the focus, keeping any id that is there for another reason.
   */
  private describe(on: boolean): void {
    if (on && !this.described) this.described = this.focusTarget();
    const el = this.described;
    if (!el) return;
    const ids = (el.getAttribute('aria-describedby') ?? '').split(/\s+/).filter(id => id && id !== this.id);
    if (on) ids.push(this.id);
    if (ids.length) el.setAttribute('aria-describedby', ids.join(' '));
    else el.removeAttribute('aria-describedby');
    if (!on) this.described = null;
  }

  /** The host if it takes the focus itself, else the first element inside it that does. */
  private focusTarget(): HTMLElement {
    const host = this.host.nativeElement;
    if (host.matches(FOCUSABLE)) return host;
    return host.querySelector<HTMLElement>(FOCUSABLE) ?? host;
  }

  ngOnDestroy(): void {
    this.clearTimer();
    this.describe(false);
    this.positionSub?.unsubscribe();
    this.overlayRef?.dispose();
    this.overlayRef = null;
    this.panel = null;
  }

  private clearTimer(): void {
    if (this.timer !== null) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }
}

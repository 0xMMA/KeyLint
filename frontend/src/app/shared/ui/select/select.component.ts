import {
  ChangeDetectionStrategy, Component, ElementRef, Injector, OnDestroy, TemplateRef, ViewContainerRef,
  booleanAttribute, computed, forwardRef, inject, input, output, signal, viewChild,
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { TemplatePortal } from '@angular/cdk/portal';
import {
  type FlexibleConnectedPositionStrategy, type OverlayRef, createFlexibleConnectedPositionStrategy, createOverlayRef,
  createRepositionScrollStrategy,
} from '@angular/cdk/overlay';
import { ScrollDispatcher, ViewportRuler } from '@angular/cdk/scrolling';
import { Subscription } from 'rxjs';
import { IconComponent } from '../icon/icon.component';

let nextId = 0;

type Option = unknown;

/**
 * A dropdown select on the CDK overlay, with an editable (combobox) mode.
 *
 *   <kl-select data-testid="log-level" ariaLabel="Log level"
 *              [options]="levels" optionLabel="label" optionValue="value"
 *              [(ngModel)]="settings.log_level" />
 *
 * Fixed list: a <button role="combobox" aria-haspopup="listbox"> that shows
 * the chosen option (WAI-ARIA select-only combobox). Editable: an
 * <input role="combobox" aria-autocomplete="list"> plus a chevron button —
 * free text is allowed, which is what PrimeNG's `[editable]` p-select did.
 * Focus stays on the trigger in both; the options are a role="listbox" in the
 * CDK overlay container on <body> (so no parent's overflow clips it), and the
 * active one is announced with aria-activedescendant.
 *
 * The open list follows its trigger when the page scrolls and closes once
 * the trigger has scrolled out of its container's view. That needs the
 * container to be known to the CDK: the app's scroll containers carry
 * `cdkScrollable` (the shell's main area, the Pyramidize page's scroll area);
 * a new scrolling container that can hold a select needs it too.
 *
 * Options are any objects. `optionLabel` names the field that is the option's
 * text (and, editable, what is written into the field), `optionValue` the
 * value that goes to the model (the option itself if unset). `optionDisplay`
 * and `optionDescription` let the list show a readable name and a muted
 * second line instead (docs/ui/design.md: two lines, never run together);
 * the fixed trigger shows `optionDisplay` too.
 *
 * Editable semantics, the same as p-select's, which
 * features/settings/model-options.ts relies on: typed text is the value,
 * keystroke by keystroke; a cleared field is "" (not null); the field shows
 * the matching option's label, or the raw value if none matches; an option
 * whose label is "" leaves the field empty so the placeholder shows.
 *
 * `filter` (editable only) makes the field a search instead: typing opens the
 * list and keeps the options whose label, display text or second line hold
 * every typed word, case-insensitively — and changes nothing. A click or Enter
 * on an option commits it; Enter on text that names no option commits that
 * text, so a free-text value still works; Escape, leaving the field or a
 * click elsewhere put the stored value back. (Committing every keystroke made
 * an abandoned search the saved value.) Opening with the chevron or ArrowDown
 * shows the whole list. Without `filter` nothing here changes: typed text is
 * the value and the list never narrows, as in p-select's editable mode.
 *
 * Keyboard: ArrowDown/ArrowUp open and move, Enter takes the active option
 * (editable: typed text that names no option leaves none active, and Enter
 * then keeps the typed text and closes), Escape closes, Tab closes; the fixed list also opens on Space/Enter and has
 * Home/End and type-ahead. `changed` (and the model) fire only on user action,
 * and only when the value actually changes: picking the option that is
 * already selected just closes the list, as p-select did.
 *
 * An outside `<label for>` names the trigger via `inputId`. Clicking it
 * focuses the trigger without opening the list (a <button> would otherwise
 * take the label's click as its own), and the list borrows the label's text
 * as its name when no ariaLabel/ariaLabelledBy is given.
 */
@Component({
  selector: 'kl-select',
  standalone: true,
  imports: [IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [{ provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => SelectComponent), multi: true }],
  host: { class: 'kl-select', '[class.kl-select-open]': 'isOpen()', '(document:click)': 'onDocumentClick($event)' },
  template: `
    @if (editable()) {
      <div class="kl-select-field kl-select-editable" [class.disabled]="isDisabled()">
        <input
          #trigger
          type="text"
          role="combobox"
          class="kl-select-input"
          autocomplete="off"
          spellcheck="false"
          aria-autocomplete="list"
          [id]="inputId() || null"
          [value]="inputText()"
          [placeholder]="placeholder()"
          [disabled]="isDisabled()"
          [attr.aria-label]="ariaLabel() || null"
          [attr.aria-labelledby]="ariaLabelledBy() || null"
          [attr.aria-describedby]="ariaDescribedBy() || null"
          [attr.aria-expanded]="isOpen()"
          [attr.aria-controls]="listboxId"
          [attr.aria-activedescendant]="activeId()"
          (input)="onType($event)"
          (keydown)="onKeydown($event)"
          (blur)="onBlur()"
        />
        <button type="button" class="kl-select-chevron" tabindex="-1" aria-label="Show options"
          [disabled]="isDisabled()" (mousedown)="$event.preventDefault()" (click)="toggle()">
          <kl-icon [name]="isOpen() ? 'chevron-up' : 'chevron-down'" />
        </button>
      </div>
    } @else {
      <button
        #trigger
        type="button"
        role="combobox"
        class="kl-select-field"
        aria-haspopup="listbox"
        [id]="inputId() || null"
        [disabled]="isDisabled()"
        [attr.aria-label]="ariaLabel() || null"
        [attr.aria-labelledby]="ariaLabelledBy() || null"
        [attr.aria-describedby]="ariaDescribedBy() || null"
        [attr.aria-expanded]="isOpen()"
        [attr.aria-controls]="listboxId"
        [attr.aria-activedescendant]="activeId()"
        (click)="toggle()"
        (keydown)="onKeydown($event)"
        (blur)="onTouched()"
      >
        @if (selectedIndex() >= 0) {
          <span class="kl-select-value">{{ displayOf(selectedOption()) }}</span>
        } @else {
          <span class="kl-select-value kl-select-placeholder">{{ placeholder() }}</span>
        }
        <kl-icon class="kl-select-chevron-icon" [name]="isOpen() ? 'chevron-up' : 'chevron-down'" />
      </button>
    }

    <ng-template #panel>
      <div role="listbox" class="kl-select-panel" [id]="listboxId" [attr.aria-label]="ariaLabel() || forLabel() || null"
        [attr.aria-labelledby]="ariaLabel() ? null : ariaLabelledBy() || null" data-testid="kl-select-panel">
        @for (option of shown(); track $index; let i = $index) {
          <div
            role="option"
            class="kl-option"
            [id]="optionId(i)"
            [class.active]="i === activeIndex()"
            [attr.aria-selected]="i === shownSelectedIndex()"
            [attr.aria-disabled]="disabledOf(option) ? 'true' : null"
            [attr.data-testid]="optionTestId()?.(option) ?? null"
            (mousedown)="$event.preventDefault()"
            (mouseenter)="hover(i)"
            (click)="choose(i)"
          >
            <span class="kl-option-label">
              {{ displayOf(option) }}
              @if (i === shownSelectedIndex()) {
                <kl-icon class="kl-option-check" name="check" [size]="14" [strokeWidth]="2.5" />
              }
            </span>
            @if (descriptionOf(option); as description) {
              <span class="kl-option-description">{{ description }}</span>
            }
          </div>
        } @empty {
          <div class="kl-option-empty">{{ searching() ? 'No matches' : 'No options' }}</div>
        }
      </div>
    </ng-template>
  `,
  styles: [`
    :host { display: block; min-width: 0; }
    .kl-select-field {
      font: inherit; font-size: 14px; color: var(--kl-text);
      display: flex; align-items: center; justify-content: space-between; gap: 8px;
      width: 100%; height: 40px; padding: 0 12px; text-align: left;
      background: var(--kl-field); border: 1px solid var(--kl-line-control); border-radius: var(--kl-radius-md);
      cursor: pointer; transition: border-color 0.12s;
    }
    .kl-select-field:hover:not(:disabled):not(.disabled) { border-color: var(--kl-line-strong); }
    :host(.kl-select-open) .kl-select-field { border-color: var(--kl-primary); }
    .kl-select-field:focus-visible,
    .kl-select-editable:has(.kl-select-input:focus-visible) { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
    .kl-select-field:disabled, .kl-select-field.disabled { background: var(--kl-rail); color: var(--kl-muted); cursor: not-allowed; }
    .kl-select-value { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
    .kl-select-placeholder { color: var(--kl-muted); }
    .kl-select-chevron-icon { color: var(--kl-muted); }
    :host(.kl-select-open) .kl-select-chevron-icon, :host(.kl-select-open) .kl-select-chevron { color: var(--kl-primary); }

    .kl-select-editable { padding: 0 4px 0 0; cursor: text; }
    .kl-select-input {
      flex: 1; min-width: 0; height: 100%; padding: 0 0 0 12px;
      font: inherit; color: inherit; background: transparent; border: 0; outline: none;
    }
    .kl-select-input::placeholder { color: var(--kl-muted); opacity: 1; }
    .kl-select-chevron {
      display: inline-flex; align-items: center; justify-content: center; width: 32px; height: 32px; padding: 0;
      color: var(--kl-muted); background: transparent; border: 0; border-radius: var(--kl-radius-sm); cursor: pointer;
    }
    .kl-select-chevron:disabled { cursor: not-allowed; }

    .kl-select-panel {
      width: 100%; max-height: 18rem; overflow-y: auto; padding: 4px;
      font-family: var(--kl-font-ui); color: var(--kl-text);
      background: var(--kl-raised); border: 1px solid var(--kl-line-control); border-radius: var(--kl-radius-lg);
      box-shadow: var(--kl-shadow-overlay);
    }
    .kl-option { display: flex; flex-direction: column; gap: 2px; padding: 8px 12px; border-radius: 5px; cursor: pointer; }
    .kl-option[aria-selected='true'] { background: var(--kl-selected); }
    .kl-option.active { background: var(--kl-raised-hover); box-shadow: inset 0 0 0 1px var(--kl-line-strong); }
    .kl-option[aria-disabled='true'] { opacity: 0.5; cursor: not-allowed; }
    .kl-option-label { display: flex; justify-content: space-between; align-items: center; gap: 8px; font-size: 14px; }
    .kl-option[aria-selected='true'] .kl-option-label { color: var(--kl-text-strong); }
    .kl-option-check { color: var(--kl-primary); }
    .kl-option-description { font-size: 12px; color: var(--kl-muted); overflow-wrap: anywhere; }
    .kl-option-empty { padding: 8px 12px; font-size: 13px; color: var(--kl-muted); }
  `],
})
export class SelectComponent implements ControlValueAccessor, OnDestroy {
  readonly options = input<readonly Option[]>([]);
  readonly optionLabel = input('');
  readonly optionValue = input('');
  readonly optionDisplay = input('');
  readonly optionDescription = input('');
  readonly optionDisabled = input('disabled');
  readonly placeholder = input('');
  readonly editable = input(false, { transform: booleanAttribute });
  /** Editable only: typing also narrows the list to the options that match; see the class comment. */
  readonly filter = input(false, { transform: booleanAttribute });
  readonly disabled = input(false, { transform: booleanAttribute });
  readonly ariaLabel = input('');
  readonly ariaLabelledBy = input('');
  /** id of an element that describes the control, set on the trigger. */
  readonly ariaDescribedBy = input('');
  /** id of the trigger, for an outside <label for>. */
  readonly inputId = input('');
  /** data-testid per option row, e.g. o => 'model-option-' + (o.id || 'default'). */
  readonly optionTestId = input<((option: Option) => string | null) | null>(null);

  /** The user picked a different option or typed (never on a value written from outside). */
  readonly changed = output<unknown>();

  private readonly injector = inject(Injector);
  private readonly vcr = inject(ViewContainerRef);
  private readonly hostEl = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly scrollDispatcher = inject(ScrollDispatcher);
  private readonly viewportRuler = inject(ViewportRuler);
  private readonly triggerRef = viewChild<ElementRef<HTMLElement>>('trigger');
  private readonly panelTpl = viewChild.required<TemplateRef<unknown>>('panel');

  private readonly baseId = `kl-select-${nextId++}`;
  readonly listboxId = `${this.baseId}-listbox`;

  protected readonly value = signal<unknown>(null);
  protected readonly isOpen = signal(false);
  protected readonly activeIndex = signal(-1);
  /** The text of the trigger's <label for>, read on open; names the listbox when nothing else does. */
  protected readonly forLabel = signal('');
  private readonly formDisabled = signal(false);
  protected readonly isDisabled = computed(() => this.disabled() || this.formDisabled());

  protected readonly selectedIndex = computed(() => {
    const v = this.value();
    return this.options().findIndex(o => this.valueOf(o) === v);
  });
  protected readonly selectedOption = computed(() => this.options()[this.selectedIndex()]);
  /** What was typed since the list opened; with `filter`, what the list is narrowed by. */
  private readonly query = signal('');
  /** With `filter`: the search in the field, not yet a value; null when the field shows the value. */
  private readonly draft = signal<string | null>(null);
  /** Whether typing searches rather than sets the value. */
  private readonly searches = computed(() => this.filter() && this.editable());
  protected readonly searching = computed(() => this.searches() && this.query().trim() !== '');
  /**
   * The options the open list shows. Every index into the list — the active
   * option, a click, the arrows — is an index into this, not into options().
   */
  protected readonly shown = computed(() => {
    const list = this.options();
    if (!this.searching()) return list;
    const words = this.query().toLowerCase().split(/\s+/).filter(Boolean);
    return list.filter(option => {
      const text = `${this.labelOf(option)} ${this.displayOf(option)} ${this.descriptionOf(option)}`.toLowerCase();
      return words.every(word => text.includes(word));
    });
  });
  protected readonly shownSelectedIndex = computed(() => {
    const v = this.value();
    return this.shown().findIndex(o => this.valueOf(o) === v);
  });
  protected readonly inputText = computed(() => {
    const draft = this.draft();
    if (draft !== null) return draft;
    const i = this.selectedIndex();
    if (i >= 0) return this.labelOf(this.options()[i]);
    const v = this.value();
    return v === null || v === undefined ? '' : String(v);
  });
  protected readonly activeId = computed(() =>
    this.isOpen() && this.activeIndex() >= 0 ? this.optionId(this.activeIndex()) : null);

  private overlayRef: OverlayRef | null = null;
  private positionStrategy: FlexibleConnectedPositionStrategy | null = null;
  private subs = new Subscription();
  private typeahead = '';
  private typeaheadTimer: ReturnType<typeof setTimeout> | null = null;

  private onChange: (value: unknown) => void = () => {};
  protected onTouched: () => void = () => {};

  // ── option fields ───────────────────────────────────────────────────────

  private field(option: Option, key: string): unknown {
    return key && option !== null && typeof option === 'object' ? (option as Record<string, unknown>)[key] : option;
  }
  labelOf(option: Option): string {
    const label = this.field(option, this.optionLabel());
    return label === null || label === undefined ? '' : String(label);
  }
  valueOf(option: Option): unknown {
    const v = this.optionValue() ? this.field(option, this.optionValue()) : option;
    return v === undefined ? null : v;
  }
  displayOf(option: Option): string {
    if (!this.optionDisplay()) return this.labelOf(option);
    const d = this.field(option, this.optionDisplay());
    return d === null || d === undefined || d === '' ? this.labelOf(option) : String(d);
  }
  descriptionOf(option: Option): string {
    if (!this.optionDescription()) return '';
    const d = this.field(option, this.optionDescription());
    return d === null || d === undefined ? '' : String(d);
  }
  disabledOf(option: Option): boolean {
    return !!this.optionDisabled() && option !== null && typeof option === 'object'
      && !!(option as Record<string, unknown>)[this.optionDisabled()];
  }
  optionId(index: number): string { return `${this.baseId}-option-${index}`; }

  // ── open / close ────────────────────────────────────────────────────────

  toggle(): void {
    if (this.isOpen()) this.close();
    else this.open();
    this.focusTrigger();
  }

  /** Opens the list; `keepQuery` (typing, with `filter`) keeps it narrowed, anything else shows every option. */
  open(keepQuery = false): void {
    if (this.isDisabled() || this.isOpen()) return;
    if (!keepQuery) this.query.set('');
    if (!this.overlayRef) {
      this.positionStrategy = createFlexibleConnectedPositionStrategy(this.injector, this.hostEl)
        .withPositions([
          { originX: 'start', originY: 'bottom', overlayX: 'start', overlayY: 'top', offsetY: 6 },
          { originX: 'start', originY: 'top', overlayX: 'start', overlayY: 'bottom', offsetY: -6 },
        ])
        .withPush(false)
        .withFlexibleDimensions(false);
      // Hears the scroll of every cdkScrollable as well as the document's.
      this.overlayRef = createOverlayRef(this.injector, {
        positionStrategy: this.positionStrategy,
        scrollStrategy: createRepositionScrollStrategy(this.injector),
        usePopover: false,
        panelClass: 'kl-select-overlay',
      });
      this.subs.add(this.overlayRef.outsidePointerEvents().subscribe(event => {
        const target = event.target as Node | null;
        if (target && this.hostEl.nativeElement.contains(target)) return;
        this.close();
      }));
      this.subs.add(this.overlayRef.detachments().subscribe(() => this.isOpen.set(false)));
      // The trigger scrolled out of its container: a list floating over
      // other content, tied to nothing visible, goes.
      this.subs.add(this.positionStrategy.positionChanges.subscribe(change => {
        if (change.scrollableViewProperties.isOriginOutsideView) queueMicrotask(() => this.close());
      }));
      // The trigger's width follows the window; the open list follows it.
      this.subs.add(this.viewportRuler.change(50).subscribe(() => {
        if (!this.isOpen()) return;
        this.matchTriggerWidth();
        this.overlayRef?.updatePosition();
      }));
    }
    this.positionStrategy?.withScrollableContainers(this.scrollDispatcher.getAncestorScrollContainers(this.hostEl));
    this.matchTriggerWidth();
    this.forLabel.set(this.associatedLabelText());
    this.overlayRef.attach(new TemplatePortal(this.panelTpl(), this.vcr));
    this.isOpen.set(true);
    const selected = this.shownSelectedIndex();
    if (selected >= 0 && !this.disabledOf(this.shown()[selected])) this.setActive(selected);
    // Editable with free text in the field: nothing active, so Enter keeps it.
    else this.setActive(this.editable() && selected < 0 ? -1 : this.step(-1, 1));
  }

  /** The list is as wide as the trigger, and never narrower than 192px. */
  private matchTriggerWidth(): void {
    const width = this.hostEl.nativeElement.getBoundingClientRect().width;
    this.overlayRef?.updateSize({ width: width || undefined, minWidth: 192 });
  }

  /** Closes the list. A search not committed by now is dropped: the field shows the value again. */
  close(): void {
    if (this.overlayRef?.hasAttached()) this.overlayRef.detach();
    this.isOpen.set(false);
    this.activeIndex.set(-1);
    this.query.set('');
    this.draft.set(null);
  }

  // ── choosing ────────────────────────────────────────────────────────────

  choose(index: number): void {
    const option = this.shown()[index];
    if (option === undefined || this.disabledOf(option)) return;
    const v = this.valueOf(option);
    if (v !== this.value()) {
      this.value.set(v);
      this.onChange(v);
      this.changed.emit(v);
    }
    this.close();
    this.focusTrigger();
  }

  protected hover(index: number): void {
    if (!this.disabledOf(this.shown()[index])) this.activeIndex.set(index);
  }

  protected onType(event: Event): void {
    const text = (event.target as HTMLInputElement).value;
    if (this.searches()) {
      // A search: it narrows the list, which opens to show it, and changes
      // no value until something is chosen.
      this.draft.set(text);
      this.query.set(text);
      if (!this.isOpen()) this.open(true);
    } else {
      this.value.set(text);
      this.onChange(text);
      this.changed.emit(text);
    }
    if (this.isOpen()) {
      // Text that names no option leaves none active (as p-select did), so
      // Enter keeps what was typed instead of taking the old active option.
      const match = this.shown().findIndex(o => this.labelOf(o) === text && !this.disabledOf(o));
      this.setActive(match);
    }
  }

  protected onKeydown(event: KeyboardEvent): void {
    if (this.isDisabled()) return;
    const open = this.isOpen();
    const fixed = !this.editable();
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        if (!open) this.open();
        else this.setActive(this.step(this.activeIndex(), 1));
        return;
      case 'ArrowUp':
        event.preventDefault();
        if (!open) this.open();
        else this.setActive(this.step(this.activeIndex(), -1));
        return;
      case 'Home':
      case 'End':
        if (!fixed || !open) return;
        event.preventDefault();
        this.setActive(event.key === 'Home' ? this.step(-1, 1) : this.step(this.shown().length, -1));
        return;
      case 'Enter':
        if (open && this.activeIndex() >= 0) {
          event.preventDefault();
          this.choose(this.activeIndex());
        } else if (open) {
          // Editable, nothing active: the typed text is the value — already,
          // or (searching) from now on, as a free-text one.
          event.preventDefault();
          const draft = this.draft();
          if (draft !== null) this.commitText(draft);
          this.close();
        } else if (fixed) {
          event.preventDefault();
          this.open();
        }
        return;
      case ' ':
        if (!fixed) return;
        event.preventDefault();
        if (open && this.activeIndex() >= 0) this.choose(this.activeIndex());
        else this.open();
        return;
      case 'Escape':
        if (open) {
          event.preventDefault();
          event.stopPropagation();
          this.close();
        }
        return;
      case 'Tab':
        if (open) this.close();
        return;
      default:
        if (fixed && event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
          this.typeAhead(event.key);
        }
    }
  }

  /** The next enabled option from `from` in `direction`, or `from` if none (clamped, no wrap). */
  private step(from: number, direction: 1 | -1): number {
    const list = this.shown();
    for (let i = from + direction; i >= 0 && i < list.length; i += direction) {
      if (!this.disabledOf(list[i])) return i;
    }
    return from >= 0 && from < list.length ? from : -1;
  }

  private setActive(index: number): void {
    this.activeIndex.set(index);
    if (index < 0) return;
    const el = document.getElementById(this.optionId(index));
    el?.scrollIntoView?.({ block: 'nearest' });
  }

  private typeAhead(char: string): void {
    if (this.typeaheadTimer) clearTimeout(this.typeaheadTimer);
    this.typeahead += char.toLowerCase();
    this.typeaheadTimer = setTimeout(() => (this.typeahead = ''), 500);
    // Fixed lists only, which never filter: shown() is options() here.
    const list = this.shown();
    const start = Math.max(this.isOpen() ? this.activeIndex() : this.shownSelectedIndex(), 0);
    for (let n = 0; n < list.length; n++) {
      const i = (start + (this.typeahead.length === 1 ? 1 : 0) + n) % list.length;
      if (!this.disabledOf(list[i]) && this.displayOf(list[i]).toLowerCase().startsWith(this.typeahead)) {
        if (this.isOpen()) this.setActive(i);
        else this.choose(i);
        return;
      }
    }
  }

  /** Commits typed text as the value, as a choice would; nothing if it is the value already. */
  private commitText(text: string): void {
    if (text === this.value()) return;
    this.value.set(text);
    this.onChange(text);
    this.changed.emit(text);
  }

  /** Leaving the field drops a search that was never committed. */
  protected onBlur(): void {
    this.onTouched();
    if (this.draft() !== null) this.close();
  }

  private focusTrigger(): void {
    this.triggerRef()?.nativeElement.focus();
  }

  /**
   * A click on the <label for> of the fixed trigger: focus it, but do not let
   * the label pass the click on to the button, which would open the list. (The
   * editable trigger is an <input>, which a label only focuses anyway.)
   * Cancelling the label's click in the bubble phase stops its activation.
   */
  protected onDocumentClick(event: MouseEvent): void {
    if (this.editable() || !this.inputId()) return;
    const target = event.target;
    const label = target instanceof Element ? target.closest('label') : null;
    if (!label || label.htmlFor !== this.inputId()) return;
    event.preventDefault();
    if (!this.isDisabled()) this.focusTrigger();
  }

  private associatedLabelText(): string {
    if (this.ariaLabel() || this.ariaLabelledBy()) return '';
    const trigger = this.triggerRef()?.nativeElement as HTMLButtonElement | HTMLInputElement | undefined;
    return Array.from(trigger?.labels ?? [])
      .map(label => label.textContent?.trim() ?? '')
      .filter(Boolean)
      .join(' ');
  }

  // ── ControlValueAccessor ────────────────────────────────────────────────

  writeValue(value: unknown): void { this.value.set(value === undefined ? null : value); }
  registerOnChange(fn: (value: unknown) => void): void { this.onChange = fn; }
  registerOnTouched(fn: () => void): void { this.onTouched = fn; }
  setDisabledState(disabled: boolean): void {
    this.formDisabled.set(disabled);
    if (disabled) this.close();
  }

  ngOnDestroy(): void {
    this.subs.unsubscribe();
    this.overlayRef?.dispose();
    this.overlayRef = null;
    if (this.typeaheadTimer) clearTimeout(this.typeaheadTimer);
  }
}

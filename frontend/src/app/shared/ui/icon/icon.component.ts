import { ChangeDetectionStrategy, Component, ElementRef, effect, inject, input } from '@angular/core';
import { ICONS, type IconName } from './icons';

/**
 * One vendored Lucide icon as inline SVG, drawn in the current text colour.
 *
 *   <kl-icon name="copy" />                     decorative (aria-hidden)
 *   <kl-icon name="check" label="Signed in" />  meaningful (role="img")
 *
 * The markup comes from the constant map in icons.ts, never from input, so
 * writing it as innerHTML cannot inject anything; Angular's [innerHTML]
 * sanitizer would strip the SVG children, which is why it is not used.
 */
@Component({
  selector: 'kl-icon',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: '',
  host: {
    class: 'kl-icon',
    '[attr.data-icon]': 'name()',
    '[attr.aria-hidden]': 'label() ? null : "true"',
    '[attr.role]': 'label() ? "img" : null',
    '[attr.aria-label]': 'label() || null',
    '[style.width.px]': 'size()',
    '[style.height.px]': 'size()',
  },
  styles: [`
    :host { display: inline-flex; flex: none; line-height: 0; }
    :host ::ng-deep svg { width: 100%; height: 100%; display: block; }
  `],
})
export class IconComponent {
  readonly name = input.required<IconName>();
  /** Edge length in px. */
  readonly size = input(16);
  readonly strokeWidth = input(2);
  /** Accessible name; leave empty for a decorative icon next to a text label. */
  readonly label = input('');

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  constructor() {
    effect(() => {
      const body = ICONS[this.name()] ?? '';
      this.host.nativeElement.innerHTML =
        `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor"` +
        ` stroke-width="${this.strokeWidth()}" stroke-linecap="round" stroke-linejoin="round" focusable="false">${body}</svg>`;
    });
  }
}

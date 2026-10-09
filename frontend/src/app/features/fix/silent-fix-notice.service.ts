import { Injectable } from '@angular/core';
import { Subject, Observable } from 'rxjs';
import type { SilentFixNotice } from '../../core/wails.service';

/** Where a notice belongs: the page its click was meant to open. */
export type SilentFixTarget = 'fix' | 'providers';

/**
 * How long a clicked notice waits for its page. The shell navigates straight
 * away; a notice still unclaimed after this did not reach its page (the
 * first-run redirect, say) and would be news from nowhere if it turned up on
 * a later visit.
 */
export const NOTICE_TTL_MS = 10_000;

/**
 * Hands a clicked hotkey-fix notification to the page it belongs to — the
 * Fix page, or Settings' AI Providers tab.
 *
 * The shell receives the click and navigates; the page may already be on
 * screen (same instance, no new ngOnInit — and the router ignores a
 * navigation to the URL it is already on) or about to be created. A page on
 * screen gets the notice from `notices$` and takes it; a page created after
 * the click takes it on creation. Either way it is shown once, and only by
 * its own page.
 */
@Injectable({ providedIn: 'root' })
export class SilentFixNoticeService {
  private pending: SilentFixNotice | null = null;
  private presentedAt = 0;
  private readonly notices = new Subject<SilentFixNotice>();

  /** Overridable clock, for tests. */
  now: () => number = () => Date.now();

  /** Emits each presented notice, for a page already on screen. */
  readonly notices$: Observable<SilentFixNotice> = this.notices.asObservable();

  present(notice: SilentFixNotice): void {
    this.pending = notice;
    this.presentedAt = this.now();
    this.notices.next(notice);
  }

  /** The notice for this page not shown yet, once; null if there is none. */
  take(target: SilentFixTarget): SilentFixNotice | null {
    const notice = this.pending;
    if (!notice) return null;
    if (this.now() - this.presentedAt > NOTICE_TTL_MS) {
      this.pending = null;
      return null;
    }
    if (notice.target !== target) return null;
    this.pending = null;
    return notice;
  }
}

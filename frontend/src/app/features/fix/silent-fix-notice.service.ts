import { Injectable } from '@angular/core';
import { Subject, Observable } from 'rxjs';
import type { SilentFixNotice } from '../../core/wails.service';

/**
 * Hands a clicked silent-fix notification to the Fix page.
 *
 * The shell receives the click and navigates; the page may already be on
 * screen (same instance, no new ngOnInit) or about to be created. A page on
 * screen gets the notice from `notices$` and takes it; a page created after
 * the click takes it on creation. Either way it is shown once.
 */
@Injectable({ providedIn: 'root' })
export class SilentFixNoticeService {
  private pending: SilentFixNotice | null = null;
  private readonly notices = new Subject<SilentFixNotice>();

  /** Emits each presented notice, for a Fix page already on screen. */
  readonly notices$: Observable<SilentFixNotice> = this.notices.asObservable();

  present(notice: SilentFixNotice): void {
    this.pending = notice;
    this.notices.next(notice);
  }

  /** The notice not shown yet, once. */
  take(): SilentFixNotice | null {
    const notice = this.pending;
    this.pending = null;
    return notice;
  }
}

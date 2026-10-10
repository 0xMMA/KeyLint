import { describe, it, expect, beforeEach } from 'vitest';
import { SilentFixNoticeService, NOTICE_TTL_MS } from './silent-fix-notice.service';
import type { SilentFixNotice } from '../../core/wails.service';

function notice(target: string): SilentFixNotice {
  return { id: 'silentfix-1', title: "Fix didn't run", body: 'x', target, detail: '', input: '', output: '' };
}

describe('SilentFixNoticeService', () => {
  let svc: SilentFixNoticeService;
  let clock: number;

  beforeEach(() => {
    svc = new SilentFixNoticeService();
    clock = 1_000_000;
    svc.now = () => clock;
  });

  it('hands a notice to its own page once', () => {
    svc.present(notice('fix'));
    expect(svc.take('providers')).toBeNull();
    expect(svc.take('fix')?.target).toBe('fix');
    expect(svc.take('fix')).toBeNull();
  });

  // A click whose page was never reached (the first-run redirect, say) must
  // not turn up out of context on a later visit.
  it('drops a notice nobody took in time', () => {
    svc.present(notice('fix'));
    clock += NOTICE_TTL_MS + 1;
    expect(svc.take('fix')).toBeNull();
  });
});

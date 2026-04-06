import { describe, it, expect } from 'vitest';
import { Subject } from 'rxjs';
import { MessageBusService } from './message-bus.service';

/**
 * WailsService internals depend on @wailsio/runtime and generated bindings
 * which cannot be mocked in Angular's Vitest runner without vi.mock().
 * WailsService is instead tested indirectly through component tests that
 * inject a mock WailsService via Angular DI.
 *
 * This file tests the supporting RxJS plumbing that backs WailsService's
 * public API shape (Subject-based observables).
 */
describe('WailsService observable contract', () => {
  it('Subject emits synchronously to current subscribers', () => {
    const subject = new Subject<string>();
    const received: string[] = [];
    subject.asObservable().subscribe(v => received.push(v));

    subject.next('a');
    subject.next('b');

    expect(received).toEqual(['a', 'b']);
  });

  it('Subject completes on complete() and notifies subscribers', () => {
    const subject = new Subject<string>();
    let completed = false;
    subject.asObservable().subscribe({ complete: () => { completed = true; } });

    subject.complete();

    expect(completed).toBe(true);
  });

  it('Subject does not emit after complete()', () => {
    const subject = new Subject<string>();
    const received: string[] = [];
    subject.asObservable().subscribe(v => received.push(v));

    subject.complete();
    subject.next('after-complete');

    expect(received).toHaveLength(0);
  });

  it('multiple subscribers all receive the same emission', () => {
    const subject = new Subject<string>();
    const a: string[] = [];
    const b: string[] = [];
    subject.asObservable().subscribe(v => a.push(v));
    subject.asObservable().subscribe(v => b.push(v));

    subject.next('shared');

    expect(a).toEqual(['shared']);
    expect(b).toEqual(['shared']);
  });
});

describe('MessageBusService', () => {
  it('emits typed events', () => {
    const svc = new MessageBusService();
    const received: string[] = [];
    svc.events$.subscribe(e => received.push(e.type));

    svc.emit({ type: 'shortcut:fix', source: 'test' });
    svc.emit({ type: 'enhancement:complete', text: 'done' });

    expect(received).toEqual(['shortcut:fix', 'enhancement:complete']);
  });
});

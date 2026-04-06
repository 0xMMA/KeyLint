import { Injectable } from '@angular/core';
import { Subject, Observable } from 'rxjs';

export type BusEvent =
  | { type: 'shortcut:single'; source: string }
  | { type: 'shortcut:double'; source: string }
  | { type: 'enhancement:complete'; text: string }
  | { type: 'enhancement:error'; message: string };

/** Application-wide RxJS event bus for decoupled feature communication. */
@Injectable({ providedIn: 'root' })
export class MessageBusService {
  private readonly bus = new Subject<BusEvent>();

  readonly events$: Observable<BusEvent> = this.bus.asObservable();

  emit(event: BusEvent): void {
    this.bus.next(event);
  }
}

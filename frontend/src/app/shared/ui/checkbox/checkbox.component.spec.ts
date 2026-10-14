import { describe, it, expect, beforeEach } from 'vitest';
import { Component, signal, ChangeDetectionStrategy } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { CheckboxComponent } from './checkbox.component';

@Component({
  standalone: true,
  imports: [CheckboxComponent, FormsModule],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <kl-checkbox data-testid="copy" [(ngModel)]="copy">Copy the result</kl-checkbox>
    <label for="outside">Outside label</label>
    <kl-checkbox data-testid="outside" inputId="outside" [(ngModel)]="outside" />
  `,
})
class Host {
  readonly copy = signal(true);
  readonly outside = signal(false);
}

describe('kl-checkbox', () => {
  let fixture: ComponentFixture<Host>;
  let host: Host;
  let el: HTMLElement;
  const q = <T extends HTMLElement = HTMLElement>(sel: string) => el.querySelector<T>(sel)!;

  async function render(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [Host] }).compileComponents();
    fixture = TestBed.createComponent(Host);
    host = fixture.componentInstance;
    el = fixture.nativeElement;
    await render();
  });

  it('a native checkbox with its own label, bound to the model', async () => {
    const box = q<HTMLInputElement>('[data-testid="copy"] input[type="checkbox"]');
    expect(box.checked).toBe(true);
    const label = q<HTMLLabelElement>('[data-testid="copy"] label');
    expect(label.htmlFor).toBe(box.id);
    expect(label.textContent).toContain('Copy the result');
    box.click();
    await render();
    expect(host.copy()).toBe(false);
  });

  it('an outside <label for> reaches it through inputId', async () => {
    const box = q<HTMLInputElement>('[data-testid="outside"] input');
    expect(box.id).toBe('outside');
    q<HTMLLabelElement>('label[for="outside"]').click();
    await render();
    expect(host.outside()).toBe(true);
  });

});

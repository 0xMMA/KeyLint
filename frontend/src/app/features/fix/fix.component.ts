import { Component, OnInit, OnDestroy, ChangeDetectorRef } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { Subscription } from 'rxjs';
import { ButtonModule } from 'primeng/button';
import { TextareaModule } from 'primeng/textarea';
import { MessageModule } from 'primeng/message';
import { CheckboxModule } from 'primeng/checkbox';
import { WailsService, type SilentFixNotice } from '../../core/wails.service';
import { LogService } from '../../core/log.service';
import { TextEnhancementService } from '../text-enhancement/text-enhancement.service';
import { SilentFixNoticeService } from './silent-fix-notice.service';

// Module-level cache — survives Angular route navigation (component destroy/recreate).
let _inputCache = '';
let _outputCache = '';
let _autoCopyCache = true;

@Component({
  selector: 'app-fix',
  standalone: true,
  imports: [CommonModule, FormsModule, ButtonModule, TextareaModule, MessageModule, CheckboxModule],
  template: `
    <div class="fix-page">
      <!-- A clicked hotkey-fix notification: what happened, in full. -->
      @if (silentNotice) {
        <p-message data-testid="silent-fix-notice" severity="warn">
          <div class="silent-notice">
            <strong data-testid="silent-fix-notice-title">{{ silentNotice.title }}</strong>
            <span data-testid="silent-fix-notice-body">{{ silentNotice.body }}</span>
            @if (silentNotice.detail) {
              <small data-testid="silent-fix-notice-detail">{{ silentNotice.detail }}</small>
            }
          </div>
        </p-message>
      }

      <div class="fix-textareas">
        <textarea
          data-testid="fix-input"
          pTextarea
          [ngModel]="inputText"
          (ngModelChange)="inputText = $event; _sync($event)"
          rows="10"
          placeholder="Paste your text here…"
          class="fix-textarea"
        ></textarea>

        <textarea
          data-testid="fix-output"
          pTextarea
          [(ngModel)]="outputText"
          rows="10"
          placeholder="Result will appear here…"
          readonly
          class="fix-textarea"
        ></textarea>
      </div>

      <div class="fix-actions">
        <p-button
          data-testid="fix-btn"
          label="Fix"
          icon="pi pi-sparkles"
          (onClick)="fix()"
          [loading]="loading"
          [disabled]="!inputText.trim()"
        />
        <div class="auto-copy-toggle">
          <p-checkbox
            data-testid="auto-copy-checkbox"
            [(ngModel)]="autoCopy"
            (ngModelChange)="_syncAutoCopy($event)"
            [binary]="true"
            inputId="autoCopy"
          />
          <label for="autoCopy">auto copy to clipboard</label>
        </div>
      </div>

      @if (error) {
        <p-message data-testid="fix-error" severity="error" [text]="error" />
      }

      @if (done) {
        <p-message data-testid="fix-done" severity="success" text="Fixed and written to clipboard!" />
      }
    </div>
  `,
  styles: [`
    .fix-page { display: flex; flex-direction: column; gap: 1rem; padding: 2.75rem; height: 100%; box-sizing: border-box; }
    .fix-textareas { display: flex; gap: 2.75rem; flex: 1; }
    .fix-textarea { flex: 1; resize: none; min-height: 200px; }
    .fix-actions { display: flex; align-items: center; gap: 0.5rem; }
    .auto-copy-toggle { display: flex; align-items: center; gap: 0.5rem; margin-left: 0.75rem; font-size: 0.875rem; color: var(--p-text-muted-color); cursor: pointer; }
    .auto-copy-toggle label { cursor: pointer; }
    .silent-notice { display: flex; flex-direction: column; gap: 0.25rem; }
    .silent-notice small { color: var(--p-text-muted-color); }
  `],
})
export class FixComponent implements OnInit, OnDestroy {
  inputText = _inputCache;
  outputText = _outputCache;
  autoCopy = _autoCopyCache;
  loading = false;
  error = '';
  done = false;
  /** The hotkey-fix notification the user clicked to get here, if any. */
  silentNotice: SilentFixNotice | null = null;

  private sub?: Subscription;

  constructor(
    private readonly wails: WailsService,
    private readonly svc: TextEnhancementService,
    private readonly cdr: ChangeDetectorRef,
    private readonly log: LogService,
    private readonly silentNotices: SilentFixNoticeService,
  ) {}

  _sync(value: string): void { _inputCache = value; }
  _syncAutoCopy(value: boolean): void { _autoCopyCache = value; }

  ngOnInit(): void {
    // The hotkey fix runs in Go with the window hidden. This page only hears
    // of it when the user clicks its notification: then it shows what
    // happened, with the text the fix ran on ready to try again.
    const pending = this.silentNotices.take('fix');
    if (pending) this.showSilentNotice(pending);
    this.sub = this.silentNotices.notices$.subscribe(() => {
      const notice = this.silentNotices.take('fix');
      if (notice) {
        this.showSilentNotice(notice);
        this.cdr.detectChanges();
      }
    });
  }

  private showSilentNotice(notice: SilentFixNotice): void {
    this.silentNotice = notice;
    this.error = '';
    this.done = false;
    if (notice.input) {
      this.inputText = notice.input;
      _inputCache = notice.input;
      // Whatever was in the result box belongs to some earlier text.
      this.outputText = notice.output ?? '';
      _outputCache = this.outputText;
    }
  }

  async fix(): Promise<void> {
    if (!this.inputText.trim()) return;
    this.loading = true;
    this.error = '';
    this.done = false;
    this.silentNotice = null;
    this.log.info('fix: enhance started');
    try {
      this.outputText = await this.svc.enhance(this.inputText);
      _outputCache = this.outputText;
      this.log.info('fix: enhance done');
      if (this.autoCopy) {
        try { await this.wails.writeClipboard(this.outputText); } catch { /* ignore */ }
        this.done = true;
      }
    } catch (e: unknown) {
      this.error = e instanceof Error ? e.message : String(e);
      this.log.error('fix: enhance failed: ' + this.error);
    } finally {
      this.loading = false;
      this.cdr.detectChanges();
    }
    if (this.done) setTimeout(() => { this.done = false; this.cdr.detectChanges(); }, 3000);
  }

  async fixClipboard(): Promise<void> {
    this.loading = true;
    this.error = '';
    this.done = false;
    this.silentNotice = null;
    this.log.info('fix: clipboard enhance started');
    try {
      this.inputText = await this.wails.readClipboard();
      _inputCache = this.inputText;
      this.cdr.detectChanges();
      this.outputText = await this.svc.enhance(this.inputText);
      _outputCache = this.outputText;
      this.log.info('fix: clipboard enhance done');
      if (this.autoCopy) {
        try { await this.wails.writeClipboard(this.outputText); } catch { /* ignore */ }
        await this.wails.pasteToForeground();
        this.done = true;
      }
    } catch (e: unknown) {
      this.error = e instanceof Error ? e.message : String(e);
      this.log.error('fix: clipboard enhance failed: ' + this.error);
    } finally {
      this.loading = false;
      this.cdr.detectChanges();
    }
    if (this.done) setTimeout(() => { this.done = false; this.cdr.detectChanges(); }, 3000);
  }

  ngOnDestroy(): void {
    this.sub?.unsubscribe();
  }
}

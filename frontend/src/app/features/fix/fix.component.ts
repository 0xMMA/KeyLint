import { Component, OnInit, OnDestroy, ChangeDetectorRef, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Subscription } from 'rxjs';
import { WailsService, type SilentFixNotice } from '../../core/wails.service';
import { LogService } from '../../core/log.service';
import { providerShortLabel } from '../../core/providers';
import { TextEnhancementService } from '../text-enhancement/text-enhancement.service';
import { effectiveModel, modelName, toModelOption } from '../settings/model-options';
import { SilentFixNoticeService } from './silent-fix-notice.service';
import { wordDiff, type WordDiff } from './word-diff';
import {
  ButtonComponent, CheckboxComponent, MessageComponent, SegmentedComponent, TagComponent, type SegmentOption,
} from '../../shared/ui';

type ResultView = 'changes' | 'clean';

// Module-level cache — survives Angular route navigation (component destroy/recreate).
let _inputCache = '';
let _outputCache = '';
/** The text the cached result was made from: the marks compare against it, not the live input. */
let _sourceCache = '';
let _autoCopyCache = true;
let _viewCache: ResultView = 'changes';

const NO_DIFF: WordDiff = { segments: [], corrections: 0 };

/**
 * The Fix page (docs/ui/design.md, mockup "Fix"): one sheet, the user's text
 * on the left and the result on the right, the result shown as proofreading
 * marks — what the fix inserted underlined in orange, what it removed struck
 * through in grey — or as clean text.
 */
@Component({
  selector: 'app-fix',
  standalone: true,
  imports: [FormsModule, ButtonComponent, CheckboxComponent, MessageComponent, SegmentedComponent, TagComponent],
  host: {
    class: 'fix-host',
    '(keydown.control.enter)': 'onShortcut($event)',
    '(keydown.meta.enter)': 'onShortcut($event)',
  },
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <div class="fix-page">
      <header class="fix-header">
        <h1 class="fix-title">Fix</h1>
        @if (providerLabel) {
          <div class="sent-to" data-testid="fix-sent-to">
            <span class="sent-to-label">Sent to</span>
            <kl-tag data-testid="fix-provider">{{ providerLabel }}</kl-tag>
            @if (modelLabel) {
              <kl-tag data-testid="fix-model">{{ modelLabel }}</kl-tag>
            }
          </div>
        }
      </header>

      <!-- A clicked hotkey-fix notification: what happened, in full. -->
      @if (silentNotice) {
        <kl-message data-testid="silent-fix-notice" severity="warn">
          <strong class="notice-title" data-testid="silent-fix-notice-title">{{ silentNotice.title }}</strong>
          <span data-testid="silent-fix-notice-body">{{ silentNotice.body }}</span>
          @if (silentNotice.detail) {
            <small class="notice-detail" data-testid="silent-fix-notice-detail">{{ silentNotice.detail }}</small>
          }
        </kl-message>
      }

      <section class="fix-sheet" aria-label="Text and result">
        <div class="fix-half fix-input-half">
          <div class="fix-half-head">
            <label for="fix-input" class="fix-half-title">Your text</label>
            <span class="fix-count" data-testid="fix-char-count">{{ charCount }}</span>
          </div>
          <textarea
            id="fix-input"
            data-testid="fix-input"
            class="fix-text fix-textarea"
            spellcheck="false"
            [ngModel]="inputText"
            (ngModelChange)="inputText = $event; _sync($event)"
            placeholder="Paste or type the text you want fixed."
          ></textarea>
        </div>

        <div class="fix-half fix-result-half">
          <div class="fix-half-head fix-result-head">
            <span class="fix-half-title" id="fix-result-title">Result</span>
            @if (outputText) {
              <kl-segmented data-testid="fix-view" ariaLabel="Result view" [options]="views"
                [value]="view" (valueChange)="setView($event)" />
            }
          </div>
          <div class="fix-text fix-result" data-testid="fix-output" role="region" aria-labelledby="fix-result-title"
            [attr.data-view]="outputText ? view : null" tabindex="0">
            @if (!outputText) {
              <p class="fix-empty" data-testid="fix-empty">
                The fixed text appears here, with every correction marked: new words underlined, removed ones struck through.
              </p>
            } @else if (view === 'changes') {
              <!-- Every run is an element: a bare text node here would carry the
                   template's indentation into the pre-wrapped text. A deleted
                   run of whitespace alone is not drawn: a struck-through space
                   only shows as a gap ("done !" for "done!"), and without it
                   the marks still spell the result exactly. -->
              @for (seg of diff.segments; track $index) {
                @switch (seg.kind) {
                  @case ('ins') {<ins class="fx">{{ seg.text }}</ins>}
                  @case ('del') {@if (hasInk(seg.text)) {<del class="fx">{{ seg.text }}</del>}}
                  @default {<span>{{ seg.text }}</span>}
                }
              }
            } @else {
              <span data-testid="fix-clean">{{ outputText }}</span>
            }
          </div>
        </div>
      </section>

      <footer class="fix-footer">
        <div class="fix-actions">
          <kl-button
            data-testid="fix-btn"
            variant="primary"
            size="lg"
            icon="sparkles"
            loadingLabel="Fixing…"
            [loading]="loading"
            [disabled]="!inputText.trim()"
            (click)="fix()"
          >Fix</kl-button>
          @if (outputText) {
            <kl-button data-testid="fix-copy-btn" variant="quiet" size="lg" icon="copy" (click)="copyResult()">
              {{ copiedNow ? 'Copied' : 'Copy result' }}
            </kl-button>
          }
          <kl-checkbox
            class="auto-copy-toggle"
            data-testid="auto-copy-checkbox"
            inputId="autoCopy"
            [(ngModel)]="autoCopy"
            (ngModelChange)="_syncAutoCopy($event)"
          >Copy the result to the clipboard</kl-checkbox>
          @if (outputText) {
            <span class="fix-summary" data-testid="fix-correction-count">
              <span class="fix-summary-count">{{ correctionsLabel }}</span>
            </span>
          }
        </div>
        <span class="fix-shortcut" data-testid="fix-shortcut-hint"><kbd>Ctrl</kbd> <kbd>Enter</kbd> {{ outputText ? 'fixes again' : 'fixes' }}</span>
      </footer>

      @if (error) {
        <kl-message data-testid="fix-error" severity="error" [text]="error" />
      }

      @if (done) {
        <kl-message data-testid="fix-done" severity="success" text="Fixed and written to clipboard!" />
      }
    </div>
  `,
  styles: [`
    :host { display: block; height: 100%; }
    .fix-page {
      display: flex; flex-direction: column; gap: 18px;
      height: 100%; min-height: 560px; padding: 36px 44px 32px;
    }
    .fix-header { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 12px; }
    .fix-title { margin: 0; font-size: 24px; font-weight: 700; letter-spacing: -0.015em; color: var(--kl-text-strong); }
    .sent-to { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font-size: 13px; }
    .sent-to-label { color: var(--kl-muted); }
    .notice-title { color: var(--kl-text-strong); }
    .notice-detail { color: var(--kl-muted); font-size: 13px; }

    /* Two columns from 600px inside the sheet, else stacked. The rows share
       the height, so on a short window each half scrolls its own text instead
       of running past the sheet (whose min height, via .fix-page, keeps both
       rows at 160px or more). */
    .fix-sheet {
      flex: 1; min-height: 0;
      display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); grid-auto-rows: minmax(160px, 1fr);
      background: var(--kl-sheet); border: 1px solid var(--kl-line); border-radius: var(--kl-radius-sheet);
      overflow: hidden; container-type: inline-size;
    }
    .fix-half { display: flex; flex-direction: column; min-height: 0; min-width: 0; }
    .fix-input-half { border-right: 1px solid var(--kl-line); }
    /* The same 600px the grid wraps at (2 x 300px, no gap): the divider turns. */
    @container (width < 600px) {
      .fix-input-half { border-right: 0; border-bottom: 1px solid var(--kl-line); }
    }
    .fix-result-half { background: var(--kl-sheet-alt); }
    .fix-half-head {
      flex: none; display: flex; align-items: center; justify-content: space-between; gap: 12px;
      height: 48px; padding: 0 20px; border-bottom: 1px solid var(--kl-line);
    }
    .fix-result-head { padding-right: 12px; }
    .fix-half-title { font-size: 13px; font-weight: 600; color: var(--kl-text-soft); }
    .fix-count { font-size: 12px; color: var(--kl-muted); }

    .fix-text {
      flex: 1; min-height: 0; margin: 0; padding: 20px 24px;
      font-family: var(--kl-font-text); font-size: 15.5px; line-height: 1.7;
      overflow-y: auto; overflow-wrap: anywhere;
    }
    .fix-textarea {
      resize: none; width: 100%; border: 0; background: transparent; color: var(--kl-text-soft);
    }
    .fix-textarea::placeholder { color: var(--kl-muted); opacity: 1; font-family: var(--kl-font-ui); font-size: 14px; }
    /* Inside the sheet's clipped corners an outer ring would be cut off. */
    .fix-text:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: -4px; border-radius: 8px; }
    .fix-textarea:focus { outline: none; }
    .fix-textarea:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: -4px; }
    .fix-result { color: var(--kl-text); white-space: pre-wrap; }
    .fix-empty {
      margin: 0; max-width: 34ch; white-space: normal;
      font-family: var(--kl-font-ui); font-size: 14px; line-height: 1.5; color: var(--kl-muted);
    }
    /* No side padding or margin on a mark: punctuation right after a
       correction has to sit as close as it does after any other word. The only
       space added is between a struck word and the word replacing it. */
    ins.fx { text-decoration: none; color: var(--kl-mark-ins); border-bottom: 2px solid var(--kl-primary); }
    del.fx { color: var(--kl-muted); text-decoration: line-through; text-decoration-color: var(--kl-muted); }
    del.fx + ins.fx { margin-left: 3px; }

    .fix-footer { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; }
    .fix-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; }
    .auto-copy-toggle { color: var(--kl-muted); margin-left: 4px; }
    .fix-summary { font-size: 13px; color: var(--kl-muted); }
    .fix-summary-count { color: var(--kl-primary-text); font-weight: 600; }
    .fix-shortcut { font-size: 13px; color: var(--kl-muted); }
  `],
})
export class FixComponent implements OnInit, OnDestroy {
  inputText = _inputCache;
  outputText = _outputCache;
  autoCopy = _autoCopyCache;
  view: ResultView = _viewCache;
  diff: WordDiff = _outputCache ? wordDiff(_sourceCache, _outputCache) : NO_DIFF;
  loading = false;
  error = '';
  done = false;
  /** "Copied" on the copy button, briefly, after the user pressed it. */
  copiedNow = false;
  /** The hotkey-fix notification the user clicked to get here, if any. */
  silentNotice: SilentFixNotice | null = null;
  /** The provider and model Fix sends to, for the "Sent to" chips. */
  providerLabel = '';
  modelLabel = '';

  readonly views: readonly SegmentOption[] = [
    { value: 'changes', label: 'Show changes' },
    { value: 'clean', label: 'Clean text' },
  ];

  private subs: Subscription[] = [];
  private destroyed = false;

  constructor(
    private readonly wails: WailsService,
    private readonly svc: TextEnhancementService,
    private readonly cdr: ChangeDetectorRef,
    private readonly log: LogService,
    private readonly silentNotices: SilentFixNoticeService,
  ) {}

  /** Whether a run has anything visible to strike through. */
  hasInk(text: string): boolean {
    return /\S/.test(text);
  }

  get charCount(): string {
    const n = this.inputText.length;
    return `${n.toLocaleString('en')} ${n === 1 ? 'character' : 'characters'}`;
  }

  get correctionsLabel(): string {
    const n = this.diff.corrections;
    if (n === 0) return 'No corrections';
    return `${n} ${n === 1 ? 'correction' : 'corrections'}`;
  }

  _sync(value: string): void { _inputCache = value; }
  _syncAutoCopy(value: boolean): void { _autoCopyCache = value; }

  setView(view: string): void {
    this.view = view === 'clean' ? 'clean' : 'changes';
    _viewCache = this.view;
  }

  ngOnInit(): void {
    // The hotkey fix runs in Go with the window hidden. This page only hears
    // of it when the user clicks its notification: then it shows what
    // happened, with the text the fix ran on ready to try again.
    const pending = this.silentNotices.take('fix');
    if (pending) this.showSilentNotice(pending);
    this.subs.push(
      this.silentNotices.notices$.subscribe(() => {
        const notice = this.silentNotices.take('fix');
        if (notice) {
          this.showSilentNotice(notice);
          this.cdr.detectChanges();
        }
      }),
    );
    // Once per visit: the page is made anew on each navigation, and Go sends
    // no settings event to follow a change made elsewhere.
    void this.loadSentTo();
  }

  /** Ctrl+Enter (Cmd+Enter on a Mac) anywhere on the page runs the fix. */
  onShortcut(event: Event): void {
    event.preventDefault();
    if (!this.loading) void this.fix();
  }

  private showSilentNotice(notice: SilentFixNotice): void {
    this.silentNotice = notice;
    this.error = '';
    this.done = false;
    if (notice.input) {
      this.inputText = notice.input;
      _inputCache = notice.input;
      // Whatever was in the result box belongs to some earlier text.
      this.setResult(notice.input, notice.output ?? '');
    }
  }

  private setResult(source: string, output: string): void {
    this.outputText = output;
    _outputCache = output;
    _sourceCache = source;
    this.diff = output ? wordDiff(source, output) : NO_DIFF;
    this.copiedNow = false;
  }

  /**
   * Which provider and model a fix goes to. The model is the stored Fix choice
   * or KeyLint's default, named as the provider's listing names it (the list
   * is cached in Go, so this does not call the provider each time).
   */
  private async loadSentTo(): Promise<void> {
    try {
      const settings = await this.wails.loadSettings();
      const provider = settings.active_provider;
      if (!provider) return;
      const model = effectiveModel(provider, 'fix', settings.models?.[provider]?.fix);
      this.providerLabel = providerShortLabel(provider);
      this.modelLabel = model ? modelName(model, []) : '';
      if (!this.destroyed) this.cdr.detectChanges();
      if (!model) return;
      const list = await this.wails.listModels(provider);
      this.modelLabel = modelName(model, (list.models ?? []).map(toModelOption));
      if (!this.destroyed) this.cdr.detectChanges();
    } catch {
      // Best effort: without settings the chips stay away.
    }
  }

  async fix(): Promise<void> {
    if (!this.inputText.trim()) return;
    const source = this.inputText;
    this.loading = true;
    this.error = '';
    this.done = false;
    this.silentNotice = null;
    this.log.info('fix: enhance started');
    try {
      this.setResult(source, await this.svc.enhance(source));
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

  /** Puts the clean result on the clipboard, whatever the view shows. */
  async copyResult(): Promise<void> {
    if (!this.outputText) return;
    try {
      await this.wails.writeClipboard(this.outputText);
      this.copiedNow = true;
      this.cdr.detectChanges();
      setTimeout(() => {
        this.copiedNow = false;
        if (!this.destroyed) this.cdr.detectChanges();
      }, 2000);
    } catch (e: unknown) {
      this.error = 'Could not copy: ' + (e instanceof Error ? e.message : String(e));
      this.cdr.detectChanges();
    }
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
      this.setResult(this.inputText, await this.svc.enhance(this.inputText));
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
    this.destroyed = true;
    this.subs.forEach(s => s.unsubscribe());
  }
}

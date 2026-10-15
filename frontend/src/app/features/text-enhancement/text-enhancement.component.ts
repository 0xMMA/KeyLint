import { Component, OnInit, OnDestroy, ChangeDetectorRef, ElementRef, ViewChild, ChangeDetectionStrategy } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { CdkScrollable } from '@angular/cdk/scrolling';
import { Subscription } from 'rxjs';
import { WailsService, type Settings, type SourcePreset } from '../../core/wails.service';
import { describeError } from '../../core/error-message';
import {
  ButtonComponent, InputDirective, MessageComponent, SegmentedComponent, SpinnerComponent,
  TooltipDirective, type SegmentOption,
} from '../../shared/ui';
import { TextEnhancementService } from './text-enhancement.service';
import { MarkdownPipe } from './markdown.pipe';
import { type TraceEntry } from './trace-entry';
import { TraceLogComponent } from './trace-log/trace-log.component';
import { TracePeekComponent } from './trace-peek/trace-peek.component';
import { SelectionBubbleComponent } from './selection-bubble/selection-bubble.component';
import { StepBarComponent, type PyramidizeStep } from './step-bar/step-bar.component';
import { IntakeStepComponent } from './intake-step/intake-step.component';
import { ChoiceStepComponent } from './choice-step/choice-step.component';
import { ResultHeaderComponent } from './result-header/result-header.component';
import { DEFAULT_FORM, formLabel } from './forms';
import { EFFORT_OPTIONS, effectiveModel, effortNote, modelName } from '../settings/model-options';
import { BedrockConnection, bedrockConnectionOf, bedrockRoutable, usesBedrockKey } from '../../core/bedrock';

// Same names as Settings › AI Providers, so the line under the form choice
// names the provider the user sees there.
const PROVIDER_LABELS: Readonly<Record<string, string>> = {
  claude: 'Anthropic API',
  'claude-code': 'Claude Code (installed CLI)',
  openai: 'OpenAI',
  openrouter: 'OpenRouter',
  bedrock: 'AWS Bedrock',
  ollama: 'Ollama (local)',
};

/** Providers that need no credential at all. */
const KEYLESS_PROVIDERS = new Set(['ollama']);

// The banner used to say "No AI API key configured" whatever was wrong, which
// is the one thing that is never wrong with the Claude Code CLI. These match
// the welcome wizard's wording for the same two states.
const MISSING_KEY_MESSAGE = 'No AI API key configured.';
const CLI_NOT_INSTALLED_MESSAGE = 'Claude Code CLI not found on this machine.';
const CLI_NOT_SIGNED_IN_MESSAGE =
  'Claude Code is installed but not signed in. Open a terminal, run `claude`, and sign in.';
const BEDROCK_NO_REGION_MESSAGE = 'No AWS region is set for Bedrock. Choose one in Settings › AI providers.';
const BEDROCK_WORLDWIDE_ONLY_MESSAGE =
  'This AWS region routes Bedrock only worldwide, and Worldwide is not chosen. Choose it, or another region, in Settings › AI providers.';

/** Send back refuses when the backend's window is no longer the result's (PR #130 review). */
const SOURCE_MOVED_MESSAGE =
  'The shortcut was used in another window since — copy the text instead.';

/**
 * One Pyramidize result: the form it was written in, its text as edited
 * since, and what it was made from — the original text, and the window the
 * session came from ('' for pasted text). Edits to the Text step later do not
 * change either.
 */
interface ResultVersion {
  id: string;
  docType: string;
  text: string;
  original: string;
  sourceApp: string;
  appName: string;
}

/** A backend call in flight, and what the working card says meanwhile. */
interface InFlight {
  kind: 'pyramidize' | 'edit';
  label: string;
}

/** What Retry repeats: a call, given the page that is on screen now. */
type RetryAction = (page: TextEnhancementComponent) => Promise<void>;

// ── Module-level state (survives navigation; cleared only on full app restart) ──
//
// Everything a call writes when it lands lives here, not on the component:
// the component is destroyed on navigation, and a reply can land on the next
// one (refreshView repaints whichever page is on screen).

let step: PyramidizeStep = 'intake';
let originalText = '';
// The window title the Pyramidize shortcut captured when it started this
// session; '' for a session started by pasting or typing. It belongs to the
// session, not to the text: editing or emptying the text keeps it, and only a
// new session (a shortcut replace or a paste reset) changes it. Send back
// checks it against the backend's current window before pasting.
let sourceApp = '';
// What the backend says about that title: the form to preselect, whether it is
// remembered, and the name to remember it under ('' when nothing safe can be).
let sourcePreset: SourcePreset | null = null;
let docType = DEFAULT_FORM;
let rememberForm = false;
let instructions = '';
// Every result of the session, oldest first. Going back to the form choice and
// running again adds one; none is ever replaced.
let results: ResultVersion[] = [];
let activeResult = -1;
let traceLog: TraceEntry[] = [];
// The original text the history last took a snapshot of.
let tracedOriginal = '';
let isPreviewMode = false;
let traceLogOpen = false;
let wasCancelled = false;
// Bumped by every run and every cancel: a reply that arrives for an older run
// (cancelled, or overtaken) is dropped.
let runSeq = 0;
let inFlight: InFlight | null = null;
let errorMessage = '';
// The step an error belongs to: it shows there and nowhere else.
let errorStep: PyramidizeStep | null = null;
// The error the last Send Back reported, '' if none.
let sendBackError = '';
let lastRequest: RetryAction | null = null;
// A note that is not a failure of what the user ran (a preset that could not
// be written or forgotten).
let notice = '';
let bannerDismissed = false; // session-only
// The page on screen, so a reply landing after a navigation repaints it.
let view: { refresh(): void } | null = null;

function refreshView(): void {
  view?.refresh();
}

/**
 * Starts Pyramidize over. For specs: the state is module-level and outlives
 * every fixture, and the test runner shares modules between spec files (see
 * .claude/rules/testing.md).
 */
export function resetPyramidizeSession(): void {
  step = 'intake';
  originalText = '';
  sourceApp = '';
  sourcePreset = null;
  docType = DEFAULT_FORM;
  rememberForm = false;
  instructions = '';
  results = [];
  activeResult = -1;
  traceLog = [];
  tracedOriginal = '';
  isPreviewMode = false;
  traceLogOpen = false;
  wasCancelled = false;
  runSeq++;
  inFlight = null;
  clearError();
  notice = '';
  bannerDismissed = false;
}

/** A new session, from the shortcut or a paste: the text, its source, nothing run yet. */
function startSession(text: string, app: string, preset: SourcePreset | null): void {
  runSeq++;
  inFlight = null;
  wasCancelled = false;
  originalText = text;
  sourceApp = app;
  sourcePreset = app ? preset : null;
  // From an app: preselect its remembered form, else email.
  docType = (app && preset?.documentType) || DEFAULT_FORM;
  rememberForm = false;
  instructions = '';
  results = [];
  activeResult = -1;
  traceLog = [];
  tracedOriginal = '';
  clearError();
  notice = '';
}

function clearError(): void {
  errorMessage = '';
  errorStep = null;
  sendBackError = '';
  lastRequest = null;
}

function setError(message: string, at: PyramidizeStep, retry: RetryAction | null): void {
  errorMessage = message;
  errorStep = at;
  lastRequest = retry;
}

function activeResultVersion(): ResultVersion | undefined {
  return results[activeResult];
}

function activeText(): string {
  return activeResultVersion()?.text ?? '';
}

function setActiveText(text: string): void {
  const r = activeResultVersion();
  if (r) r.text = text;
}

/** Room the selection bubble needs, to keep it inside the window. */
const BUBBLE_WIDTH = 340;
const BUBBLE_HEIGHT = 48;

/** How much of the text the Form step quotes. */
const EXCERPT_LENGTH = 90;

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, Math.max(min, max)));
}

function makeId(): string {
  return Math.random().toString(36).slice(2);
}

function addTrace(label: string, snapshot: string, resultId?: string): void {
  traceLog = [
    ...traceLog,
    { id: makeId(), label, snapshot, timestamp: new Date(), resultId },
  ];
}

/** A snapshot of the original text, once per version of it the user ran or brought in. */
function traceOriginal(): void {
  if (originalText.trim() && originalText !== tracedOriginal) {
    addTrace('Original', originalText);
    tracedOriginal = originalText;
  }
}

/**
 * Pyramidize (roadmap E3 phase 3, the "Pyramidize v3 flow" mockup): Text →
 * Form → Result, with a step bar over them. The shortcut brings its text along
 * and opens on Form; without one the page opens on Text.
 *
 * Provider, model and effort come from Settings only; the Form step says
 * which, with a link there. The only choices here are the form and the user's
 * own instructions.
 *
 * Going back to Form and running again always starts from the original text
 * and adds a result; earlier results stay whole and selectable, and the
 * history keeps every snapshot.
 *
 * This component owns the session — state, backend calls, the editor; the
 * steps, the history list, the snapshot preview and the selection bubble are
 * presentational children.
 */
@Component({
  selector: 'app-text-enhancement',
  standalone: true,
  imports: [
    FormsModule, NgTemplateOutlet, CdkScrollable,
    ButtonComponent, InputDirective, MessageComponent, SegmentedComponent, SpinnerComponent, TooltipDirective,
    StepBarComponent, IntakeStepComponent, ChoiceStepComponent, ResultHeaderComponent,
    TraceLogComponent, TracePeekComponent, SelectionBubbleComponent,
    MarkdownPipe,
  ],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <div class="pyramidize-page">
      <section class="canvas-area" cdkScrollable aria-label="Pyramidize">
        <!-- Mounted from the start, so what a run says is announced when it changes. -->
        <div class="sr-only" aria-live="polite" data-testid="live-status">{{ isLoading ? stepLabel : '' }}</div>

        <div class="pz-stack" [class.pz-stack-fill]="shownStep !== 'choice'">
          <app-step-bar
            [current]="shownStep"
            [canChoose]="!!originalTextView.trim()"
            [canSeeResult]="resultFormsView.length > 0"
            [disabled]="isLoading"
            [source]="sourceLabelView"
            (go)="goToStep($event)"
          />

          @if (!bannerDismissedView && !apiKeySet) {
            <kl-message data-testid="api-key-banner" severity="warn" closable (closed)="dismissBanner()">
              <span data-testid="api-key-banner-message">{{ credentialsMessage }}</span>
            </kl-message>
          }

          @if (noticeView) {
            <kl-message data-testid="preset-notice" severity="warn" closable (closed)="dismissNotice()">{{ noticeView }}</kl-message>
          }

          @switch (shownStep) {
            @case ('intake') {
              <app-intake-step
                [text]="originalTextView"
                (textChange)="onOriginalChange($event)"
                (paste)="pasteFromClipboard()"
                (next)="goToStep('choice')"
              />
            }
            @case ('choice') {
              @if (running === 'pyramidize') {
                <div class="pz-working" data-testid="step-indicator">
                  <div class="pz-busy-card">
                    <kl-spinner [size]="20" class="text-primary" />
                    <span class="flex-1 text-sm" data-testid="step-label">{{ stepLabel }}</span>
                    <kl-button #cancelBtn data-testid="cancel-btn" size="sm" (click)="cancelOperation()">Cancel</kl-button>
                  </div>
                </div>
              } @else {
                <app-choice-step
                  [excerpt]="excerptView"
                  [app]="rememberAppView"
                  [rememberedDocType]="rememberedDocTypeView"
                  [runOn]="runOnView"
                  [canRun]="canRun"
                  [docType]="docTypeView"
                  (docTypeChange)="docTypeView = $event"
                  [remember]="rememberView"
                  (rememberChange)="rememberView = $event"
                  [instructions]="instructionsView"
                  (instructionsChange)="instructionsView = $event"
                  (run)="pyramidize()"
                  (editText)="goToStep('intake')"
                  (forget)="forgetPreset()"
                >
                  @if (errorStepView === 'choice') {
                    <ng-container *ngTemplateOutlet="errorRow" />
                  }
                </app-choice-step>
              }
            }
            @case ('result') {
              <app-result-header
                [forms]="resultFormsView"
                [active]="activeResultView"
                (picked)="showResult($event)"
                (back)="goToStep('choice')"
              >
                <div resultTools class="flex flex-wrap items-center gap-2" [attr.inert]="isLoading ? '' : null">
                  <kl-segmented data-testid="canvas-mode" ariaLabel="Editor view" [options]="canvasModes"
                    [value]="isPreviewModeView ? 'preview' : 'edit'" (valueChange)="setPreviewMode($event === 'preview')" />
                  <kl-button #historyToggle data-testid="trace-log-toggle" size="sm" icon="rotate-ccw-clock"
                    [variant]="traceLogOpenView ? 'quiet' : 'ghost'" [pressed]="traceLogOpenView"
                    (click)="toggleTraceLog()">{{ historyLabel }}</kl-button>
                </div>
              </app-result-header>

              <div class="pz-editor">
                <div class="pz-panel" [attr.inert]="isLoading ? '' : null">
                  @if (isPreviewModeView) {
                    <div class="pz-sheet canvas-preview" data-testid="canvas-preview" [innerHTML]="canvasTextView | markdown"></div>
                  } @else {
                    <div class="canvas-edit-wrapper" (mouseup)="onCanvasMouseUp($event)">
                      <textarea
                        #canvasTextarea
                        data-testid="canvas-textarea"
                        class="pz-sheet"
                        aria-label="Pyramidized text"
                        [(ngModel)]="canvasTextView"
                        (keydown)="onCanvasKeydown($event)"
                      ></textarea>
                    </div>
                  }
                  @if (showSelectionBubble && !isPreviewModeView) {
                    <app-selection-bubble
                      [style.top.px]="bubbleY"
                      [style.left.px]="bubbleX"
                      [(instruction)]="selectionInstruction"
                      (apply)="applySelectionInstruction()"
                      (closed)="dismissSelectionBubble()"
                    />
                  }
                </div>

                @if (isLoading) {
                  <div class="pz-busy" data-testid="step-indicator">
                    <div class="pz-busy-card">
                      <kl-spinner [size]="20" class="text-primary" />
                      <span class="flex-1 text-sm" data-testid="step-label">{{ stepLabel }}</span>
                      <kl-button #cancelBtn data-testid="cancel-btn" size="sm" (click)="cancelOperation()">Cancel</kl-button>
                    </div>
                  </div>
                }
              </div>

              @if (errorStepView === 'result') {
                <ng-container *ngTemplateOutlet="errorRow" />
              }

              <div class="flex flex-wrap items-center gap-2.5">
                <label for="pz-global-instruction" class="sr-only">Change the whole text</label>
                <input
                  klInput
                  id="pz-global-instruction"
                  data-testid="global-instruction-input"
                  class="w-auto min-w-0 flex-[1_1_260px]"
                  [(ngModel)]="globalInstruction"
                  placeholder="Change the whole text, e.g. make it shorter"
                  [disabled]="!canvasTextView.trim() || isLoading"
                  (keydown.control.enter)="applyGlobalInstruction()"
                />
                <kl-button
                  data-testid="apply-instruction-btn"
                  size="lg"
                  [variant]="globalInstruction.trim() ? 'primary' : 'quiet'"
                  [disabled]="!globalInstruction.trim() || !canvasTextView.trim() || isLoading"
                  (click)="applyGlobalInstruction()"
                >Apply<span klButtonEnd><kbd>Ctrl</kbd> <kbd>Enter</kbd></span></kl-button>
              </div>

              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="flex flex-wrap gap-2">
                  <kl-button data-testid="copy-rich-text-btn" size="lg" icon="file-text"
                    [disabled]="!canvasTextView" (click)="copyAsRichText()">Copy as rich text</kl-button>
                  <kl-button data-testid="copy-markdown-btn" variant="ghost" size="lg" icon="copy"
                    [disabled]="!canvasTextView" (click)="copyAsMarkdown()">Copy Markdown</kl-button>
                </div>
                @if (activeSendBackApp; as app) {
                  <kl-button data-testid="send-back-btn" variant="primary" size="lg" icon="corner-down-left"
                    [disabled]="!canvasTextView || sendingBack" (click)="sendBack()">{{ 'Send back to ' + app }}</kl-button>
                }
              </div>
            }
          }
        </div>

        <!-- A history snapshot over the editor (UX-06) -->
        @if (shownStep === 'result' && !isLoading && activeEntry; as entry) {
          <app-trace-peek
            data-testid="trace-peek-overlay"
            [entry]="entry"
            [pinned]="!!peekEntry"
            (revert)="revertTo(entry)"
            (closed)="closePeek()"
          />
        }
      </section>

      @if (shownStep === 'result' && traceLogOpenView) {
        <app-trace-log
          data-testid="trace-log-panel"
          [attr.inert]="isLoading ? '' : null"
          [entries]="traceLogView"
          [pinnedId]="peekEntry?.id ?? null"
          (pin)="peekTrace($event)"
          (hover)="hoverTrace($event)"
          (unhover)="clearHoverTrace()"
          (checkpoint)="addCheckpoint()"
          (collapse)="toggleTraceLog(); focusIn(historyToggleRef)"
        />
      }
    </div>

    <ng-template #errorRow>
      @if (errorMessage) {
        <kl-message data-testid="error-row" severity="error">
          <div class="flex items-center gap-2">
            <span class="error-text" [klTooltip]="errorMessage" klTooltipPosition="top">{{ errorMessage }}</span>
            @if (errorMessage !== sendBackError) {
              <!-- Not for a Send Back error: the text it points to is on the
                   clipboard (or was never pasted), and the result's own copy
                   buttons are right there. -->
              <kl-button data-testid="copy-error-btn" variant="ghost" size="sm" icon="copy" iconOnly
                ariaLabel="Copy error" klTooltip="Copy error" klTooltipPosition="top" (click)="copyError()" />
            }
            @if (canRetry) {
              <kl-button data-testid="retry-btn" size="sm" icon="refresh-cw" (click)="retry()">Retry</kl-button>
            }
          </div>
        </kl-message>
      }
    </ng-template>
  `,
  styles: [`
    :host { display: block; height: 100%; }
    .pyramidize-page { display: flex; height: 100%; overflow: hidden; }

    /* The steps scroll here in a window too small for them; at the default
       window size none does: e2e/pyramidize-fit.spec.ts. */
    .canvas-area { position: relative; flex: 1; min-width: 0; display: flex; padding: 24px 36px 24px; overflow-y: auto; }
    .pz-stack { flex: 1; min-width: 0; max-width: 920px; display: flex; flex-direction: column; gap: 20px; }
    /* Text and Result fill the height: their sheet takes what is left. */
    .pz-stack-fill { min-height: 0; }
    .pz-editor { position: relative; flex: 1; min-height: 160px; display: flex; flex-direction: column; }
    .pz-panel { position: relative; flex: 1; min-height: 0; display: flex; flex-direction: column; }
    .canvas-edit-wrapper { flex: 1; min-height: 0; display: flex; flex-direction: column; }

    /* The user's text, on a sheet (design.md: Literata 15.5/1.7). */
    .pz-sheet {
      flex: 1; min-height: 0; width: 100%; margin: 0; padding: 24px 32px; resize: none; overflow-y: auto;
      font-family: var(--kl-font-text); font-size: 15.5px; line-height: 1.7; color: var(--kl-text-sheet);
      background: var(--kl-sheet); border: 1px solid var(--kl-line); border-radius: var(--kl-radius-sheet);
    }
    .pz-sheet:focus { outline: none; border-color: var(--kl-line-control); }
    .pz-sheet:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
    .canvas-preview { overflow-wrap: anywhere; }
    /* The pipe turns a blank line into <br>, so blocks carry no margin of their own. */
    .canvas-preview ::ng-deep :is(h1, h2, h3) {
      margin: 0 0 0.25em; font-weight: 600; line-height: 1.35; color: var(--kl-text-strong);
    }
    .canvas-preview ::ng-deep h1 { font-size: 21px; }
    .canvas-preview ::ng-deep h2 { font-size: 18px; }
    .canvas-preview ::ng-deep h3 { font-size: 16px; }
    .canvas-preview ::ng-deep :is(p, ul, ol) { margin: 0; }
    .canvas-preview ::ng-deep :is(ul, ol) { padding-left: 1.3em; }
    .canvas-preview ::ng-deep ul { list-style: disc; }
    .canvas-preview ::ng-deep ol { list-style: decimal; }
    .canvas-preview ::ng-deep li { margin-bottom: 0.25em; }
    .canvas-preview ::ng-deep strong { font-weight: 600; color: var(--kl-text-strong); }
    .canvas-preview ::ng-deep code {
      padding: 1px 5px; font-family: ui-monospace, monospace; font-size: 0.85em;
      background: var(--kl-raised); border-radius: var(--kl-radius-sm);
    }

    .pz-working { display: flex; flex: 1; align-items: center; justify-content: center; min-height: 420px; }
    .pz-busy {
      position: absolute; inset: 0; z-index: 5; display: flex; align-items: center; justify-content: center;
      background: color-mix(in srgb, var(--kl-ground), transparent 25%); border-radius: var(--kl-radius-sheet);
    }
    .pz-busy-card {
      display: flex; align-items: center; gap: 14px; min-width: 300px; padding: 14px 14px 14px 18px;
      background: var(--kl-raised); border: 1px solid var(--kl-line-control); border-radius: var(--kl-radius-lg);
      box-shadow: var(--kl-shadow-overlay);
    }

    /* Clipped to two lines; the tooltip has it all (UX-03). */
    .error-text {
      flex: 1; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical;
      overflow: hidden; overflow-wrap: anywhere;
    }
  `],
})

export class TextEnhancementComponent implements OnInit, OnDestroy {
  // ── Component views (mirror module-level state) ──

  /** The step on screen: Result only while there is one, Form only while there is text. */
  get shownStep(): PyramidizeStep {
    if (step === 'result' && results.length) return 'result';
    if (step !== 'intake' && originalText.trim()) return 'choice';
    return 'intake';
  }

  get originalTextView(): string { return originalText; }
  set originalTextView(v: string) { originalText = v; }

  /** The result shown; '' while there is none. */
  get canvasTextView(): string { return activeText(); }
  set canvasTextView(v: string) { setActiveText(v); }

  /** The picked form. Picking another one drops an error about the last run (and its Retry). */
  get docTypeView(): string { return docType; }
  set docTypeView(v: string) {
    if (v !== docType) clearError();
    docType = v;
  }

  get rememberView(): boolean { return rememberForm; }
  set rememberView(v: boolean) { rememberForm = v; }

  /** The instructions. Changing them drops an error about the last run (and its Retry). */
  get instructionsView(): string { return instructions; }
  set instructionsView(v: string) {
    if (v !== instructions) clearError();
    instructions = v;
  }

  get traceLogView(): TraceEntry[] { return traceLog; }
  get isPreviewModeView(): boolean { return isPreviewMode; }
  get traceLogOpenView(): boolean { return traceLogOpen; }
  get bannerDismissedView(): boolean { return bannerDismissed; }
  get noticeView(): string { return notice; }

  get isLoading(): boolean { return inFlight !== null; }
  /** What is running: a Pyramidize run (shown on the Form step) or an edit of a result. */
  get running(): InFlight['kind'] | null { return inFlight?.kind ?? null; }
  get stepLabel(): string { return inFlight?.label ?? ''; }

  get errorMessage(): string { return errorMessage; }
  get sendBackError(): string { return sendBackError; }
  /** The step the error row shows on, or null. */
  get errorStepView(): PyramidizeStep | null { return errorMessage ? errorStep : null; }
  get canRetry(): boolean { return lastRequest !== null; }

  /** The session's app by name, as the page shows it ("Outlook" for "Inbox - Outlook"). */
  private get sessionAppName(): string { return sourcePreset?.app || sourceApp; }

  /** The name "Remember <form> for <app>" stores under; '' offers no box (pasted text, or nothing safe to remember). */
  get rememberAppView(): string { return sourceApp ? sourcePreset?.app ?? '' : ''; }

  /** The form remembered for the session's app, or null. */
  get rememberedDocTypeView(): string | null {
    return this.rememberAppView && sourcePreset?.remembered ? sourcePreset.documentType : null;
  }

  /** Where the text came from, for the step bar. */
  get sourceLabelView(): string {
    if (!originalText.trim()) return '';
    return sourceApp ? `From ${this.sessionAppName}` : 'Pasted into KeyLint';
  }

  /** The app the shown result can be sent back to; '' when it was made from pasted text. */
  get activeSendBackApp(): string {
    const r = activeResultVersion();
    return r?.sourceApp ? r.appName : '';
  }

  /** The first line of the text, for the Form step. */
  get excerptView(): string {
    const line = originalText.split('\n').map(l => l.trim()).find(l => l) ?? '';
    return line.length > EXCERPT_LENGTH ? `${line.slice(0, EXCERPT_LENGTH).trimEnd()}…` : line;
  }

  /** Each result's form, oldest first. Kept while the results are: a new array on every check would be NG0100. */
  get resultFormsView(): readonly string[] {
    if (this.formsCache?.of !== results) {
      this.formsCache = { of: results, forms: results.map(r => r.docType) };
    }
    return this.formsCache.forms;
  }
  private formsCache: { of: ResultVersion[]; forms: readonly string[] } | null = null;

  get activeResultView(): number { return activeResult; }

  /** "<model> · <provider> · effort <level>", all from Settings. */
  get runOnView(): string {
    const provider = this.settingsProvider;
    if (!provider) return '';
    const providerLabel = PROVIDER_LABELS[provider] ?? provider;
    const stored = this.settingsModels?.[provider];
    const parts = [modelName(effectiveModel(provider, 'pyramidize', stored?.pyramidize), []), providerLabel].filter(Boolean);
    // Only where the provider does something with effort (not Ollama).
    if (effortNote(provider)) {
      const level = stored?.pyramidize_effort ?? '';
      const label = EFFORT_OPTIONS.find(o => o.value === level)?.label.toLowerCase() ?? level;
      parts.push(level ? `effort ${label}` : 'default effort');
    }
    return parts.join(' · ');
  }

  /** Whether "Pyramidize as …" (and its Enter) can run now. */
  get canRun(): boolean {
    return !!originalText.trim() && !this.isLoading;
  }

  /** The history toggle's label, with how many snapshots there are. */
  get historyLabel(): string {
    const n = traceLog.length;
    return n === 0 ? 'History' : `History, ${n} ${n === 1 ? 'step' : 'steps'}`;
  }

  // ── Component-local state ──
  /** The provider Settings names, read on every visit. */
  settingsProvider = '';
  private settingsModels: Settings['models'] | null = null;
  /** The AWS Bedrock connection from Settings, for the banner. */
  private bedrock: BedrockConnection = bedrockConnectionOf(null);
  sendingBack = false;
  apiKeySet = true;
  /** Set on destroy so a late answer does not refresh a dead view. */
  private destroyed = false;
  /** What the banner says — the reason depends on the provider. */
  credentialsMessage = MISSING_KEY_MESSAGE;
  globalInstruction = '';

  // Selection bubble
  showSelectionBubble = false;
  bubbleX = 0;
  bubbleY = 0;
  selectionInstruction = '';
  private selectionStart = 0;
  private selectionEnd = 0;

  // Trace peek — peekEntry is sticky (click), hoverEntry is transient (mouseenter)
  peekEntry: TraceEntry | null = null;
  hoverEntry: TraceEntry | null = null;

  get activeEntry(): TraceEntry | null { return this.peekEntry ?? this.hoverEntry; }

  private sub?: Subscription;

  @ViewChild('canvasTextarea') canvasTextareaRef?: ElementRef<HTMLTextAreaElement>;
  @ViewChild('historyToggle', { read: ElementRef }) historyToggleRef?: ElementRef<HTMLElement>;

  /**
   * While a step runs, what started it is gone (the Form step) or inert (the
   * editor), which drops its focus; Cancel takes it. When the step ends, a
   * focus left nowhere goes to the result.
   */
  @ViewChild('cancelBtn', { read: ElementRef })
  private set cancelBtn(ref: ElementRef<HTMLElement> | undefined) {
    queueMicrotask(() => {
      if (this.destroyed) return;
      const active = document.activeElement;
      const lost = !active || active === document.body;
      if (ref) {
        if (lost || this.host.nativeElement.querySelector('.pz-editor')?.contains(active)) this.focusIn(ref);
      } else if (lost) {
        this.canvasTextareaRef?.nativeElement.focus();
      }
    });
  }

  /** Focuses the first focusable element in `ref` (a kl-button host, say). */
  focusIn(ref: ElementRef<HTMLElement> | undefined): void {
    const el = ref?.nativeElement;
    const target = el?.matches('button, input, textarea') ? el : el?.querySelector<HTMLElement>('button, input, textarea');
    target?.focus();
  }

  /** After a pinned preview closes, the focus goes back to its history entry. */
  private focusEntry(id: string | undefined): void {
    if (!id) return;
    // Its focus would open the preview again, as a keyboard focus does.
    this.restoringFocus = true;
    this.host.nativeElement.querySelector<HTMLElement>(`[data-entry-id="${id}"]`)?.focus();
    this.restoringFocus = false;
  }
  private restoringFocus = false;

  readonly canvasModes: readonly SegmentOption[] = [
    { value: 'edit', label: 'Edit' },
    { value: 'preview', label: 'Preview' },
  ];

  constructor(
    private readonly wails: WailsService,
    private readonly svc: TextEnhancementService,
    private readonly cdr: ChangeDetectorRef,
    private readonly host: ElementRef<HTMLElement>,
  ) {}

  /** Repaints this page, unless it is gone: what a call that lands calls (refreshView). */
  refresh(): void {
    if (!this.destroyed) this.cdr.detectChanges();
  }

  async ngOnInit(): Promise<void> {
    view = this;
    // Re-read on every visit: the component is destroyed on navigation, and
    // this is where a provider, model or effort chosen in Settings reaches
    // the line under the form choice.
    const settings = await this.wails.loadSettings();
    this.settingsProvider = settings.active_provider ?? '';
    this.settingsModels = settings.models ?? null;
    this.bedrock = bedrockConnectionOf(settings);

    // Not awaited: for the claude-code provider this probes the CLI, which
    // spawns processes — and the page has a banner slot for the answer, so it
    // can paint first and fill it when it arrives. Waiting here was what made
    // the first paint hang behind a process spawn (#55).
    void this.refreshCredentialsBanner();
    // A preset added or removed in Settings › App defaults since the last visit.
    void this.refreshSourcePreset();

    this.refresh();

    this.sub = this.wails.shortcutPyramidize$.subscribe(() => {
      this.wails.takePendingPyramidize();
      void this.loadShortcutClipboard();
    });
    // A shortcut pressed on another page navigated here before this page was
    // listening; the event is gone, the pending flag is not.
    if (this.wails.takePendingPyramidize()) {
      void this.loadShortcutClipboard();
    }
  }

  /** Starts a session from the text the Pyramidize shortcut copied: it opens on Form. */
  private async loadShortcutClipboard(): Promise<void> {
    const clipboardContent = await this.wails.readClipboard();
    const app = await this.wails.getSourceApp();
    // Asked before the confirm, so a declined replace changes nothing below.
    const preset = app ? await this.wails.getSourcePreset(app) : null;

    if ((originalText || results.length) && !confirm('Replace current session with new clipboard content?')) {
      return;
    }
    // A run for the old session would go on, billed, where no Cancel reaches it.
    if (inFlight) void this.svc.cancelOperation();
    startSession(clipboardContent, app, preset);
    this.resetView();
    traceOriginal();
    step = originalText.trim() ? 'choice' : 'intake';
    this.refresh();
  }

  /** What the page shows of an old session: the bubble and a history preview. */
  private resetView(): void {
    this.peekEntry = null;
    this.hoverEntry = null;
    this.closeSelectionBubble();
    this.globalInstruction = '';
  }

  /** Refreshes what is remembered for this session's app. */
  private async refreshSourcePreset(): Promise<void> {
    const title = sourceApp;
    if (!title) return;
    const preset = await this.wails.getSourcePreset(title);
    if (sourceApp !== title) return;
    sourcePreset = preset;
    this.refresh();
  }

  /** The Text step's edits. They do not change where the session came from. */
  onOriginalChange(value: string): void {
    originalText = value;
  }

  /**
   * Refreshes the banner for the provider Settings names.
   *
   * Ollama carries no credential, so asking the keyring about it would always
   * answer "not set" and show a banner the user cannot act on. The Claude Code
   * CLI does have a usable answer — installed and signed in — and each of its
   * two failures needs its own wording.
   *
   * ngOnInit does not await this, so it ends with its own change detection;
   * the app is zoneless and nothing else would repaint the banner.
   */
  private async refreshCredentialsBanner(): Promise<void> {
    const provider = this.settingsProvider;
    let ok: boolean;
    let message = MISSING_KEY_MESSAGE;

    if (!provider) {
      ok = true;
    } else if (provider === 'claude-code') {
      // A failed RPC must not throw out of a call nobody awaits. An unanswerable
      // probe means "cannot tell", which is better shown as the banner than as
      // an unhandled rejection. The Go side caches the answer.
      const status = await this.wails.getClaudeCodeStatus().catch(() => null);
      ok = !!status?.installed && !!status.loggedIn;
      message = status?.installed ? CLI_NOT_SIGNED_IN_MESSAGE : CLI_NOT_INSTALLED_MESSAGE;
    } else if (KEYLESS_PROVIDERS.has(provider)) {
      ok = true;
    } else if (provider === 'bedrock' && !this.bedrock.region) {
      ok = false;
      message = BEDROCK_NO_REGION_MESSAGE;
    } else if (provider === 'bedrock' && !bedrockRoutable(this.bedrock, await this.wails.getBedrockRegions().catch(() => []))) {
      ok = false;
      message = BEDROCK_WORLDWIDE_ONLY_MESSAGE;
    } else if (provider === 'bedrock' && !usesBedrockKey(this.bedrock)) {
      // This computer's AWS sign-in: whether it works is known only by
      // trying, and the error then says what to do.
      ok = true;
    } else {
      const keyStatus = await this.wails.getKeyStatus(provider);
      ok = keyStatus.is_set;
    }

    this.apiKeySet = ok;
    this.credentialsMessage = message;
    this.refresh();
  }

  /**
   * A step bar click, Edit text, Choose the form, Try another form. Nothing
   * moves while a step runs; an error (and its Retry) stays with its step.
   */
  goToStep(target: PyramidizeStep): void {
    if (this.isLoading) return;
    if (target !== step) clearError();
    step = target;
    this.closeSelectionBubble();
    this.peekEntry = null;
    this.hoverEntry = null;
  }

  /** A version chip: shows that result; the others stay as they are. */
  showResult(index: number): void {
    if (index < 0 || index >= results.length || this.isLoading) return;
    activeResult = index;
    this.closeSelectionBubble();
    this.peekEntry = null;
    this.hoverEntry = null;
  }

  setPreviewMode(preview: boolean): void {
    isPreviewMode = preview;
  }

  toggleTraceLog(): void {
    traceLogOpen = !traceLogOpen;
    this.peekEntry = null;
  }

  dismissBanner(): void {
    bannerDismissed = true;
  }

  dismissNotice(): void {
    notice = '';
  }

  onCanvasKeydown(event: KeyboardEvent): void {
    if (event.ctrlKey && event.key === 'Enter') {
      event.preventDefault();
      void this.applyGlobalInstruction();
    }
  }

  /**
   * Paste from clipboard on the Text step. Over a session with text or
   * results it asks first, as the shortcut does, and a yes starts a new
   * session — pasted, so from no app. Into an empty session it only fills the
   * text.
   */
  async pasteFromClipboard(): Promise<void> {
    const text = await this.wails.readClipboard();
    if (originalText.trim() || results.length) {
      if (!confirm('Replace current session with the clipboard content?')) return;
      if (inFlight) void this.svc.cancelOperation();
      startSession(text, '', null);
      this.resetView();
    } else {
      originalText = text;
    }
    this.refresh();
  }

  /**
   * "Pyramidize as <form>": one call from the original text, in the picked
   * form, with the user's instructions. The result is added to the session's
   * results, never written over an earlier one. A ticked "Remember" box
   * stores the form for the app first.
   */
  async pyramidize(): Promise<void> {
    if (!this.canRun) return;

    const form = docType;
    // Running from here on, before anything is awaited: a double click or a
    // held Enter starts one run, not two.
    const seq = ++runSeq;
    wasCancelled = false;
    inFlight = { kind: 'pyramidize', label: `Writing the ${formLabel(form).toLowerCase()}…` };
    clearError();
    this.refresh();

    if (rememberForm && sourceApp && sourcePreset?.app) {
      await this.rememberFormForApp(form);
      if (seq !== runSeq) return; // cancelled, or a new session, meanwhile
    }
    traceOriginal();

    const made = { original: originalText, sourceApp, appName: this.sessionAppName };
    try {
      const result = await this.svc.pyramidize({
        text: made.original,
        documentType: form,
        customInstructions: instructions.trim(),
        // Provider, model and effort: whatever Settings say (the backend reads them).
        provider: '',
        model: '',
        effort: '',
        promptVariant: 0,
      });
      if (seq !== runSeq) return; // cancelled, or overtaken by a newer run
      const version: ResultVersion = { id: makeId(), docType: form, text: result.fullDocument, ...made };
      results = [...results, version];
      activeResult = results.length - 1;
      addTrace(`Pyramidized as ${formLabel(form).toLowerCase()}`, version.text, version.id);
      step = 'result';
    } catch (e: unknown) {
      if (!wasCancelled && seq === runSeq) {
        // Retry runs what is picked now; changing the pick dropped this row anyway.
        setError(`Pyramidize failed: ${describeError(e)}`, 'choice', page => page.pyramidize());
      }
    } finally {
      if (seq === runSeq) inFlight = null;
      refreshView();
    }
  }

  /** Stores `form` as the app's preset, as the ticked box asked; then asks the backend what is remembered now. */
  private async rememberFormForApp(form: string): Promise<void> {
    const app = sourcePreset!.app;
    const title = sourceApp;
    rememberForm = false;
    try {
      await this.svc.setAppPreset({ sourceApp: app, documentType: form });
    } catch (e: unknown) {
      // The run goes ahead; the user hears that nothing was remembered.
      notice = `Could not remember ${formLabel(form)} for ${app}: ${describeError(e)}`;
      return;
    }
    const fresh = await this.wails.getSourcePreset(title);
    if (sourceApp === title) sourcePreset = fresh;
  }

  /** Forget: removes the app's preset, then asks the backend what is remembered now. */
  async forgetPreset(): Promise<void> {
    const preset = sourcePreset;
    const title = sourceApp;
    if (!preset?.remembered || !title) return;
    try {
      await this.svc.deleteAppPreset(preset.app);
    } catch (e: unknown) {
      notice = `Could not forget the form for ${preset.app}: ${describeError(e)}`;
      this.refresh();
      return;
    }
    const fresh = await this.wails.getSourcePreset(title);
    if (sourceApp === title) sourcePreset = fresh;
    this.refresh();
  }

  async cancelOperation(): Promise<void> {
    wasCancelled = true;
    runSeq++;
    inFlight = null;
    this.refresh();
    await this.svc.cancelOperation();
  }

  /** "Change the whole text" for the result shown. */
  async applyGlobalInstruction(): Promise<void> {
    const target = activeResultVersion();
    if (!target || !this.globalInstruction.trim() || !target.text.trim()) return;
    await this.refine(target.id, this.globalInstruction);
  }

  /** A whole-text change of one result, against the text that result was made from. */
  async refine(resultId: string, instruction: string): Promise<void> {
    const target = results.find(r => r.id === resultId);
    if (!target || inFlight) return;

    const seq = ++runSeq;
    wasCancelled = false;
    inFlight = { kind: 'edit', label: 'Refining…' };
    clearError();
    this.refresh();

    try {
      const result = await this.svc.refineGlobal({
        fullCanvas: target.text,
        originalText: target.original,
        instruction,
        documentType: target.docType,
        provider: '',
        model: '',
        effort: '',
      });
      if (seq !== runSeq) return;
      addTrace(`Refined: "${instruction.slice(0, 30)}"`, target.text, target.id);
      target.text = result.newCanvas;
      if (this.globalInstruction === instruction) this.globalInstruction = '';
    } catch (e: unknown) {
      if (!wasCancelled && seq === runSeq) {
        setError(`Refine failed: ${describeError(e)}`, 'result', page => page.refine(resultId, instruction));
      }
    } finally {
      if (seq === runSeq) inFlight = null;
      refreshView();
    }
  }

  onCanvasMouseUp(event: MouseEvent): void {
    // The textarea's own selection, not window.getSelection(): in Chromium
    // (WebView2, so every Windows user) a selection inside a textarea leaves
    // the document selection collapsed, and the bubble never appeared.
    const textarea = this.canvasTextareaRef?.nativeElement;
    const start = textarea?.selectionStart ?? 0;
    const end = textarea?.selectionEnd ?? 0;
    if (!textarea || !textarea.value.slice(start, end).trim()) {
      this.showSelectionBubble = false;
      this.refresh();
      return;
    }

    this.selectionStart = start;
    this.selectionEnd = end;

    this.showSelectionBubble = true;
    // Above and left of the pointer, but never off the window.
    this.bubbleX = clamp(event.clientX - 100, 8, window.innerWidth - BUBBLE_WIDTH - 8);
    this.bubbleY = clamp(event.clientY - 80, 8, window.innerHeight - BUBBLE_HEIGHT - 8);
    this.selectionInstruction = '';
    this.refresh();
  }

  /** Escape or the bubble's close button: back to the text the selection is in. */
  dismissSelectionBubble(): void {
    this.closeSelectionBubble();
    this.canvasTextareaRef?.nativeElement.focus();
  }

  closeSelectionBubble(): void {
    this.showSelectionBubble = false;
    this.selectionInstruction = '';
  }

  /** The selection bubble's Apply, for the result shown. */
  async applySelectionInstruction(): Promise<void> {
    if (!this.selectionInstruction.trim()) return;
    const target = activeResultVersion();
    if (!target) return;
    const textarea = this.canvasTextareaRef?.nativeElement;
    const start = textarea ? textarea.selectionStart : this.selectionStart;
    const end = textarea ? textarea.selectionEnd : this.selectionEnd;
    const selectedText = target.text.slice(start, end);
    const instruction = this.selectionInstruction;
    this.closeSelectionBubble();
    if (!selectedText.trim()) return;
    await this.splice(target.id, start, selectedText, instruction);
  }

  /**
   * Rewrites a passage of one result, against the text it was made from. A
   * retry finds the passage again if the text moved meanwhile.
   */
  async splice(resultId: string, start: number, selectedText: string, instruction: string): Promise<void> {
    const target = results.find(r => r.id === resultId);
    if (!target || inFlight) return;

    const seq = ++runSeq;
    wasCancelled = false;
    inFlight = { kind: 'edit', label: 'Splicing…' };
    clearError();
    this.refresh();

    try {
      const result = await this.svc.splice({
        fullCanvas: target.text,
        originalText: target.original,
        selectedText,
        instruction,
        provider: '',
        model: '',
        effort: '',
      });
      if (seq !== runSeq) return;
      const at = target.text.slice(start, start + selectedText.length) === selectedText
        ? start : target.text.indexOf(selectedText);
      if (at < 0) {
        setError('Splice failed: the selected passage is no longer in the text.', 'result', null);
        return;
      }
      addTrace(`Splice: "${instruction.slice(0, 30)}"`, target.text, target.id);
      target.text = target.text.slice(0, at) + result.rewrittenSection + target.text.slice(at + selectedText.length);
    } catch (e: unknown) {
      if (!wasCancelled && seq === runSeq) {
        setError(`Splice failed: ${describeError(e)}`, 'result',
          page => page.splice(resultId, start, selectedText, instruction));
      }
    } finally {
      if (seq === runSeq) inFlight = null;
      refreshView();
    }
  }

  addCheckpoint(): void {
    const target = activeResultVersion();
    if (target?.text) {
      addTrace('Checkpoint', target.text, target.id);
      this.refresh();
    }
  }

  peekTrace(entry: TraceEntry): void {
    // One Escape closes one thing: an open bubble would close with the preview.
    this.closeSelectionBubble();
    this.peekEntry = entry;
    this.hoverEntry = null;
    this.refresh();
  }

  hoverTrace(entry: TraceEntry): void {
    if (!this.peekEntry && !this.restoringFocus) {
      this.hoverEntry = entry;
      this.refresh();
    }
  }

  clearHoverTrace(): void {
    this.hoverEntry = null;
    this.refresh();
  }

  closePeek(): void {
    const id = this.peekEntry?.id;
    this.peekEntry = null;
    this.hoverEntry = null;
    this.refresh();
    this.focusEntry(id);
  }

  /**
   * Puts a snapshot back into the result it was taken of — showing that
   * result — after saving what was there to the history. A snapshot of the
   * original text belongs to no result and goes into the one shown.
   */
  revertTo(entry: TraceEntry): void {
    const own = entry.resultId ? results.find(r => r.id === entry.resultId) : undefined;
    const target = own ?? activeResultVersion();
    if (!target) return;
    activeResult = results.indexOf(target);
    addTrace(`Reverted to: ${entry.label}`, target.text, target.id);
    target.text = entry.snapshot;
    this.peekEntry = null;
    this.refresh();
    this.focusEntry(entry.id);
  }

  async copyAsMarkdown(): Promise<void> {
    await this.wails.writeClipboard(activeText());
  }

  async copyAsRichText(): Promise<void> {
    const plain = activeText();
    const html = new MarkdownPipe().transform(plain);
    try {
      // Native Clipboard API required here for HTML MIME type support
      // (WailsService.writeClipboard only handles plain text)
      await navigator.clipboard.write([
        new ClipboardItem({
          'text/html': new Blob([html], { type: 'text/html' }),
          'text/plain': new Blob([plain], { type: 'text/plain' }),
        }),
      ]);
    } catch {
      await this.wails.writeClipboard(plain);
    }
  }

  async copyError(): Promise<void> {
    await this.wails.writeClipboard(errorMessage);
  }

  /**
   * Sends the shown result back to the window its session came from — only
   * while the backend's captured window still is that one. A shortcut used in
   * another window since (a replace the user declined) moved the backend's
   * target; pasting there would put the text into the wrong window.
   */
  async sendBack(): Promise<void> {
    const target = activeResultVersion();
    if (!target?.sourceApp || this.sendingBack) return;
    this.sendingBack = true;
    clearError();
    this.refresh();
    try {
      if ((await this.wails.getSourceApp()) !== target.sourceApp) {
        setError(SOURCE_MOVED_MESSAGE, 'result', null);
        sendBackError = SOURCE_MOVED_MESSAGE;
        return;
      }
      // Formatted as the Preview shows it, for the apps that paste HTML.
      await this.svc.sendBack(target.text, new MarkdownPipe().transform(target.text));
    } catch (e: unknown) {
      // The backend's message says whether the text is on the clipboard and
      // what to do next.
      const message = describeError(e);
      setError(message, 'result', page => page.sendBack());
      sendBackError = message;
    } finally {
      this.sendingBack = false;
      this.refresh();
    }
  }

  async retry(): Promise<void> {
    const action = lastRequest;
    if (action) await action(this);
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.sub?.unsubscribe();
    if (view === this) view = null;
  }
}

import { Component, OnInit, OnDestroy, ChangeDetectorRef, ElementRef, ViewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { Subscription } from 'rxjs';
import { WailsService } from '../../core/wails.service';
import { noteForModelSource } from '../../core/model-source';
import { describeError } from '../../core/error-message';
import { DOCUMENT_TYPE_OPTIONS, unavailableProviderName } from '../../core/constants';
import {
  ButtonComponent, InputDirective, MessageComponent, SegmentedComponent, SpinnerComponent,
  TabComponent, TabPanelComponent, TabsComponent, TooltipDirective, type SegmentOption,
} from '../../shared/ui';
import { TextEnhancementService } from './text-enhancement.service';
import { MarkdownPipe } from './markdown.pipe';
import { type TraceEntry } from './trace-entry';
import { OptionsPanelComponent } from './options-panel/options-panel.component';
import { TraceLogComponent } from './trace-log/trace-log.component';
import { TracePeekComponent } from './trace-peek/trace-peek.component';
import { SelectionBubbleComponent } from './selection-bubble/selection-bubble.component';

// ── Module-level state (survives navigation; cleared only on full app restart) ──

// Same names as Settings › AI Providers, so "Settings uses X" below reads as
// the same X the user sees there.
const PROVIDER_OPTIONS = [
  { label: 'Anthropic API', value: 'claude' },
  { label: 'Claude Code (installed CLI)', value: 'claude-code' },
  { label: 'OpenAI', value: 'openai' },
  { label: 'Ollama (local)', value: 'ollama' },
];

/** Lets a user say "use what Settings says" without knowing the model name. */
const DEFAULT_MODEL_OPTION = { id: '', label: 'KeyLint default' };

/** Providers that need no credential at all. */
const KEYLESS_PROVIDERS = new Set(['ollama']);

// The banner used to say "No AI API key configured" whatever was wrong, which
// is the one thing that is never wrong with the Claude Code CLI. These match
// the welcome wizard's wording for the same two states.
const MISSING_KEY_MESSAGE = 'No AI API key configured.';
const CLI_NOT_INSTALLED_MESSAGE = 'Claude Code CLI not found on this machine.';
const CLI_NOT_SIGNED_IN_MESSAGE =
  'Claude Code is installed but not signed in. Open a terminal, run `claude`, and sign in.';

let originalText = '';
let pyramidizedText = '';   // snapshot of most recent foundation call
let canvasText = '';         // live working surface
let sourceApp = '';          // captured source app name (from hotkey)
let docType = 'auto';
let commStyle = 'professional';
let relLevel = 'professional';
let traceLog: TraceEntry[] = [];
let activeTab: 'original' | 'canvas' = 'original';
let isPreviewMode = false;
let traceLogOpen = false;
let wasCancelled = false;
let bannerDismissed = false; // session-only
// Empty until ngOnInit reads the active provider from settings. Initialising
// it to a provider here made that branch unreachable, so the panel always
// opened on Claude whatever the user had configured.
let selectedProvider = '';
// Whether the user picked a provider in this panel this session, other than
// the one Settings names. Without such a pick the panel follows Settings on
// every visit, so a switch made there is not ignored until a restart.
let providerPickedThisSession = false;
// What Settings named when that pick was made. A pick made because Settings
// named a provider that cannot work (a saved AWS Bedrock) is no override of a
// choice; it only stands until Settings names something usable.
let pickedWhileSettingsWas = '';

/**
 * Puts the provider choice back to "follow Settings". For specs: the state is
 * module-level and outlives every fixture, and the test runner shares modules
 * between spec files (see .claude/rules/testing.md).
 */
export function resetPyramidizeProviderSession(): void {
  selectedProvider = '';
  selectedModel = '';
  providerPickedThisSession = false;
  pickedWhileSettingsWas = '';
}
// Empty means "whatever settings say for this provider" — the backend resolves it.
let selectedModel = '';
let qualityThreshold = 0.65;
let advancedOpen = false;

/** Room the selection bubble needs, to keep it inside the window. */
const BUBBLE_WIDTH = 340;
const BUBBLE_HEIGHT = 48;

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, Math.max(min, max)));
}

function makeId(): string {
  return Math.random().toString(36).slice(2);
}

function addTrace(label: string, snapshot: string): void {
  traceLog = [
    ...traceLog,
    { id: makeId(), label, snapshot, timestamp: new Date() },
  ];
}

/**
 * Pyramidize (docs/ui/design.md, mockup "Pyramidize"): the options column on
 * the left, the text on a sheet in the middle (Editor and Original tabs), and
 * the history column on the right when it is open.
 *
 * This component owns the session — state, backend calls, shortcuts; the
 * options column, the history list, the snapshot preview and the selection
 * bubble are presentational children.
 */
@Component({
  selector: 'app-text-enhancement',
  standalone: true,
  imports: [
    FormsModule,
    ButtonComponent, InputDirective, MessageComponent, SegmentedComponent, SpinnerComponent,
    TabsComponent, TabComponent, TabPanelComponent, TooltipDirective,
    OptionsPanelComponent, TraceLogComponent, TracePeekComponent, SelectionBubbleComponent,
    MarkdownPipe,
  ],
  template: `
    <div class="pyramidize-page">

      <app-pyramidize-options
        [providerOptions]="providerOptions"
        [provider]="providerView"
        (providerPicked)="providerView = $event; onProviderChange()"
        [sessionOverrideOf]="sessionOverrideOf"
        [settingsProviderUnusable]="settingsProviderUnusable"
        [unavailableProvider]="unavailableProviderView"
        (followSettings)="followSettingsProvider()"
        [modelOptions]="currentModelOptions"
        [(model)]="modelView"
        [modelListNote]="modelListNote"
        [docTypeOptions]="docTypeOptions"
        [(docType)]="docTypeView"
        (docTypePicked)="onDocTypeChange()"
        [detectedType]="detectedTypeView"
        [commStyleOptions]="commStyleOptions"
        [(commStyle)]="commStyleView"
        [relLevelOptions]="relLevelOptions"
        [(relLevel)]="relLevelView"
        [(customInstructions)]="customInstructions"
        [canPyramidize]="canPyramidize"
        [loading]="isLoading"
        [hasResult]="!!canvasTextView.trim()"
        (pyramidize)="pyramidize()"
        [showBanner]="!bannerDismissedView && !apiKeySet"
        [bannerMessage]="credentialsMessage"
        (dismissBanner)="dismissBanner()"
        [advancedOpen]="advancedOpenView"
        (toggleAdvanced)="toggleAdvanced()"
        [(qualityThreshold)]="qualityThresholdView"
        (thresholdChanged)="saveThreshold()"
      />

      <!-- ── Editor ── -->
      <section class="canvas-area" aria-label="Editor">
        <div class="pz-stack">
          <div class="pz-editor">
            <kl-tabs class="pz-tabs" ariaLabel="Text" [value]="activeTabView" (valueChange)="onTabChange($event)"
              [attr.inert]="isLoading ? '' : null">
              <kl-tab value="canvas" data-testid="tab-canvas">Editor</kl-tab>
              <kl-tab value="original" data-testid="tab-original">Original</kl-tab>
              <div klTabsEnd class="flex flex-wrap items-center justify-end gap-2 py-1">
                @if (activeTabView === 'canvas') {
                  <kl-segmented data-testid="canvas-mode" ariaLabel="Editor view" [options]="canvasModes"
                    [value]="isPreviewModeView ? 'preview' : 'edit'" (valueChange)="setPreviewMode($event === 'preview')" />
                }
                <kl-button #historyToggle data-testid="trace-log-toggle" size="sm" icon="rotate-ccw-clock"
                  [variant]="traceLogOpenView ? 'quiet' : 'ghost'" [pressed]="traceLogOpenView"
                  (click)="toggleTraceLog()">{{ historyLabel }}</kl-button>
              </div>

              <kl-tab-panel value="canvas" class="pz-panel">
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
                      (ngModelChange)="onCanvasChange($event)"
                      placeholder="The pyramidized text appears here, ready to edit."
                      (keydown)="onCanvasKeydown($event)"
                    ></textarea>
                  </div>
                }
                @if (showSelectionBubble && !isPreviewModeView) {
                  <app-selection-bubble
                    [style.top.px]="bubbleY"
                    [style.left.px]="bubbleX"
                    [(instruction)]="selectionInstruction"
                    [disabled]="!!unavailableProviderView"
                    (apply)="applySelectionInstruction()"
                    (closed)="dismissSelectionBubble()"
                  />
                }
              </kl-tab-panel>

              <kl-tab-panel value="original" class="pz-panel">
                @if (!originalTextView) {
                  <div class="pz-empty">
                    <p class="m-0 text-muted">Paste or type the text you want to pyramidize.</p>
                    <kl-button data-testid="paste-from-clipboard-btn" icon="clipboard" (click)="pasteFromClipboard()">
                      Paste from clipboard
                    </kl-button>
                  </div>
                }
                <textarea
                  #originalTextarea
                  data-testid="original-textarea"
                  class="pz-sheet"
                  aria-label="Original text"
                  [(ngModel)]="originalTextView"
                  (ngModelChange)="onOriginalChange($event)"
                  (keydown)="onOriginalKeydown($event)"
                ></textarea>
              </kl-tab-panel>
            </kl-tabs>

            @if (isLoading) {
              <div class="pz-busy" data-testid="step-indicator">
                <div class="pz-busy-card" role="status">
                  <kl-spinner [size]="20" class="text-primary" />
                  <span class="flex-1 text-sm" data-testid="step-label">{{ stepLabel }}</span>
                  <kl-button #cancelBtn data-testid="cancel-btn" size="sm" (click)="cancelOperation()">Cancel</kl-button>
                </div>
              </div>
            }
          </div>

          @if (errorMessage) {
            <kl-message data-testid="error-row" severity="error">
              <div class="flex items-center gap-2">
                <span class="error-text" [klTooltip]="errorMessage" klTooltipPosition="top">{{ errorMessage }}</span>
                @if (errorMessage !== sendBackError) {
                  <!-- Not for a Send Back error: the text it points to is on the
                       clipboard, and copying the error would replace it. -->
                  <kl-button data-testid="copy-error-btn" variant="ghost" size="sm" icon="copy" iconOnly
                    ariaLabel="Copy error" klTooltip="Copy error" klTooltipPosition="top" (click)="copyError()" />
                }
                <kl-button data-testid="retry-btn" size="sm" icon="refresh-cw" (click)="retry()">Retry</kl-button>
              </div>
            </kl-message>
          }

          @if (refinementWarning) {
            <kl-message data-testid="refinement-warning" severity="warn" [text]="refinementWarning" />
          }

          <div class="flex flex-wrap items-center gap-2.5">
            <label for="pz-global-instruction" class="sr-only">Change the whole text</label>
            <input
              #instructionInput
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
              [variant]="globalInstruction.trim() && !unavailableProviderView ? 'primary' : 'quiet'"
              [disabled]="!globalInstruction.trim() || !canvasTextView.trim() || isLoading || !!unavailableProviderView"
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
            @if (sourceAppView) {
              <kl-button data-testid="send-back-btn" variant="primary" size="lg" icon="corner-down-left"
                [disabled]="!canvasTextView || sendingBack" (click)="sendBack()">{{ 'Send back to ' + sourceAppView }}</kl-button>
            }
          </div>
        </div>

        <!-- A history snapshot over the editor (UX-06) -->
        @if (!isLoading && activeEntry; as entry) {
          <app-trace-peek
            data-testid="trace-peek-overlay"
            [entry]="entry"
            [pinned]="!!peekEntry"
            (revert)="revertTo(entry)"
            (closed)="closePeek()"
          />
        }
      </section>

      @if (traceLogOpenView) {
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
  `,
  styles: [`
    :host { display: block; height: 100%; }
    .pyramidize-page { display: flex; height: 100%; overflow: hidden; }

    .canvas-area { position: relative; flex: 1; min-width: 0; display: flex; padding: 12px 28px 24px; overflow: hidden; }
    .pz-stack { flex: 1; min-width: 0; max-width: 880px; display: flex; flex-direction: column; gap: 16px; }
    .pz-editor { position: relative; flex: 1; min-height: 0; display: flex; flex-direction: column; }
    .pz-tabs { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 16px; }
    /* Only the active panel: the inactive one keeps kl-tab-panel's display: none. */
    .pz-panel:not([hidden]) { position: relative; flex: 1; min-height: 0; display: flex; flex-direction: column; }
    .canvas-edit-wrapper { flex: 1; min-height: 0; display: flex; flex-direction: column; }

    /* The user's text, on a sheet (design.md: Literata 15.5/1.7). */
    .pz-sheet {
      flex: 1; min-height: 0; width: 100%; margin: 0; padding: 28px 36px; resize: none; overflow-y: auto;
      font-family: var(--kl-font-text); font-size: 15.5px; line-height: 1.7; color: var(--kl-text-sheet);
      background: var(--kl-sheet); border: 1px solid var(--kl-line); border-radius: var(--kl-radius-sheet);
    }
    .pz-sheet::placeholder { color: var(--kl-muted); opacity: 1; font-family: var(--kl-font-ui); font-size: 14px; }
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

    .pz-empty {
      position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); z-index: 1;
      display: flex; flex-direction: column; align-items: center; gap: 12px; font-size: 14px;
      text-align: center; pointer-events: none;
    }
    .pz-empty kl-button { pointer-events: auto; }

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
  get originalTextView(): string { return originalText; }
  set originalTextView(v: string) { originalText = v; }

  get canvasTextView(): string { return canvasText; }
  set canvasTextView(v: string) { canvasText = v; }

  get docTypeView(): string { return docType; }
  set docTypeView(v: string) { docType = v; }

  get commStyleView(): string { return commStyle; }
  set commStyleView(v: string) { commStyle = v; }

  get relLevelView(): string { return relLevel; }
  set relLevelView(v: string) { relLevel = v; }

  get traceLogView(): TraceEntry[] { return traceLog; }

  get activeTabView(): string { return activeTab; }

  get isPreviewModeView(): boolean { return isPreviewMode; }

  get traceLogOpenView(): boolean { return traceLogOpen; }

  get sourceAppView(): string { return sourceApp; }

  get bannerDismissedView(): boolean { return bannerDismissed; }

  get providerView(): string { return selectedProvider; }
  /**
   * Settings' provider label while this session's pick differs from it, else
   * null. A pick that silently differed from the marker on Settings › AI
   * Providers is the confusion that page exists to remove.
   */
  get sessionOverrideOf(): string | null {
    if (!this.pickDiffersFromSettings || unavailableProviderName(this.settingsProvider)) return null;
    return PROVIDER_OPTIONS.find(p => p.value === this.settingsProvider)?.label ?? this.settingsProvider;
  }

  /**
   * The name of the unusable provider Settings names, while the user is on
   * another one because of it. Not an override, so no link back: going back
   * would land on a provider that cannot run.
   */
  get settingsProviderUnusable(): string | null {
    return this.pickDiffersFromSettings ? unavailableProviderName(this.settingsProvider) : null;
  }

  private get pickDiffersFromSettings(): boolean {
    return providerPickedThisSession && !!this.settingsProvider && selectedProvider !== this.settingsProvider;
  }
  /** Name of a provider settings still names but nothing can use yet (#22). */
  get unavailableProviderView(): string | null { return unavailableProviderName(selectedProvider); }
  set providerView(v: string) { selectedProvider = v; }

  get modelView(): string { return selectedModel; }
  set modelView(v: string) { selectedModel = v; }

  get qualityThresholdView(): number { return qualityThreshold; }
  set qualityThresholdView(v: number) { qualityThreshold = v; }

  get advancedOpenView(): boolean { return advancedOpen; }

  // ── Component-local state ──
  /** The provider Settings names, read on every visit. */
  settingsProvider = '';
  isLoading = false;
  stepLabel = '';
  errorMessage = '';
  sendingBack = false;
  /** The error the last Send Back reported, '' if none. */
  sendBackError = '';
  refinementWarning = '';
  apiKeySet = true;
  /** Set on destroy so a late model list does not refresh a dead view. */
  private destroyed = false;
  /** What the banner says — the reason depends on the provider. */
  credentialsMessage = MISSING_KEY_MESSAGE;
  customInstructions = '';
  globalInstruction = '';
  detectedTypeView = '';

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

  // Retry state
  private lastRequest: (() => Promise<void>) | null = null;

  private sub?: Subscription;

  @ViewChild('canvasTextarea') canvasTextareaRef?: ElementRef<HTMLTextAreaElement>;
  @ViewChild('originalTextarea') private originalTextareaRef?: ElementRef<HTMLTextAreaElement>;
  @ViewChild('historyToggle', { read: ElementRef }) historyToggleRef?: ElementRef<HTMLElement>;

  /**
   * While a step runs the tabs are inert, which drops the focus of a textarea
   * that started it (Ctrl+Enter); Cancel takes it. When the step ends, a focus
   * left nowhere goes back to the text.
   */
  @ViewChild('cancelBtn', { read: ElementRef })
  private set cancelBtn(ref: ElementRef<HTMLElement> | undefined) {
    queueMicrotask(() => {
      if (this.destroyed) return;
      const active = document.activeElement;
      const lost = !active || active === document.body;
      if (ref) {
        if (lost || this.host.nativeElement.querySelector('kl-tabs')?.contains(active)) this.focusIn(ref);
      } else if (lost) {
        const text = activeTab === 'canvas' ? this.canvasTextareaRef : this.originalTextareaRef;
        text?.nativeElement.focus();
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

  readonly providerOptions = PROVIDER_OPTIONS;

  readonly canvasModes: readonly SegmentOption[] = [
    { value: 'edit', label: 'Edit' },
    { value: 'preview', label: 'Preview' },
  ];

  /** Whether the Pyramidize button (and its Ctrl+Enter) can run now. */
  get canPyramidize(): boolean {
    return !!originalText.trim() && !this.isLoading && !this.unavailableProviderView;
  }

  /** The history toggle's label, with how many snapshots there are. */
  get historyLabel(): string {
    const n = traceLog.length;
    return n === 0 ? 'History' : `History, ${n} ${n === 1 ? 'step' : 'steps'}`;
  }

  /** The provider's models, with a leading entry for the configured default. */
  modelOptions: Array<{ id: string; label: string }> = [DEFAULT_MODEL_OPTION];
  /** Why this list is what it is, or "" when it needs no explaining. */
  modelListNote = '';

  get currentModelOptions(): Array<{ id: string; label: string }> {
    return this.modelOptions;
  }


  readonly docTypeOptions = [
    { label: 'Detect automatically', value: 'auto' },
    ...DOCUMENT_TYPE_OPTIONS,
  ];

  readonly commStyleOptions = [
    { label: 'Professional', value: 'professional' },
    { label: 'Casual', value: 'casual' },
    { label: 'Concise', value: 'concise' },
    { label: 'Detailed', value: 'detailed' },
    { label: 'Persuasive', value: 'persuasive' },
    { label: 'Neutral', value: 'neutral' },
    { label: 'Diplomatic', value: 'diplomatic' },
    { label: 'Direct', value: 'direct' },
  ];

  readonly relLevelOptions = [
    { label: 'Professional', value: 'professional' },
    { label: 'Close', value: 'close' },
    { label: 'Authority', value: 'authority' },
    { label: 'Public', value: 'public' },
  ];

  constructor(
    private readonly wails: WailsService,
    private readonly svc: TextEnhancementService,
    private readonly cdr: ChangeDetectorRef,
    private readonly router: Router,
    private readonly host: ElementRef<HTMLElement>,
  ) {}

  async ngOnInit(): Promise<void> {
    sourceApp = await this.wails.getSourceApp();
    const settings = await this.wails.loadSettings();

    // Follow Settings unless the user picked another provider here this
    // session. Re-read on every visit: the component is destroyed on
    // navigation, and the backend never emits settings:changed, so this is
    // where a switch made in Settings reaches this panel.
    this.settingsProvider = settings.active_provider ?? '';
    if (providerPickedThisSession) {
      const caughtUp = selectedProvider === this.settingsProvider;
      const forcedPickOutlived = !!unavailableProviderName(pickedWhileSettingsWas)
        && this.settingsProvider !== pickedWhileSettingsWas;
      // An override ends once Settings agrees with it, so the next switch made
      // there is followed again; a forced pick ends once Settings names
      // something else, because it was never the user's preference.
      if (caughtUp || forcedPickOutlived) {
        providerPickedThisSession = false;
      }
    }
    if (!providerPickedThisSession && this.settingsProvider && selectedProvider !== this.settingsProvider) {
      selectedProvider = this.settingsProvider;
      // Empty means "whatever Settings says for this provider".
      selectedModel = '';
    }

    // Not awaited: for the claude-code provider this probes the CLI, which
    // spawns processes — and the page has a banner slot for the answer, so it
    // can paint first and fill it when it arrives. Waiting here was what made
    // the first paint hang behind a process spawn (#55).
    void this.refreshCredentialsBanner();
    // Same reasoning: the page has nothing to show from the model list yet.
    void this.loadModelOptions();

    qualityThreshold = await this.wails.getQualityThreshold();

    this.cdr.detectChanges();

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

  /** Starts a session from the text the Pyramidize shortcut copied. */
  private async loadShortcutClipboard(): Promise<void> {
    const clipboardContent = await this.wails.readClipboard();
    sourceApp = await this.wails.getSourceApp();

    if (originalText && !confirm('Replace current session with new clipboard content?')) {
      return;
    }

    wasCancelled = false;
    originalText = clipboardContent;
    pyramidizedText = '';
    canvasText = '';
    traceLog = [];
    this.peekEntry = null;
    this.hoverEntry = null;
    this.detectedTypeView = '';
    activeTab = 'original';
    this.errorMessage = '';
    this.refinementWarning = '';

    if (originalText.trim()) {
      addTrace('Original', originalText);
    }
    this.cdr.detectChanges();
  }

  onOriginalChange(value: string): void {
    originalText = value;
  }

  onCanvasChange(value: string): void {
    canvasText = value;
  }

  onDocTypeChange(): void {
    if (docType !== 'auto') {
      this.detectedTypeView = '';
    }
  }

  async onProviderChange(): Promise<void> {
    // Picking Settings' own provider is no override; anything else is one.
    providerPickedThisSession = selectedProvider !== this.settingsProvider;
    pickedWhileSettingsWas = this.settingsProvider;
    // Back to "whatever settings say": a model from the previous provider would
    // not exist on this one.
    selectedModel = '';
    await this.loadModelOptions();
    // The "no API key" banner belongs to the provider, so re-evaluate it here.
    await this.refreshCredentialsBanner();
    this.cdr.detectChanges();
  }

  /** Drops this session's pick and goes back to the provider Settings names. */
  async followSettingsProvider(): Promise<void> {
    selectedProvider = this.settingsProvider;
    await this.onProviderChange();
  }

  /** Loads the current provider's models; the list is shared with Settings. */
  private async loadModelOptions(): Promise<void> {
    const provider = selectedProvider;
    if (unavailableProviderName(provider)) {
      // Nothing to list, and asking would only produce "could not be reached".
      this.modelOptions = [DEFAULT_MODEL_OPTION];
      this.modelListNote = '';
      return;
    }
    const list = await this.wails.listModels(provider);
    if (provider !== selectedProvider) return;
    this.modelOptions = [DEFAULT_MODEL_OPTION, ...(list.models ?? [])];
    this.modelListNote = noteForModelSource(list.source, provider, (list.models?.length ?? 0) > 0);
    if (!this.destroyed) {
      this.cdr.detectChanges();
    }
  }

  /**
   * Refreshes the banner for the provider selected right now.
   *
   * Ollama carries no credential, so asking the keyring about it would always
   * answer "not set" and show a banner the user cannot act on. The Claude Code
   * CLI does have a usable answer — installed and signed in — and each of its
   * two failures needs its own wording.
   *
   * Deliberately one async function rather than a helper pair: the claude-code
   * branch can take a while, so a quick second switch may finish first and a
   * stale answer must not overwrite the current one — hence the provider check
   * before the assignment below.
   *
   * ngOnInit no longer awaits this, so it ends with its own change detection;
   * the app is zoneless and nothing else would repaint the banner.
   */
  private async refreshCredentialsBanner(): Promise<void> {
    const provider = selectedProvider;
    let ok: boolean;
    let message = MISSING_KEY_MESSAGE;

    if (provider === 'claude-code') {
      // A failed RPC must not throw out of a call nobody awaits. An unanswerable
      // probe means "cannot tell", which is better shown as the banner than as
      // an unhandled rejection. The Go side caches the answer, so this is cheap
      // on every switch after the first.
      const status = await this.wails.getClaudeCodeStatus().catch(() => null);
      ok = !!status?.installed && !!status.loggedIn;
      message = status?.installed ? CLI_NOT_SIGNED_IN_MESSAGE : CLI_NOT_INSTALLED_MESSAGE;
    } else if (KEYLESS_PROVIDERS.has(provider) || unavailableProviderName(provider)) {
      // An unavailable provider has its own note; a key banner would mislead.
      ok = true;
    } else {
      const keyStatus = await this.wails.getKeyStatus(provider);
      ok = keyStatus.is_set;
    }

    // A newer switch already answered; this result is stale.
    if (provider !== selectedProvider) return;
    this.apiKeySet = ok;
    this.credentialsMessage = message;
    // The app is zoneless, and ngOnInit no longer awaits this — so without an
    // explicit refresh the banner's state changes and nothing repaints. Same
    // reason loadModelOptions ends this way.
    if (!this.destroyed) {
      this.cdr.detectChanges();
    }
  }

  onTabChange(value: unknown): void {
    activeTab = value as 'original' | 'canvas';
  }

  setPreviewMode(preview: boolean): void {
    isPreviewMode = preview;
  }

  toggleTraceLog(): void {
    traceLogOpen = !traceLogOpen;
    this.peekEntry = null;
  }

  toggleAdvanced(): void {
    advancedOpen = !advancedOpen;
  }

  dismissBanner(): void {
    bannerDismissed = true;
  }

  async saveThreshold(): Promise<void> {
    try {
      await this.wails.setQualityThreshold(qualityThreshold);
    } catch {
      // best-effort
    }
  }

  onOriginalKeydown(event: KeyboardEvent): void {
    if (event.ctrlKey && event.key === 'Enter') {
      event.preventDefault();
      void this.pyramidize();
    }
  }

  onCanvasKeydown(event: KeyboardEvent): void {
    if (event.ctrlKey && event.key === 'Enter') {
      event.preventDefault();
      void this.applyGlobalInstruction();
    }
  }

  async pasteFromClipboard(): Promise<void> {
    const text = await this.wails.readClipboard();
    originalText = text;
    if (originalText.trim()) {
      addTrace('Original', originalText);
    }
    this.cdr.detectChanges();
  }

  async pyramidize(): Promise<void> {
    if (!originalText.trim() || unavailableProviderName(selectedProvider)) return;

    if (canvasText.trim()) {
      if (!confirm('Re-pyramidize? The current editor content will be saved to the history.')) {
        return;
      }
      addTrace('Editor (saved)', canvasText);
    }

    wasCancelled = false;
    this.errorMessage = '';
    this.refinementWarning = '';
    this.isLoading = true;
    this.stepLabel = 'Step 1/2: Detecting…';
    this.cdr.detectChanges();

    const req = {
      text: originalText,
      documentType: docType,
      communicationStyle: commStyle,
      relationshipLevel: relLevel,
      customInstructions: this.customInstructions,
      provider: selectedProvider,
      model: selectedModel,
      promptVariant: 0,
    };

    const doCall = async (): Promise<void> => {
      this.stepLabel = docType === 'auto' ? 'Step 1/2: Detecting…' : 'Step 1/2: Structuring…';
      this.cdr.detectChanges();

      const result = await this.svc.pyramidize(req);

      if (docType === 'auto' && result.detectedType) {
        this.detectedTypeView = this.docTypeLabel(result.detectedType);
        this.stepLabel = 'Step 2/2: Structuring…';
        this.cdr.detectChanges();
      }

      pyramidizedText = result.fullDocument;
      canvasText = result.fullDocument;
      this.refinementWarning = result.refinementWarning ?? '';

      addTrace('Pyramidized', canvasText);
      activeTab = 'canvas';
    };

    this.lastRequest = async () => {
      this.isLoading = true;
      this.errorMessage = '';
      this.stepLabel = 'Step 1/2: Detecting…';
      this.cdr.detectChanges();
      try {
        await doCall();
      } catch (e: unknown) {
        // A failed retry used to clear the error row and show nothing.
        if (!wasCancelled) {
          this.errorMessage = `Pyramidize failed: ${describeError(e)}`;
        }
      } finally {
        this.isLoading = false;
        this.cdr.detectChanges();
      }
    };

    try {
      await doCall();
    } catch (e: unknown) {
      if (!wasCancelled) {
        this.errorMessage = `Pyramidize failed: ${describeError(e)}`;
      }
    } finally {
      this.isLoading = false;
      this.cdr.detectChanges();
    }
  }

  async cancelOperation(): Promise<void> {
    wasCancelled = true;
    await this.svc.cancelOperation();
    this.isLoading = false;
    this.stepLabel = '';
    this.cdr.detectChanges();
  }

  async applyGlobalInstruction(): Promise<void> {
    if (!this.globalInstruction.trim() || !canvasText.trim()) return;
    if (unavailableProviderName(selectedProvider)) return; // the note explains why (#22)

    const instruction = this.globalInstruction;
    this.lastRequest = () => this.applyGlobalInstruction();
    this.isLoading = true;
    this.stepLabel = 'Refining…';
    this.errorMessage = '';
    wasCancelled = false;
    this.cdr.detectChanges();

    try {
      const result = await this.svc.refineGlobal({
        fullCanvas: canvasText,
        originalText,
        instruction,
        documentType: docType,
        communicationStyle: commStyle,
        relationshipLevel: relLevel,
        provider: selectedProvider,
        model: selectedModel,
      });
      addTrace(`Refined: "${instruction.slice(0, 30)}"`, canvasText);
      canvasText = result.newCanvas;
      this.globalInstruction = '';
    } catch (e: unknown) {
      if (!wasCancelled) {
        this.errorMessage = `Refine failed: ${describeError(e)}`;
      }
    } finally {
      this.isLoading = false;
      this.cdr.detectChanges();
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
      this.cdr.detectChanges();
      return;
    }

    this.selectionStart = start;
    this.selectionEnd = end;

    this.showSelectionBubble = true;
    // Above and left of the pointer, but never off the window.
    this.bubbleX = clamp(event.clientX - 100, 8, window.innerWidth - BUBBLE_WIDTH - 8);
    this.bubbleY = clamp(event.clientY - 80, 8, window.innerHeight - BUBBLE_HEIGHT - 8);
    this.selectionInstruction = '';
    this.cdr.detectChanges();
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

  async applySelectionInstruction(): Promise<void> {
    if (!this.selectionInstruction.trim()) return;
    if (unavailableProviderName(selectedProvider)) return; // the note explains why (#22)

    const textarea = this.canvasTextareaRef?.nativeElement;
    const start = textarea ? textarea.selectionStart : this.selectionStart;
    const end = textarea ? textarea.selectionEnd : this.selectionEnd;
    const selectedText = canvasText.slice(start, end);

    if (!selectedText.trim()) {
      this.closeSelectionBubble();
      return;
    }

    const instruction = this.selectionInstruction;
    this.lastRequest = () => this.applySelectionInstruction();
    this.closeSelectionBubble();
    this.isLoading = true;
    this.stepLabel = 'Splicing…';
    wasCancelled = false;
    this.cdr.detectChanges();

    try {
      const result = await this.svc.splice({
        fullCanvas: canvasText,
        originalText,
        selectedText,
        instruction,
        provider: selectedProvider,
        model: selectedModel,
      });
      const before = canvasText.slice(0, start);
      const after = canvasText.slice(end);
      const oldCanvas = canvasText;
      addTrace(`Splice: "${instruction.slice(0, 30)}"`, oldCanvas);
      canvasText = before + result.rewrittenSection + after;
    } catch (e: unknown) {
      if (!wasCancelled) {
        this.errorMessage = `Splice failed: ${describeError(e)}`;
      }
    } finally {
      this.isLoading = false;
      this.cdr.detectChanges();
    }
  }

  addCheckpoint(): void {
    if (canvasText) {
      addTrace('Checkpoint', canvasText);
      this.cdr.detectChanges();
    }
  }

  peekTrace(entry: TraceEntry): void {
    // One Escape closes one thing: an open bubble would close with the preview.
    this.closeSelectionBubble();
    this.peekEntry = entry;
    this.hoverEntry = null;
    this.cdr.detectChanges();
  }

  hoverTrace(entry: TraceEntry): void {
    if (!this.peekEntry && !this.restoringFocus) {
      this.hoverEntry = entry;
      this.cdr.detectChanges();
    }
  }

  clearHoverTrace(): void {
    this.hoverEntry = null;
    this.cdr.detectChanges();
  }

  closePeek(): void {
    const id = this.peekEntry?.id;
    this.peekEntry = null;
    this.hoverEntry = null;
    this.cdr.detectChanges();
    this.focusEntry(id);
  }

  revertTo(entry: TraceEntry): void {
    addTrace(`Reverted to: ${entry.label}`, canvasText);
    canvasText = entry.snapshot;
    this.peekEntry = null;
    activeTab = 'canvas';
    this.cdr.detectChanges();
    this.focusEntry(entry.id);
  }

  async copyAsMarkdown(): Promise<void> {
    await this.wails.writeClipboard(canvasText);
  }

  async copyAsRichText(): Promise<void> {
    const pipe = new MarkdownPipe();
    const html = pipe.transform(canvasText);
    const plain = canvasText;
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
    await this.wails.writeClipboard(this.errorMessage);
  }

  async sendBack(): Promise<void> {
    if (this.sendingBack) return;
    this.lastRequest = () => this.sendBack();
    this.sendingBack = true;
    this.errorMessage = '';
    this.sendBackError = '';
    this.cdr.detectChanges();
    try {
      await this.svc.sendBack(canvasText);
    } catch (e: unknown) {
      // The backend's message says whether the text is on the clipboard and
      // what to do next.
      this.sendBackError = describeError(e);
      this.errorMessage = this.sendBackError;
    } finally {
      this.sendingBack = false;
      this.cdr.detectChanges();
    }
  }

  async retry(): Promise<void> {
    if (this.lastRequest) {
      await this.lastRequest();
    }
  }

  /** "Email" for a detected "EMAIL"; an unknown type as the backend named it. */
  private docTypeLabel(type: string): string {
    const known = DOCUMENT_TYPE_OPTIONS.find(o => o.value === type.toLowerCase());
    return known?.label ?? type.charAt(0).toUpperCase() + type.slice(1).toLowerCase();
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.sub?.unsubscribe();
  }
}

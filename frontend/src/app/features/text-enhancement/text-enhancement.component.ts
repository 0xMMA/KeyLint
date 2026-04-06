import { Component, OnInit, OnDestroy, ChangeDetectorRef, ElementRef, ViewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { Subscription } from 'rxjs';
import { SelectModule } from 'primeng/select';
import { ButtonModule } from 'primeng/button';
import { TextareaModule } from 'primeng/textarea';
import { InputTextModule } from 'primeng/inputtext';
import { InputNumber } from 'primeng/inputnumber';
import { ProgressSpinnerModule } from 'primeng/progressspinner';
import { MessageModule } from 'primeng/message';
import { Tabs, TabList, Tab, TabPanels, TabPanel } from 'primeng/tabs';
import { TooltipModule } from 'primeng/tooltip';
import { WailsService } from '../../core/wails.service';
import { DOCUMENT_TYPE_OPTIONS } from '../../core/constants';
import { TextEnhancementService } from './text-enhancement.service';
import { MarkdownPipe } from './markdown.pipe';

// ── Module-level state (survives navigation; cleared only on full app restart) ──
interface TraceEntry {
  id: string;
  label: string;
  snapshot: string;
  timestamp: Date;
}

const PROVIDER_OPTIONS = [
  { label: 'Anthropic', value: 'claude' },
  { label: 'OpenAI', value: 'openai' },
  { label: 'Ollama', value: 'ollama' },
];

const PROVIDER_MODELS: Record<string, Array<{ label: string; value: string }>> = {
  claude: [
    { label: 'Sonnet 4.6', value: 'claude-sonnet-4-6' },
    { label: 'Opus 4.6', value: 'claude-opus-4-6' },
    { label: 'Haiku 4.5', value: 'claude-haiku-4-5' },
  ],
  openai: [
    { label: 'GPT-5.2', value: 'gpt-5.2' },
    { label: 'GPT-5.2 Pro', value: 'gpt-5.2-pro' },
    { label: 'GPT-4.1', value: 'gpt-4.1' },
    { label: 'GPT-4.1 Mini', value: 'gpt-4.1-mini' },
    { label: 'o3', value: 'o3' },
  ],
  ollama: [
    { label: 'llama3.2', value: 'llama3.2' },
    { label: 'mistral', value: 'mistral' },
    { label: 'gemma3', value: 'gemma3' },
    { label: 'phi4', value: 'phi4' },
    { label: 'qwen2.5', value: 'qwen2.5' },
  ],
};

const DEFAULT_MODELS: Record<string, string> = {
  claude: 'claude-sonnet-4-6',
  openai: 'gpt-5.2',
  ollama: 'llama3.2',
};

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
let selectedProvider = 'claude';
let selectedModel = 'claude-sonnet-4-6';
let qualityThreshold = 0.65;
let advancedOpen = false;

function makeId(): string {
  return Math.random().toString(36).slice(2);
}

function addTrace(label: string, snapshot: string): void {
  traceLog = [
    ...traceLog,
    { id: makeId(), label, snapshot, timestamp: new Date() },
  ];
}

@Component({
  selector: 'app-text-enhancement',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    SelectModule,
    ButtonModule,
    TextareaModule,
    InputTextModule,
    InputNumber,
    ProgressSpinnerModule,
    MessageModule,
    Tabs, TabList, Tab, TabPanels, TabPanel,
    TooltipModule,
    MarkdownPipe,
  ],
  template: `
    <div class="pyramidize-page">

      <!-- ── Left panel ── -->
      <div class="left-panel">

        @if (!bannerDismissedView && !apiKeySet) {
          <div class="api-key-banner" data-testid="api-key-banner">
            <span>⚠ No AI API key configured.</span>
            <p-button
              icon="pi pi-times"
              size="small"
              severity="secondary"
              [text]="true"
              (onClick)="dismissBanner()"
              pTooltip="Dismiss"
              appendTo="body"
            />
          </div>
        }

        <!-- Provider + Model selectors (UX-01) -->
        <div class="form-group">
          <label>Provider</label>
          <p-select
            data-testid="provider-select"
            [(ngModel)]="providerView"
            [options]="providerOptions"
            optionLabel="label"
            optionValue="value"
            (onChange)="onProviderChange()"
          />
        </div>

        <div class="form-group">
          <label>Model</label>
          <p-select
            data-testid="model-select"
            [(ngModel)]="modelView"
            [options]="currentModelOptions"
            optionLabel="label"
            optionValue="value"
          />
        </div>

        <div class="form-group">
          <label>Document Type</label>
          <p-select
            data-testid="doc-type-select"
            [(ngModel)]="docTypeView"
            [options]="docTypeOptions"
            optionLabel="label"
            optionValue="value"
            (onChange)="onDocTypeChange()"
          />
          @if (detectedTypeView) {
            <div class="detection-badge">
              <span class="detection-dot">●</span>
              <span>{{ detectedTypeView }}</span>
            </div>
          }
        </div>

        <div class="form-group">
          <label>Communication Style</label>
          <p-select
            [(ngModel)]="commStyleView"
            [options]="commStyleOptions"
            optionLabel="label"
            optionValue="value"
          />
        </div>

        <div class="form-group">
          <label>Relationship Level</label>
          <p-select
            [(ngModel)]="relLevelView"
            [options]="relLevelOptions"
            optionLabel="label"
            optionValue="value"
          />
        </div>

        <div class="form-group">
          <label>Custom Instructions</label>
          <textarea
            pTextarea
            [(ngModel)]="customInstructions"
            rows="3"
            placeholder="Optional one-off instruction…"
            class="custom-instructions-textarea"
          ></textarea>
        </div>

        <p-button
          data-testid="pyramidize-btn"
          label="Pyramidize"
          icon="pi pi-sparkles"
          [disabled]="!originalTextView.trim() || isLoading"
          (onClick)="pyramidize()"
          [loading]="isLoading"
          class="pyramidize-btn-full"
        >
          <ng-template #content>
            <span>Pyramidize</span>
            <span class="shortcut-hint">Ctrl+↵</span>
          </ng-template>
        </p-button>

        <!-- Advanced section (UX-04) -->
        <div class="advanced-section">
          <button class="advanced-toggle" (click)="toggleAdvanced()" type="button">
            <i class="pi" [class.pi-chevron-right]="!advancedOpenView" [class.pi-chevron-down]="advancedOpenView"></i>
            <span>Advanced</span>
          </button>
          @if (advancedOpenView) {
            <div class="advanced-body">
              <div class="form-group">
                <label>Quality threshold</label>
                <div class="threshold-row">
                  <p-inputnumber
                    [(ngModel)]="qualityThresholdView"
                    [min]="0" [max]="1" [step]="0.05"
                    [maxFractionDigits]="2"
                    [showButtons]="false"
                    inputStyleClass="threshold-input"
                    (onChange)="saveThreshold()"
                  />
                  <span class="threshold-hint">0–1</span>
                </div>
                <small class="hint-text">Scores below this trigger a refinement pass (default 0.65).</small>
              </div>
            </div>
          }
        </div>
      </div>

      <!-- ── Canvas area ── -->
      <div class="canvas-area">

        @if (isLoading) {
          <!-- Step indicator during operations -->
          <div class="step-indicator">
            <p-progressSpinner styleClass="step-spinner" />
            <span class="step-label">{{ stepLabel }}</span>
            <p-button
              data-testid="cancel-btn"
              label="Cancel"
              severity="secondary"
              size="small"
              (onClick)="cancelOperation()"
            />
          </div>
        } @else {
          <div class="tabs-container">
            <p-tabs [value]="activeTabView" (valueChange)="onTabChange($event)">
              <p-tablist>
                <p-tab value="original">Original</p-tab>
                <p-tab value="canvas">Editor</p-tab>
              </p-tablist>
              <p-tabpanels>
                <!-- Original tab -->
                <p-tabpanel value="original">
                  <div class="tab-panel-content">
                    @if (!originalTextView) {
                      <div class="empty-original">
                        <p class="hint-text">Paste or type text to pyramidize.</p>
                        <p-button
                          data-testid="paste-from-clipboard-btn"
                          label="Paste from Clipboard"
                          icon="pi pi-clipboard"
                          severity="secondary"
                          (onClick)="pasteFromClipboard()"
                        />
                      </div>
                    }
                    <textarea
                      #originalTextarea
                      data-testid="original-textarea"
                      pTextarea
                      [(ngModel)]="originalTextView"
                      (ngModelChange)="onOriginalChange($event)"
                      placeholder="Paste or type text to pyramidize…"
                      class="canvas-textarea"
                      (keydown)="onOriginalKeydown($event)"
                    ></textarea>
                  </div>
                </p-tabpanel>

                <!-- Editor tab -->
                <p-tabpanel value="canvas">
                  <div class="tab-panel-content">
                    <div class="canvas-mode-toggle">
                      <p-button
                        label="Edit"
                        size="small"
                        [severity]="!isPreviewModeView ? 'primary' : 'secondary'"
                        (onClick)="setPreviewMode(false)"
                      />
                      <p-button
                        label="Preview"
                        size="small"
                        [severity]="isPreviewModeView ? 'primary' : 'secondary'"
                        (onClick)="setPreviewMode(true)"
                      />
                    </div>

                    @if (isPreviewModeView) {
                      <div
                        class="canvas-preview"
                        [innerHTML]="canvasTextView | markdown"
                      ></div>
                    } @else {
                      <div class="canvas-edit-wrapper" (mouseup)="onCanvasMouseUp($event)">
                        <textarea
                          #canvasTextarea
                          data-testid="canvas-textarea"
                          pTextarea
                          [(ngModel)]="canvasTextView"
                          (ngModelChange)="onCanvasChange($event)"
                          placeholder="Editor will appear here after Pyramidize…"
                          class="canvas-textarea"
                          (keydown)="onCanvasKeydown($event)"
                        ></textarea>
                      </div>
                    }

                    <!-- Selection bubble -->
                    @if (showSelectionBubble && !isPreviewModeView) {
                      <div
                        class="selection-bubble"
                        [style.top.px]="bubbleY"
                        [style.left.px]="bubbleX"
                      >
                        <input
                          pInputText
                          [(ngModel)]="selectionInstruction"
                          placeholder="Ask AI…"
                          class="bubble-input"
                          (keydown.enter)="applySelectionInstruction()"
                        />
                        <p-button
                          icon="pi pi-sparkles"
                          label="Apply"
                          size="small"
                          [disabled]="!selectionInstruction.trim()"
                          (onClick)="applySelectionInstruction()"
                        />
                        <p-button
                          icon="pi pi-times"
                          size="small"
                          severity="secondary"
                          [text]="true"
                          (onClick)="closeSelectionBubble()"
                        />
                      </div>
                    }
                  </div>
                </p-tabpanel>
              </p-tabpanels>
            </p-tabs>
          </div>
        }

        <!-- Error display (UX-03) -->
        @if (errorMessage) {
          <div class="error-row" data-testid="error-row">
            <span
              class="error-text"
              [pTooltip]="errorMessage"
              appendTo="body"
              tooltipPosition="top"
            >❌ {{ errorMessage }}</span>
            <p-button
              icon="pi pi-copy"
              size="small"
              severity="secondary"
              [text]="true"
              pTooltip="Copy error"
              appendTo="body"
              (onClick)="copyError()"
            />
            <p-button label="Retry" size="small" severity="secondary" (onClick)="retry()" />
          </div>
        }

        <!-- Refinement warning -->
        @if (refinementWarning) {
          <div class="refinement-warning">
            ⚠ {{ refinementWarning }}
          </div>
        }

        <!-- Instruction bar -->
        <div class="instruction-bar">
          <input
            #instructionInput
            pInputText
            data-testid="global-instruction-input"
            [(ngModel)]="globalInstruction"
            placeholder="Global instruction… (e.g. 'make it shorter')"
            class="instruction-input"
            [disabled]="!canvasTextView.trim() || isLoading"
            (keydown.control.enter)="applyGlobalInstruction()"
          />
          <p-button
            data-testid="apply-instruction-btn"
            label="Apply"
            icon="pi pi-play"
            size="small"
            [disabled]="!globalInstruction.trim() || !canvasTextView.trim() || isLoading"
            (onClick)="applyGlobalInstruction()"
          >
            <ng-template #content>
              <span>Apply</span>
              <span class="shortcut-hint">Ctrl+↵</span>
            </ng-template>
          </p-button>
        </div>

        <!-- Action row -->
        <div class="action-row">
          <p-button
            data-testid="copy-markdown-btn"
            label="Copy Markdown"
            icon="pi pi-copy"
            severity="secondary"
            size="small"
            [disabled]="!canvasTextView"
            (onClick)="copyAsMarkdown()"
          />
          <p-button
            data-testid="copy-rich-text-btn"
            label="Copy Rich Text"
            icon="pi pi-file-word"
            severity="secondary"
            size="small"
            [disabled]="!canvasTextView"
            (onClick)="copyAsRichText()"
          />
          @if (sourceAppView) {
            <p-button
              data-testid="send-back-btn"
              [label]="'Send back to ' + sourceAppView"
              icon="pi pi-send"
              severity="secondary"
              size="small"
              [disabled]="!canvasTextView"
              (onClick)="sendBack()"
            />
          }
        </div>

        <!-- Trace peek overlay (UX-06) -->
        @if (activeEntry) {
          <div class="trace-peek-overlay" data-testid="trace-peek-overlay">
            <div class="trace-peek-panel">
              <div class="trace-peek-header">
                <span class="trace-peek-title">{{ activeEntry.label }}</span>
                <span class="trace-peek-time">{{ formatTime(activeEntry.timestamp) }}</span>
                @if (!peekEntry) {
                  <span class="trace-peek-hint">Click to pin</span>
                }
                @if (peekEntry) {
                  <p-button
                    icon="pi pi-times"
                    size="small"
                    severity="secondary"
                    [text]="true"
                    (onClick)="closePeek()"
                  />
                }
              </div>
              <pre class="trace-peek-content">{{ activeEntry.snapshot }}</pre>
              <div class="trace-peek-footer">
                <p-button
                  label="Revert to here"
                  size="small"
                  severity="danger"
                  (onClick)="revertTo(activeEntry)"
                />
                @if (peekEntry) {
                  <p-button
                    label="Close"
                    size="small"
                    severity="secondary"
                    (onClick)="closePeek()"
                  />
                }
              </div>
            </div>
          </div>
        }
      </div>

      <!-- ── Trace log panel ── -->
      <div
        class="trace-panel"
        [class.collapsed]="!traceLogOpenView"
        data-testid="trace-log-panel"
      >
        @if (traceLogOpenView) {
          <div class="trace-header">
            <span class="trace-title">Trace Log</span>
            <p-button
              data-testid="add-checkpoint-btn"
              icon="pi pi-plus"
              size="small"
              severity="secondary"
              [text]="true"
              pTooltip="Add checkpoint"
              tooltipPosition="left"
              appendTo="body"
              (onClick)="addCheckpoint()"
            />
            <p-button
              icon="pi pi-chevron-right"
              size="small"
              severity="secondary"
              [text]="true"
              pTooltip="Collapse"
              tooltipPosition="left"
              appendTo="body"
              (onClick)="toggleTraceLog()"
            />
          </div>
          <div class="trace-list">
            @for (entry of traceLogView; track entry.id) {
              <div
                class="trace-entry"
                [class.active]="peekEntry?.id === entry.id"
                (click)="peekTrace(entry)"
                (mouseenter)="hoverTrace(entry)"
                (mouseleave)="clearHoverTrace()"
              >
                <span class="trace-label">{{ entry.label }}</span>
                <span class="trace-time">{{ formatTime(entry.timestamp) }}</span>
              </div>
            }
          </div>
        } @else {
          <div class="trace-icon-strip">
            <p-button
              icon="pi pi-history"
              size="small"
              severity="secondary"
              [text]="true"
              pTooltip="Trace log"
              tooltipPosition="left"
              appendTo="body"
              (onClick)="toggleTraceLog()"
            />
          </div>
        }
      </div>
    </div>
  `,
  styles: [`
    :host { display: block; height: 100%; }

    .pyramidize-page {
      display: flex;
      flex-direction: row;
      height: 100%;
      overflow: hidden;
      gap: 0;
    }

    /* ── Left panel ── */
    .left-panel {
      width: 280px;
      min-width: 280px;
      display: flex;
      flex-direction: column;
      gap: 0.75rem;
      padding: 1rem;
      border-right: 1px solid var(--p-content-border-color);
      overflow-y: auto;
    }

    .api-key-banner {
      background: var(--p-amber-100, #fef3c7);
      color: var(--p-amber-900, #78350f);
      border: 1px solid var(--p-amber-300, #fcd34d);
      border-radius: 6px;
      padding: 0.5rem 0.75rem;
      font-size: 0.8rem;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 0.5rem;
    }

    .detection-badge {
      display: flex;
      align-items: center;
      gap: 0.3rem;
      font-size: 0.75rem;
      color: var(--p-primary-color);
      font-weight: 600;
      letter-spacing: 0.05em;
      margin-top: 0.25rem;
    }
    .detection-dot { font-size: 0.6rem; }

    .form-group {
      display: flex;
      flex-direction: column;
      gap: 0.3rem;
    }
    label {
      font-size: 0.8rem;
      color: var(--p-text-muted-color);
    }

    .custom-instructions-textarea {
      width: 100%;
      resize: vertical;
      font-size: 0.85rem;
    }

    .pyramidize-btn-full {
      width: 100%;
    }
    .pyramidize-btn-full ::ng-deep button {
      width: 100%;
      justify-content: space-between;
    }

    .shortcut-hint {
      font-size: 0.7rem;
      opacity: 0.6;
      margin-left: 0.5rem;
    }

    /* Advanced section (UX-04) */
    .advanced-section {
      border-top: 1px solid var(--p-content-border-color);
      padding-top: 0.5rem;
      margin-top: 0.25rem;
    }
    .advanced-toggle {
      background: none;
      border: none;
      cursor: pointer;
      display: flex;
      align-items: center;
      gap: 0.4rem;
      font-size: 0.8rem;
      color: var(--p-text-muted-color);
      padding: 0.2rem 0;
      width: 100%;
    }
    .advanced-toggle:hover { color: var(--p-text-color); }
    .advanced-body { padding-top: 0.5rem; }
    .threshold-row {
      display: flex;
      align-items: center;
      gap: 0.4rem;
    }
    .threshold-input {
      width: 80px !important;
      font-size: 0.85rem;
    }
    .threshold-hint { font-size: 0.75rem; color: var(--p-text-muted-color); }
    .hint-text { font-size: 0.78rem; color: var(--p-text-muted-color); margin: 0; }

    /* ── Canvas area ── */
    .canvas-area {
      flex: 1;
      display: flex;
      flex-direction: column;
      overflow: hidden;
      padding: 1rem;
      gap: 0.75rem;
      min-width: 0;
      min-height: 0;
      position: relative;
    }

    .step-indicator {
      display: flex;
      align-items: center;
      gap: 1rem;
      padding: 1rem;
      background: var(--p-content-hover-background);
      border-radius: 8px;
    }
    .step-spinner { width: 24px; height: 24px; }
    .step-label { flex: 1; font-size: 0.9rem; }

    /* Tabs container — must flex-grow to fill available space (UX-07) */
    .tabs-container {
      flex: 1;
      overflow: hidden;
      min-height: 0;
      display: flex;
      flex-direction: column;
    }
    .tabs-container ::ng-deep .p-tabs {
      flex: 1;
      display: flex;
      flex-direction: column;
      overflow: hidden;
      min-height: 0;
    }
    /* display:flex on the CONTAINER (.p-tabpanels) is safe — it does not make hidden
       panels visible. The [hidden] attribute sets display:none on each inactive
       .p-tabpanel element itself, so those children don't participate in flex layout.
       Without display:flex here the active panel's flex:1 has no effect (flex
       properties only work inside a flex formatting context). */
    .tabs-container ::ng-deep .p-tabpanels {
      flex: 1;
      overflow: hidden;
      min-height: 0;
      display: flex;
      flex-direction: column;
    }
    /* Active panel fills the .p-tabpanels flex container.
       We must NOT set display on the generic .p-tabpanel selector — that would
       override the UA-stylesheet display:none applied via the [hidden] attribute
       on inactive panels and make them visible simultaneously. */
    .tabs-container ::ng-deep .p-tabpanel:not([hidden]) {
      flex: 1;
      overflow: hidden;
      min-height: 0;
      display: flex;
      flex-direction: column;
    }

    .tab-panel-content {
      flex: 1;
      overflow: hidden;
      min-height: 0;
      display: flex;
      flex-direction: column;
      gap: 0.5rem;
      position: relative;
    }

    .empty-original {
      position: absolute;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 0.75rem;
      pointer-events: none;
      z-index: 1;
    }
    .empty-original p-button { pointer-events: all; }

    /* Canvas textarea and preview fill remaining height (UX-07) */
    .canvas-textarea {
      flex: 1;
      min-height: 0;
      resize: none;
      width: 100%;
      font-family: var(--p-font-family);
      font-size: 0.9rem;
      line-height: 1.6;
    }

    .canvas-mode-toggle {
      display: flex;
      gap: 0.5rem;
      flex-shrink: 0;
    }

    .canvas-preview {
      flex: 1;
      min-height: 0;
      padding: 1rem;
      border: 1px solid var(--p-content-border-color);
      border-radius: 6px;
      overflow-y: auto;
      line-height: 1.7;
    }
    .canvas-preview ::ng-deep h1 { font-size: 1.4rem; margin: 0.5rem 0; }
    .canvas-preview ::ng-deep h2 { font-size: 1.2rem; margin: 0.5rem 0; }
    .canvas-preview ::ng-deep h3 { font-size: 1rem; margin: 0.4rem 0; }
    .canvas-preview ::ng-deep p  { margin: 0.4rem 0; }
    .canvas-preview ::ng-deep ul, .canvas-preview ::ng-deep ol { padding-left: 1.5rem; margin: 0.4rem 0; }
    .canvas-preview ::ng-deep code {
      background: var(--p-content-hover-background);
      padding: 1px 4px;
      border-radius: 3px;
      font-family: monospace;
      font-size: 0.85em;
    }

    .canvas-edit-wrapper {
      flex: 1;
      min-height: 0;
      display: flex;
      flex-direction: column;
      position: relative;
    }

    /* Selection bubble */
    .selection-bubble {
      position: fixed;
      background: var(--p-surface-overlay, var(--p-surface-card));
      border: 1px solid var(--p-content-border-color);
      border-radius: 8px;
      padding: 0.5rem;
      display: flex;
      gap: 0.4rem;
      align-items: center;
      z-index: 1000;
      box-shadow: 0 4px 16px rgba(0,0,0,0.25);
    }
    .bubble-input { width: 180px; font-size: 0.85rem; }

    /* Error row — clipped to 2 lines (UX-03) */
    .error-row {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      color: var(--p-red-400, #f87171);
      font-size: 0.85rem;
      padding: 0.5rem 0.75rem;
      background: var(--p-content-hover-background);
      border-radius: 6px;
      flex-shrink: 0;
    }
    .error-text {
      flex: 1;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
      overflow: hidden;
      word-break: break-word;
      cursor: default;
    }

    .refinement-warning {
      font-size: 0.8rem;
      color: var(--p-amber-400, #fbbf24);
      padding: 0.4rem 0.5rem;
      flex-shrink: 0;
    }

    /* Instruction bar */
    .instruction-bar {
      display: flex;
      gap: 0.5rem;
      align-items: center;
      border-top: 1px solid var(--p-content-border-color);
      padding-top: 0.75rem;
      flex-shrink: 0;
    }
    .instruction-input { flex: 1; }

    /* Action row */
    .action-row {
      display: flex;
      gap: 0.5rem;
      flex-wrap: wrap;
      flex-shrink: 0;
    }

    /* Trace peek overlay — covers the canvas area (UX-06) */
    .trace-peek-overlay {
      position: absolute;
      inset: 0;
      background: rgba(0,0,0,0.45);
      display: flex;
      align-items: stretch;
      z-index: 50;
      padding: 0.75rem;
    }
    .trace-peek-panel {
      flex: 1;
      background: var(--p-surface-card, var(--p-surface-900));
      border: 1px solid var(--p-content-border-color);
      border-radius: 8px;
      display: flex;
      flex-direction: column;
      overflow: hidden;
    }
    .trace-peek-header {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      padding: 0.75rem 1rem;
      border-bottom: 1px solid var(--p-content-border-color);
      flex-shrink: 0;
    }
    .trace-peek-title { flex: 1; font-weight: 600; font-size: 0.9rem; }
    .trace-peek-time { font-size: 0.75rem; color: var(--p-text-muted-color); }
    .trace-peek-hint { font-size: 0.7rem; color: var(--p-text-muted-color); font-style: italic; }
    .trace-peek-content {
      flex: 1;
      overflow-y: auto;
      white-space: pre-wrap;
      word-break: break-word;
      padding: 1rem;
      margin: 0;
      font-size: 0.85rem;
      line-height: 1.6;
      font-family: var(--p-font-family);
    }
    .trace-peek-footer {
      display: flex;
      gap: 0.5rem;
      padding: 0.75rem 1rem;
      border-top: 1px solid var(--p-content-border-color);
      flex-shrink: 0;
    }

    /* ── Trace log panel ── */
    .trace-panel {
      width: 260px;
      min-width: 260px;
      border-left: 1px solid var(--p-content-border-color);
      display: flex;
      flex-direction: column;
      overflow: hidden;
      transition: width 0.2s ease;
    }
    .trace-panel.collapsed {
      width: 42px;
      min-width: 42px;
    }

    .trace-icon-strip {
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: 0.5rem 0;
    }

    .trace-header {
      display: flex;
      align-items: center;
      gap: 0.25rem;
      padding: 0.5rem 0.75rem;
      border-bottom: 1px solid var(--p-content-border-color);
    }
    .trace-title { flex: 1; font-size: 0.8rem; font-weight: 600; }

    .trace-list {
      flex: 1;
      overflow-y: auto;
      padding: 0.25rem 0;
    }

    .trace-entry {
      display: flex;
      flex-direction: column;
      padding: 0.4rem 0.75rem;
      cursor: pointer;
      border-bottom: 1px solid var(--p-content-border-color);
      transition: background 0.1s;
    }
    .trace-entry:hover { background: var(--p-content-hover-background); }
    .trace-entry.active { background: var(--p-highlight-background); }
    .trace-label { font-size: 0.8rem; }
    .trace-time { font-size: 0.7rem; color: var(--p-text-muted-color); }
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
  set providerView(v: string) { selectedProvider = v; }

  get modelView(): string { return selectedModel; }
  set modelView(v: string) { selectedModel = v; }

  get qualityThresholdView(): number { return qualityThreshold; }
  set qualityThresholdView(v: number) { qualityThreshold = v; }

  get advancedOpenView(): boolean { return advancedOpen; }

  // ── Component-local state ──
  isLoading = false;
  stepLabel = '';
  errorMessage = '';
  refinementWarning = '';
  apiKeySet = true;
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

  readonly providerOptions = PROVIDER_OPTIONS;

  get currentModelOptions(): Array<{ label: string; value: string }> {
    return PROVIDER_MODELS[selectedProvider] ?? PROVIDER_MODELS['claude'];
  }

  readonly docTypeOptions = [
    { label: 'AUTO (detect)', value: 'auto' },
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
  ) {}

  async ngOnInit(): Promise<void> {
    sourceApp = await this.wails.getSourceApp();
    const settings = await this.wails.loadSettings();

    // Initialise provider from settings if not already set this session
    if (!selectedProvider && settings.active_provider) {
      selectedProvider = settings.active_provider;
      selectedModel = DEFAULT_MODELS[selectedProvider] ?? 'claude-sonnet-4-6';
    }

    const keyStatus = await this.wails.getKeyStatus(selectedProvider);
    this.apiKeySet = keyStatus.is_set;

    qualityThreshold = await this.wails.getQualityThreshold();

    this.cdr.detectChanges();

    this.sub = this.wails.shortcutDouble$.subscribe(async () => {
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
      this.detectedTypeView = '';
      activeTab = 'original';
      this.errorMessage = '';
      this.refinementWarning = '';

      if (originalText.trim()) {
        addTrace('Original', originalText);
      }
      this.cdr.detectChanges();
    });
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

  onProviderChange(): void {
    // Reset model to default for new provider
    selectedModel = DEFAULT_MODELS[selectedProvider] ?? '';
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
    if (!originalText.trim()) return;

    if (canvasText.trim()) {
      if (!confirm('Re-pyramidize? The current editor content will be saved to the trace log.')) {
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
        this.detectedTypeView = result.detectedType.toUpperCase();
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
      } finally {
        this.isLoading = false;
        this.cdr.detectChanges();
      }
    };

    try {
      await doCall();
    } catch (e: unknown) {
      if (!wasCancelled) {
        this.errorMessage = `Pyramidize failed: ${e instanceof Error ? e.message : String(e)}`;
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
        this.errorMessage = `Refine failed: ${e instanceof Error ? e.message : String(e)}`;
      }
    } finally {
      this.isLoading = false;
      this.cdr.detectChanges();
    }
  }

  onCanvasMouseUp(event: MouseEvent): void {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || !sel.toString().trim()) {
      this.showSelectionBubble = false;
      this.cdr.detectChanges();
      return;
    }

    const textarea = this.canvasTextareaRef?.nativeElement;
    if (textarea) {
      this.selectionStart = textarea.selectionStart;
      this.selectionEnd = textarea.selectionEnd;
    }

    this.showSelectionBubble = true;
    this.bubbleX = event.clientX - 100;
    this.bubbleY = event.clientY - 80;
    this.selectionInstruction = '';
    this.cdr.detectChanges();
  }

  closeSelectionBubble(): void {
    this.showSelectionBubble = false;
    this.selectionInstruction = '';
  }

  async applySelectionInstruction(): Promise<void> {
    if (!this.selectionInstruction.trim()) return;

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
        this.errorMessage = `Splice failed: ${e instanceof Error ? e.message : String(e)}`;
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
    this.peekEntry = entry;
    this.hoverEntry = null;
    this.cdr.detectChanges();
  }

  hoverTrace(entry: TraceEntry): void {
    if (!this.peekEntry) {
      this.hoverEntry = entry;
      this.cdr.detectChanges();
    }
  }

  clearHoverTrace(): void {
    this.hoverEntry = null;
    this.cdr.detectChanges();
  }

  closePeek(): void {
    this.peekEntry = null;
    this.hoverEntry = null;
  }

  revertTo(entry: TraceEntry): void {
    addTrace(`Reverted to: ${entry.label}`, canvasText);
    canvasText = entry.snapshot;
    this.peekEntry = null;
    activeTab = 'canvas';
    this.cdr.detectChanges();
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
    await this.svc.sendBack(canvasText);
  }

  async retry(): Promise<void> {
    if (this.lastRequest) {
      await this.lastRequest();
    }
  }

  formatTime(d: Date): string {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

  ngOnDestroy(): void {
    this.sub?.unsubscribe();
  }
}

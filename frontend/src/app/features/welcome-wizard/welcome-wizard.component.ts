import { Component, OnInit, ChangeDetectorRef } from '@angular/core';
import { Router } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { SelectModule } from 'primeng/select';
import { WailsService, ClaudeCodeStatus } from '../../core/wails.service';

@Component({
  selector: 'app-welcome-wizard',
  standalone: true,
  imports: [CommonModule, FormsModule, ButtonModule, InputTextModule, SelectModule],
  template: `
    <div class="wizard-wrapper">
      <div class="wizard-card">
        <h1>Welcome to <span class="brand">KeyLint</span></h1>
        <p class="subtitle">Let's get you set up in a few quick steps.</p>

        <!-- Step indicators -->
        <div class="wizard-steps">
          @for (s of stepDefs; track s.value) {
            <div class="wizard-step" [class.active]="step === s.value" [class.done]="step > s.value">
              <span class="step-circle">{{ step > s.value ? '✓' : s.value }}</span>
              <span class="step-label">{{ s.label }}</span>
            </div>
            @if (!$last) {
              <div class="step-line" [class.done]="step > s.value"></div>
            }
          }
        </div>

        <!-- Step content -->
        <div class="wizard-content">
          @switch (step) {
            @case (1) {
              <div data-testid="step-1-content">
                <p>KeyLint uses AI to fix grammar, spelling, and improve your writing — triggered by a global keyboard shortcut.</p>
                <div class="step-footer">
                  <p-button data-testid="wizard-next" label="Get Started" icon="pi pi-arrow-right" iconPos="right" (onClick)="step = 2" />
                </div>
              </div>
            }
            @case (2) {
              <div data-testid="step-2-content">
                @if (claudeCode) {
                  <button
                    type="button"
                    class="cli-option"
                    data-testid="wizard-claude-code"
                    [disabled]="!claudeCodeReady"
                    (click)="useClaudeCode()"
                  >
                    <span class="cli-option-title">Use Claude Code (installed CLI)</span>
                    <span class="cli-option-hint" data-testid="wizard-claude-code-hint">{{ claudeCodeHint }}</span>
                  </button>
                  <p class="or-divider">or use a provider with an API key:</p>
                } @else {
                  <p>Choose which AI provider to use for text enhancement:</p>
                }
                <p-select
                  [(ngModel)]="selectedProvider"
                  [options]="providers"
                  optionLabel="label"
                  optionValue="value"
                  placeholder="Select a provider"
                />
                <div class="step-footer">
                  <p-button data-testid="wizard-back" label="Back" severity="secondary" (onClick)="step = 1" />
                  <p-button data-testid="wizard-next" label="Next" icon="pi pi-arrow-right" iconPos="right" (onClick)="step = usesClaudeCode ? 4 : 3" [disabled]="!selectedProvider" />
                </div>
              </div>
            }
            @case (3) {
              <div data-testid="step-3-content">
                <p>Enter your API key for <strong>{{ providerLabel }}</strong>:</p>
                <input pInputText [(ngModel)]="apiKey" type="password" [placeholder]="apiKeyPlaceholder" style="width: 100%" />
                <div class="step-footer">
                  <p-button data-testid="wizard-back" label="Back" severity="secondary" (onClick)="step = 2" />
                  <p-button data-testid="wizard-next" label="Next" icon="pi pi-arrow-right" iconPos="right" (onClick)="step = 4" [disabled]="!apiKey && selectedProvider !== 'ollama'" />
                </div>
              </div>
            }
            @case (4) {
              <div data-testid="step-4-content">
                <p>You're all set! Press <kbd>Ctrl+G</kbd> anywhere to enhance selected text.</p>
                <div class="step-footer">
                  <p-button data-testid="wizard-back" label="Back" severity="secondary" (onClick)="step = usesClaudeCode ? 2 : 3" />
                  <p-button data-testid="wizard-finish" label="Start Using KeyLint" icon="pi pi-check" (onClick)="finish()" [loading]="finishing" />
                </div>
              </div>
            }
          }
        </div>
      </div>
    </div>
  `,
  styles: [`
    .wizard-wrapper {
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      background: var(--p-surface-950, #09090b);
    }
    .wizard-card {
      background: var(--p-surface-900, #18181b);
      border: 1px solid var(--p-surface-700, #3f3f46);
      border-radius: 12px;
      padding: 2.5rem;
      width: 520px;
      color: var(--p-surface-100, #f4f4f5);
    }
    h1 { margin: 0 0 0.5rem; }
    .brand { color: var(--p-primary-color, #f97316); }
    .subtitle { color: var(--p-surface-400, #a1a1aa); margin-bottom: 2rem; }

    .wizard-steps {
      display: flex;
      align-items: center;
      margin-bottom: 2rem;
    }
    .wizard-step {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 4px;
      flex-shrink: 0;
    }
    .step-circle {
      width: 32px;
      height: 32px;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 0.875rem;
      font-weight: 600;
      border: 2px solid var(--p-surface-600, #52525b);
      background: transparent;
      color: var(--p-surface-400, #a1a1aa);
      transition: all 0.2s;
    }
    .wizard-step.active .step-circle {
      border-color: var(--p-primary-color, #f97316);
      background: var(--p-primary-color, #f97316);
      color: #fff;
    }
    .wizard-step.done .step-circle {
      border-color: var(--p-primary-color, #f97316);
      background: var(--p-primary-color, #f97316);
      color: #fff;
    }
    .step-label {
      font-size: 0.75rem;
      color: var(--p-surface-400, #a1a1aa);
      white-space: nowrap;
    }
    .wizard-step.active .step-label,
    .wizard-step.done .step-label {
      color: var(--p-primary-color, #f97316);
    }
    .step-line {
      flex: 1;
      height: 2px;
      background: var(--p-surface-600, #52525b);
      margin: 0 8px;
      margin-bottom: 20px;
      transition: background 0.2s;
    }
    .step-line.done {
      background: var(--p-primary-color, #f97316);
    }

    .wizard-content {
      min-height: 140px;
    }
    .step-footer {
      display: flex;
      gap: 0.75rem;
      margin-top: 1.5rem;
      justify-content: flex-end;
    }
    .cli-option {
      display: flex;
      flex-direction: column;
      gap: 2px;
      width: 100%;
      text-align: left;
      padding: 0.75rem 1rem;
      margin-bottom: 1rem;
      border-radius: 8px;
      border: 1px solid var(--p-primary-color, #f97316);
      background: transparent;
      color: var(--p-surface-100, #f4f4f5);
      cursor: pointer;
    }
    .cli-option:disabled {
      border-color: var(--p-surface-600, #52525b);
      color: var(--p-surface-400, #a1a1aa);
      cursor: not-allowed;
    }
    .cli-option-title { font-weight: 600; }
    .cli-option-hint { font-size: 0.8rem; color: var(--p-surface-400, #a1a1aa); }
    .or-divider {
      font-size: 0.8rem;
      color: var(--p-surface-400, #a1a1aa);
      margin: 0 0 0.5rem;
    }
    kbd {
      background: var(--p-surface-800, #27272a);
      padding: 2px 6px;
      border-radius: 4px;
      font-family: monospace;
    }
  `],
})
export class WelcomeWizardComponent implements OnInit {
  step = 1;
  selectedProvider = 'openai';
  apiKey = '';
  finishing = false;
  /** Null until detection finishes; the CLI option only appears once known. */
  claudeCode: ClaudeCodeStatus | null = null;

  readonly stepDefs = [
    { value: 1, label: 'Welcome' },
    { value: 2, label: 'AI Provider' },
    { value: 3, label: 'API Key' },
    { value: 4, label: 'Done' },
  ];

  readonly providers = [
    { label: 'OpenAI (GPT)', value: 'openai' },
    { label: 'Anthropic Claude', value: 'claude' },
    { label: 'Ollama (local, free)', value: 'ollama' },
  ];

  constructor(
    private readonly wails: WailsService,
    private readonly router: Router,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  async ngOnInit(): Promise<void> {
    const isFirst = await this.wails.isFirstRun();
    if (!isFirst) {
      await this.router.navigate(['/']);
      return;
    }
    this.claudeCode = await this.wails.getClaudeCodeStatus();
    // The app is zoneless: without this the option never appears, because
    // nothing else triggers change detection after detection finishes.
    this.cdr.detectChanges();
  }

  /** True when the CLI can be used right now — installed and already signed in. */
  get claudeCodeReady(): boolean {
    return !!this.claudeCode?.installed && !!this.claudeCode?.loggedIn;
  }

  get usesClaudeCode(): boolean {
    return this.selectedProvider === 'claude-code';
  }

  get claudeCodeHint(): string {
    if (!this.claudeCode?.installed) {
      return 'Not found on this machine.';
    }
    if (!this.claudeCode.loggedIn) {
      return 'Installed but not signed in. Open a terminal, run `claude`, and sign in.';
    }
    return 'Signed in — uses your own subscription, no API key needed.';
  }

  /** One-click path: pick the CLI and skip the API key step entirely. */
  useClaudeCode(): void {
    if (!this.claudeCodeReady) return;
    this.selectedProvider = 'claude-code';
    this.apiKey = '';
    this.step = 4;
  }

  get providerLabel(): string {
    if (this.usesClaudeCode) return 'Claude Code (installed CLI)';
    return this.providers.find(p => p.value === this.selectedProvider)?.label ?? '';
  }

  get apiKeyPlaceholder(): string {
    switch (this.selectedProvider) {
      case 'openai': return 'sk-…';
      case 'claude': return 'sk-ant-…';
      case 'claude-code': return 'No key required — you are signed in to the CLI';
      default: return 'No key required for Ollama';
    }
  }

  async finish(): Promise<void> {
    this.finishing = true;
    try {
      const settings = await this.wails.loadSettings();
      settings.active_provider = this.selectedProvider;
      await this.wails.saveSettings(settings);
      // Store API key securely in the OS keyring (not in settings.json)
      if (this.apiKey && this.selectedProvider !== 'ollama' && this.selectedProvider !== 'claude-code') {
        await this.wails.setKey(this.selectedProvider, this.apiKey);
      }
      await this.wails.completeSetup();
      await this.router.navigate(['/']);
    } finally {
      this.finishing = false;
    }
  }
}

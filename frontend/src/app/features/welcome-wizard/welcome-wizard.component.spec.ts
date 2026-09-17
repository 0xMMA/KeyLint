import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { ComponentFixture } from '@angular/core/testing';
import { Router } from '@angular/router';
import { provideRouter } from '@angular/router';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { WelcomeWizardComponent } from './welcome-wizard.component';
import { WailsService, ClaudeCodeStatus } from '../../core/wails.service';
import { createWailsMock, defaultSettings, defaultClaudeCodeStatus } from '../../../testing/wails-mock';

describe('WelcomeWizardComponent', () => {
  let fixture: ComponentFixture<WelcomeWizardComponent>;
  let component: WelcomeWizardComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;
  let router: Router;

  beforeEach(async () => {
    wailsMock = createWailsMock();
    wailsMock.isFirstRun.mockResolvedValue(true);

    await TestBed.configureTestingModule({
      imports: [WelcomeWizardComponent],
      providers: [
        provideRouter([{ path: 'enhance', component: WelcomeWizardComponent }]),
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
      ],
    }).compileComponents();

    router = TestBed.inject(Router);
    fixture = TestBed.createComponent(WelcomeWizardComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
  });

  // Click the native button inside a p-button with a given data-testid.
  async function clickBtn(testid: string): Promise<void> {
    const btn = el.querySelector<HTMLButtonElement>(`[data-testid="${testid}"] button`);
    if (!btn) throw new Error(`Button [data-testid="${testid}"] button not found`);
    btn.click();
    fixture.detectChanges();
    await fixture.whenStable();
  }

  // Navigate from step 1 to the target step via button clicks.
  async function goToStep(target: number): Promise<void> {
    for (let s = 1; s < target; s++) {
      await clickBtn('wizard-next');
    }
  }

  // --- DOM tests ---

  it('shows step 1 content initially', () => {
    expect(el.querySelector('[data-testid="step-1-content"]')).toBeTruthy();
    expect(el.querySelector('[data-testid="step-2-content"]')).toBeFalsy();
    expect(el.querySelector('[data-testid="step-3-content"]')).toBeFalsy();
    expect(el.querySelector('[data-testid="step-4-content"]')).toBeFalsy();
  });

  it('shows step 2 content after clicking Get Started', async () => {
    await clickBtn('wizard-next');
    expect(el.querySelector('[data-testid="step-2-content"]')).toBeTruthy();
    expect(el.querySelector('[data-testid="step-1-content"]')).toBeFalsy();
  });

  it('shows step 3 content after clicking Next twice', async () => {
    await goToStep(3);
    expect(el.querySelector('[data-testid="step-3-content"]')).toBeTruthy();
    expect(el.querySelector('[data-testid="step-2-content"]')).toBeFalsy();
  });

  it('goes back to step 1 from step 2 via Back button', async () => {
    await clickBtn('wizard-next'); // → step 2
    await clickBtn('wizard-back'); // → step 1
    expect(el.querySelector('[data-testid="step-1-content"]')).toBeTruthy();
    expect(el.querySelector('[data-testid="step-2-content"]')).toBeFalsy();
  });

  it('shows step 4 content with a Finish button', async () => {
    await clickBtn('wizard-next'); // → step 2
    await clickBtn('wizard-next'); // → step 3
    // Set key so the step-3 Next button is enabled
    component.apiKey = 'sk-test';
    fixture.detectChanges();
    await fixture.whenStable();
    await clickBtn('wizard-next'); // → step 4
    expect(el.querySelector('[data-testid="step-4-content"]')).toBeTruthy();
    expect(el.querySelector('[data-testid="wizard-finish"]')).toBeTruthy();
  });

  it('step 3 Next button is disabled when API key is blank and provider is not ollama', async () => {
    await goToStep(3);
    // apiKey is '' and selectedProvider is 'openai' by default
    const btn = el.querySelector<HTMLButtonElement>('[data-testid="wizard-next"] button');
    expect(btn?.disabled).toBe(true);
  });

  it('step 3 Next button is enabled for ollama without API key', async () => {
    component.selectedProvider = 'ollama';
    fixture.detectChanges();
    await fixture.whenStable();
    await goToStep(3);
    const btn = el.querySelector<HTMLButtonElement>('[data-testid="wizard-next"] button');
    expect(btn?.disabled).toBe(false);
  });

  // --- Logic tests ---

  it('creates successfully', () => {
    expect(component).toBeTruthy();
  });

  it('starts on step 1', () => {
    expect(component.step).toBe(1);
  });

  it('redirects away when not first run', async () => {
    wailsMock.isFirstRun.mockResolvedValue(false);
    const navSpy = vi.spyOn(router, 'navigate').mockResolvedValue(true);
    await component.ngOnInit();
    expect(navSpy).toHaveBeenCalledWith(['/']);
  });

  it('does not redirect when first run', async () => {
    wailsMock.isFirstRun.mockResolvedValue(true);
    const navSpy = vi.spyOn(router, 'navigate').mockResolvedValue(true);
    await component.ngOnInit();
    expect(navSpy).not.toHaveBeenCalled();
  });

  it('returns correct providerLabel', () => {
    component.selectedProvider = 'claude';
    expect(component.providerLabel).toBe('Anthropic Claude');
  });

  it('returns correct apiKeyPlaceholder for ollama', () => {
    component.selectedProvider = 'ollama';
    expect(component.apiKeyPlaceholder).toBe('No key required for Ollama');
  });

  it('finish() saves settings, completes setup, and navigates', async () => {
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings });
    const navSpy = vi.spyOn(router, 'navigate').mockResolvedValue(true);
    component.selectedProvider = 'openai';
    component.apiKey = 'sk-abc123';
    await component.finish();
    expect(wailsMock.saveSettings).toHaveBeenCalled();
    expect(wailsMock.completeSetup).toHaveBeenCalled();
    expect(navSpy).toHaveBeenCalledWith(['/']);
  });

  it('finish() stores API key in keyring (not in settings)', async () => {
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings });
    vi.spyOn(router, 'navigate').mockResolvedValue(true);
    component.selectedProvider = 'openai';
    component.apiKey = 'sk-test-key';
    await component.finish();
    expect(wailsMock.setKey).toHaveBeenCalledWith('openai', 'sk-test-key');
    // key must NOT be in saved settings
    const saved = wailsMock.saveSettings.mock.calls[0][0];
    expect(saved.providers?.openai_key).toBeUndefined();
  });

  it('finish() resets finishing flag after completion', async () => {
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings });
    vi.spyOn(router, 'navigate').mockResolvedValue(true);
    await component.finish();
    expect(component.finishing).toBe(false);
  });
});

describe('WelcomeWizardComponent — Claude Code fast path', () => {
  let fixture: ComponentFixture<WelcomeWizardComponent>;
  let component: WelcomeWizardComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;

  // Renders the wizard on the provider step with a given detection result.
  async function renderStep2(status: Partial<ClaudeCodeStatus>): Promise<void> {
    wailsMock = createWailsMock();
    wailsMock.isFirstRun.mockResolvedValue(true);
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings });
    wailsMock.getClaudeCodeStatus.mockResolvedValue({ ...defaultClaudeCodeStatus, ...status });

    await TestBed.configureTestingModule({
      imports: [WelcomeWizardComponent],
      providers: [
        provideRouter([{ path: 'enhance', component: WelcomeWizardComponent }]),
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(WelcomeWizardComponent);
    component = fixture.componentInstance;
    // Pre-set async state: letting it land mid-render trips NG0100.
    component.claudeCode = { ...defaultClaudeCodeStatus, ...status };
    component.step = 2;
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
  }

  beforeEach(() => {
    TestBed.resetTestingModule();
  });

  function cliOption(): HTMLButtonElement {
    const btn = el.querySelector<HTMLButtonElement>('[data-testid="wizard-claude-code"]');
    if (!btn) throw new Error('Claude Code option not rendered');
    return btn;
  }

  function hint(): string {
    return el.querySelector('[data-testid="wizard-claude-code-hint"]')?.textContent?.trim() ?? '';
  }

  it('offers a signed-in CLI as a one-click option', async () => {
    await renderStep2({ installed: true, loggedIn: true, path: '/usr/local/bin/claude', version: '2.1.274' });

    expect(cliOption().disabled).toBe(false);
    expect(cliOption().textContent).toContain('Use Claude Code (installed CLI)');
    expect(hint()).toContain('no API key needed');
  });

  it('skips the API key step when the CLI is chosen', async () => {
    await renderStep2({ installed: true, loggedIn: true, path: '/usr/local/bin/claude' });

    cliOption().click();
    fixture.detectChanges();
    await fixture.whenStable();

    expect(el.querySelector('[data-testid="step-4-content"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="step-3-content"]')).toBeNull();
    expect(component.selectedProvider).toBe('claude-code');
  });

  it('saves the CLI as the provider without storing any key', async () => {
    await renderStep2({ installed: true, loggedIn: true, path: '/usr/local/bin/claude' });

    cliOption().click();
    fixture.detectChanges();
    await fixture.whenStable();

    await component.finish();

    expect(wailsMock.saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ active_provider: 'claude-code' }),
    );
    // The user signed in through Anthropic's own flow; KeyLint holds no credential.
    expect(wailsMock.setKey).not.toHaveBeenCalled();
    expect(wailsMock.completeSetup).toHaveBeenCalled();
  });

  it('disables the option when the CLI is installed but signed out', async () => {
    await renderStep2({ installed: true, loggedIn: false, path: '/usr/local/bin/claude' });

    expect(cliOption().disabled).toBe(true);
    expect(hint()).toContain('not signed in');
    expect(hint()).toContain('sign in');
  });

  it('disables the option when the CLI is not installed', async () => {
    await renderStep2({ installed: false });

    expect(cliOption().disabled).toBe(true);
    expect(hint()).toContain('Not found on this machine');
  });

  it('keeps the API key providers available alongside the CLI', async () => {
    await renderStep2({ installed: true, loggedIn: true });

    expect(el.querySelector('p-select')).not.toBeNull();
    expect(component.providers.map(p => p.value)).toContain('openai');
  });

  it('goes back to the provider step from the finish step on the CLI path', async () => {
    await renderStep2({ installed: true, loggedIn: true });

    cliOption().click();
    fixture.detectChanges();
    await fixture.whenStable();

    const back = el.querySelector<HTMLButtonElement>('[data-testid="wizard-back"] button');
    back!.click();
    fixture.detectChanges();
    await fixture.whenStable();

    expect(el.querySelector('[data-testid="step-2-content"]')).not.toBeNull();
  });
});

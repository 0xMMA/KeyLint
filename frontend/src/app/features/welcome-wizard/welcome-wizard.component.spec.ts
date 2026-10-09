import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { WelcomeWizardComponent } from './welcome-wizard.component';
import { WailsService, ClaudeCodeStatus, KeyStatus, Settings } from '../../core/wails.service';
import { createWailsMock, defaultSettings, defaultClaudeCodeStatus, WailsMock } from '../../../testing/wails-mock';

const SIGNED_IN: ClaudeCodeStatus = { installed: true, path: '/usr/local/bin/claude', version: '2.1.0', loggedIn: true };
const SIGNED_OUT: ClaudeCodeStatus = { ...SIGNED_IN, loggedIn: false };

interface Setup {
  cli?: ClaudeCodeStatus | Promise<ClaudeCodeStatus>;
  keys?: Partial<Record<string, KeyStatus>>;
  settings?: Partial<Settings>;
}

describe('WelcomeWizardComponent', () => {
  let fixture: ComponentFixture<WelcomeWizardComponent>;
  let el: HTMLElement;
  let wails: WailsMock;
  let router: Router;

  async function render(setup: Setup = {}): Promise<void> {
    wails = createWailsMock();
    wails.loadSettings.mockResolvedValue({ ...defaultSettings, ...setup.settings });
    wails.getKeyStatus.mockImplementation(async (p: string) => setup.keys?.[p] ?? { is_set: false, source: 'none' });
    if (setup.cli instanceof Promise) {
      wails.getClaudeCodeStatus.mockReturnValue(setup.cli);
    } else {
      wails.getClaudeCodeStatus.mockResolvedValue(setup.cli ?? { ...defaultClaudeCodeStatus });
    }

    TestBed.resetTestingModule();
    await TestBed.configureTestingModule({
      imports: [WelcomeWizardComponent],
      providers: [
        provideRouter([]),
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wails },
      ],
    }).compileComponents();

    router = TestBed.inject(Router);
    vi.spyOn(router, 'navigate').mockResolvedValue(true);
    fixture = TestBed.createComponent(WelcomeWizardComponent);
    el = fixture.nativeElement;
    await settle();
  }

  async function settle(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  const q = (testid: string) => el.querySelector<HTMLElement>(`[data-testid="${testid}"]`);

  async function click(testid: string): Promise<void> {
    const host = q(testid);
    if (!host) throw new Error(`[data-testid="${testid}"] not found`);
    (host.querySelector('button') ?? host).click();
    await settle();
  }

  function finishButton(): HTMLButtonElement {
    const btn = q('wizard-finish')?.querySelector('button');
    if (!btn) throw new Error('finish button not rendered');
    return btn;
  }

  function radio(provider: string): HTMLInputElement {
    return q(`wizard-radio-${provider}`) as HTMLInputElement;
  }

  async function pick(provider: string): Promise<void> {
    const input = radio(provider);
    input.checked = true;
    input.dispatchEvent(new Event('change'));
    await settle();
  }

  async function toProviders(): Promise<void> {
    await click('wizard-next');
  }

  describe('welcome screen', () => {
    beforeEach(async () => render());

    it('opens on what KeyLint does, with the shortcut as keycaps', () => {
      expect(q('wizard-welcome')).toBeTruthy();
      const caps = [...el.querySelectorAll('.keycap')].map(k => k.textContent?.trim());
      expect(caps).toEqual(['Ctrl', 'G']);
    });

    it('moves on to the provider choice', async () => {
      await toProviders();
      expect(q('wizard-provider')).toBeTruthy();
      expect(q('wizard-welcome')).toBeFalsy();
    });

    it('lets the user skip setup straight from the first screen', async () => {
      await click('wizard-skip');
      expect(wails.completeSetup).toHaveBeenCalled();
      expect(router.navigate).toHaveBeenCalledWith(['/']);
      expect(wails.setActiveProvider).not.toHaveBeenCalled();
    });
  });

  describe('provider choice', () => {
    it('offers the CLI as a normal option, preselected when signed in, with no key field', async () => {
      await render({ cli: SIGNED_IN });
      await toProviders();

      expect(radio('claude-code').checked).toBe(true);
      expect(q('wizard-status-claude-code')?.textContent).toContain('signed in');
      expect(q('wizard-key-input')).toBeFalsy();
      expect(finishButton().disabled).toBe(false);

      await click('wizard-finish');
      expect(wails.setActiveProvider).toHaveBeenCalledWith('claude-code');
      expect(wails.setKey).not.toHaveBeenCalled();
      expect(wails.completeSetup).toHaveBeenCalled();
      expect(router.navigate).toHaveBeenCalledWith(['/']);
    });

    it('shows the CLI option as checking while detection runs, then updates it', async () => {
      let release!: (s: ClaudeCodeStatus) => void;
      await render({ cli: new Promise(r => { release = r; }) });
      await toProviders();

      expect(q('wizard-option-claude-code')).toBeTruthy();
      expect(q('wizard-cli-checking')).toBeTruthy();
      expect(radio('claude-code').checked).toBe(false);

      release(SIGNED_IN);
      await settle();
      expect(q('wizard-cli-checking')).toBeFalsy();
      expect(radio('claude-code').checked).toBe(true);
    });

    it('does not move a selection the user made when detection lands later', async () => {
      let release!: (s: ClaudeCodeStatus) => void;
      await render({ cli: new Promise(r => { release = r; }) });
      await toProviders();
      await pick('ollama');

      release(SIGNED_IN);
      await settle();
      expect(radio('ollama').checked).toBe(true);
    });

    it('preselects nothing when nothing works — OpenAI is not assumed', async () => {
      await render();
      await toProviders();
      for (const p of ['openai', 'claude', 'claude-code', 'ollama']) {
        expect(radio(p).checked, p).toBe(false);
      }
      expect(finishButton().disabled).toBe(true);
      // ...and Set up later is still right there.
      expect(q('wizard-skip')?.querySelector('button')?.disabled).toBe(false);
    });

    it('preselects the saved provider when it works', async () => {
      await render({
        cli: SIGNED_IN,
        settings: { active_provider: 'claude' },
        keys: { claude: { is_set: true, source: 'keyring' } },
      });
      await toProviders();
      expect(radio('claude').checked).toBe(true);
    });

    it('preselects a provider with a stored key when the CLI is absent', async () => {
      await render({ keys: { claude: { is_set: true, source: 'keyring' } } });
      await toProviders();
      expect(radio('claude').checked).toBe(true);
    });

    it('says a stored key is there and lets the user through without typing it', async () => {
      await render({ keys: { claude: { is_set: true, source: 'keyring' } } });
      await toProviders();

      expect(q('wizard-key-stored')?.textContent).toContain('already saved');
      expect(q('wizard-key-input')).toBeFalsy();
      expect(finishButton().disabled).toBe(false);

      await click('wizard-finish');
      expect(wails.setActiveProvider).toHaveBeenCalledWith('claude');
      expect(wails.setKey).not.toHaveBeenCalled();
      expect(wails.completeSetup).toHaveBeenCalled();
    });

    it('names the environment variable when the key comes from there', async () => {
      await render({ keys: { openai: { is_set: true, source: 'env' } } });
      await toProviders();
      expect(q('wizard-key-stored')?.textContent).toContain('OPENAI_API_KEY');
      expect(q('wizard-replace-key')).toBeFalsy();
    });

    it('offers to replace a stored key', async () => {
      await render({ keys: { claude: { is_set: true, source: 'keyring' } } });
      await toProviders();
      await click('wizard-replace-key');
      expect(q('wizard-key-input')).toBeTruthy();
    });

    it('asks for a key when none is stored, and saves the typed one to the keyring', async () => {
      await render();
      await toProviders();
      await pick('openai');
      expect(finishButton().disabled).toBe(true);

      const input = q('wizard-key-input') as HTMLInputElement;
      input.value = 'sk-test';
      input.dispatchEvent(new Event('input'));
      await settle();
      expect(finishButton().disabled).toBe(false);

      wails.getKeyStatus.mockResolvedValue({ is_set: true, source: 'keyring' });
      await click('wizard-finish');
      expect(wails.setActiveProvider).toHaveBeenCalledWith('openai');
      expect(wails.setKey).toHaveBeenCalledWith('openai', 'sk-test');
      expect(wails.completeSetup).toHaveBeenCalled();
    });

    it('does not finish when the keyring refused the key, and says so', async () => {
      await render();
      await toProviders();
      await pick('openai');
      const input = q('wizard-key-input') as HTMLInputElement;
      input.value = 'sk-test';
      input.dispatchEvent(new Event('input'));
      await settle();

      await click('wizard-finish');
      await settle(); // several awaits deep: key, status
      expect(wails.completeSetup).not.toHaveBeenCalled();
      // Nothing moved: the provider without a working key was not made active.
      expect(wails.setActiveProvider).not.toHaveBeenCalled();
      expect(q('wizard-error')?.textContent).toContain('keyring');
    });

    it('surfaces a keyring that refused a replacement key', async () => {
      await render({ keys: { claude: { is_set: true, source: 'keyring' } } });
      await toProviders();
      await click('wizard-replace-key');
      const input = q('wizard-key-input') as HTMLInputElement;
      input.value = 'sk-ant-new';
      input.dispatchEvent(new Event('input'));
      await settle();

      // The old key is still there, so only the rejection can tell.
      wails.setKey.mockRejectedValue(new Error('keyring locked'));
      await click('wizard-finish');
      await settle();
      expect(q('wizard-error')?.textContent).toContain('keyring locked');
      expect(wails.setActiveProvider).not.toHaveBeenCalled();
      expect(wails.completeSetup).not.toHaveBeenCalled();
    });

    it('can go back to the saved key after choosing to replace it', async () => {
      await render({ keys: { claude: { is_set: true, source: 'keyring' } } });
      await toProviders();
      await click('wizard-replace-key');
      expect(finishButton().disabled).toBe(true);
      await click('wizard-keep-key');
      expect(q('wizard-key-stored')).toBeTruthy();
      expect(finishButton().disabled).toBe(false);
    });

    it('keeps the selection once the user starts replacing a key', async () => {
      let release!: (s: ClaudeCodeStatus) => void;
      await render({
        keys: { claude: { is_set: true, source: 'keyring' } },
        cli: new Promise(r => { release = r; }),
      });
      await toProviders();
      expect(radio('claude').checked).toBe(true);
      await click('wizard-replace-key');

      release(SIGNED_IN);
      await settle();
      expect(radio('claude').checked).toBe(true);
    });

    it('needs no key for Ollama', async () => {
      await render();
      await toProviders();
      await pick('ollama');
      expect(q('wizard-key-input')).toBeFalsy();
      expect(finishButton().disabled).toBe(false);
    });

    it('explains a signed-out CLI and checks again on request', async () => {
      await render({ cli: SIGNED_OUT });
      await toProviders();
      await pick('claude-code');
      expect(q('wizard-cli-hint')?.textContent).toContain('not signed in');

      wails.getClaudeCodeStatus.mockResolvedValue(SIGNED_IN);
      await click('wizard-recheck');
      expect(wails.getClaudeCodeStatus).toHaveBeenLastCalledWith(true);
      expect(q('wizard-cli-hint')).toBeFalsy();
      expect(q('wizard-status-claude-code')?.textContent).toContain('signed in');
    });

    it('finishes setup without a provider via Set up later', async () => {
      await render();
      await toProviders();
      await click('wizard-skip');
      expect(wails.completeSetup).toHaveBeenCalled();
      expect(wails.setActiveProvider).not.toHaveBeenCalled();
      expect(router.navigate).toHaveBeenCalledWith(['/']);
    });

    it('shows an error and stays when saving the choice fails', async () => {
      await render({ cli: SIGNED_IN });
      await toProviders();
      wails.setActiveProvider.mockRejectedValue(new Error('disk full'));
      await click('wizard-finish');
      expect(q('wizard-error')?.textContent).toContain('disk full');
      expect(wails.completeSetup).not.toHaveBeenCalled();
      expect(router.navigate).not.toHaveBeenCalled();
    });

    it('never saves a whole settings object', async () => {
      await render({ cli: SIGNED_IN });
      await toProviders();
      await click('wizard-finish');
      expect(wails.saveSettings).not.toHaveBeenCalled();
    });
  });
});

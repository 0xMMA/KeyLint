import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { AppPresetsComponent } from './app-presets.component';
import { WailsService, type AppPreset } from '../../../core/wails.service';
import { createWailsMock, defaultSettings } from '../../../../testing/wails-mock';

describe('AppPresetsComponent', () => {
  let fixture: ComponentFixture<AppPresetsComponent>;
  let el: HTMLElement;
  let wails: ReturnType<typeof createWailsMock>;
  /** What the backend holds; GetAppPresets answers from it. */
  let stored: AppPreset[];

  /** Lets the backend round trips (promise chains the fixture does not track) land, then repaints. */
  async function settle(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
    await new Promise(r => setTimeout(r));
    fixture.detectChanges();
  }

  async function render(presets: AppPreset[]): Promise<void> {
    stored = presets.map(p => ({ ...p }));
    wails = createWailsMock();
    wails.getAppPresets.mockImplementation(async () => stored.map(p => ({ ...p })));
    // As the Go service does: matched by name ignoring case, replaced in place.
    const same = (a: string, b: string) => a.toLowerCase() === b.toLowerCase();
    wails.setAppPreset.mockImplementation(async (p: AppPreset) => {
      const i = stored.findIndex(s => same(s.sourceApp, p.sourceApp));
      stored = i < 0 ? [...stored, { ...p }] : stored.map((s, j) => (j === i ? { ...p } : s));
    });
    wails.deleteAppPreset.mockImplementation(async (app: string) => {
      stored = stored.filter(s => !same(s.sourceApp, app));
    });
    await TestBed.configureTestingModule({
      imports: [AppPresetsComponent],
      providers: [{ provide: WailsService, useValue: wails }],
    }).compileComponents();
    fixture = TestBed.createComponent(AppPresetsComponent);
    fixture.componentRef.setInput('settings', { ...defaultSettings, app_presets: [] });
    el = fixture.nativeElement;
    await settle();
  }

  const q = (testid: string) => el.querySelector<HTMLElement>(`[data-testid="${testid}"]`);
  const click = async (host: Element | null, label?: string) => {
    const buttons = Array.from(host!.querySelectorAll<HTMLButtonElement>('button'));
    (label ? buttons.find(b => b.getAttribute('aria-label') === label) : buttons[0])!.click();
    await settle();
  };

  beforeEach(() => TestBed.resetTestingModule());
  afterEach(() => document.querySelector('.cdk-overlay-container')?.replaceChildren());

  // Nothing saves presets by itself: the text used to say apps were detected
  // automatically. It says what really happens with a name added here.
  it('says how presets come about when there are none', async () => {
    await render([]);
    const text = q('presets-empty')!.textContent!;
    expect(text).toContain('No app presets yet');
    expect(text).toContain('window title');
    expect(text).not.toMatch(/automatic/i);
    expect(q('preset-add-btn')).not.toBeNull();
  });

  it('lists the saved presets with their document type', async () => {
    await render([{ sourceApp: 'OUTLOOK.EXE', documentType: 'email' }]);
    expect(q('preset-OUTLOOK.EXE')?.textContent).toContain('OUTLOOK.EXE');
    expect(q('preset-OUTLOOK.EXE')?.textContent).toContain('Email');
    expect(q('presets-empty')).toBeNull();
  });

  it('adds one, and keeps the page\'s settings in step so Save cannot undo it', async () => {
    await render([]);
    await click(q('preset-add-btn'));
    const input = q('preset-add-row')!.querySelector('input')!;
    input.value = 'Slack';
    input.dispatchEvent(new Event('input'));
    await settle();
    await click(q('preset-add-row'), 'Add preset');

    expect(wails.setAppPreset).toHaveBeenCalledWith({ sourceApp: 'Slack', documentType: 'email' });
    expect(q('preset-Slack')).not.toBeNull();
    expect(fixture.componentInstance.settings()!.app_presets).toEqual([{ sourceApp: 'Slack', documentType: 'email' }]);
  });

  it('will not add a preset without an app name', async () => {
    await render([]);
    await click(q('preset-add-btn'));
    const add = Array.from(q('preset-add-row')!.querySelectorAll('button')).find(b => b.getAttribute('aria-label') === 'Add preset')!;
    expect(add.disabled).toBe(true);
  });

  it('edits and deletes a preset by name, each button saying which one', async () => {
    await render([{ sourceApp: 'Confluence', documentType: 'wiki' }]);
    await click(q('preset-Confluence'), 'Edit Confluence');
    const input = q('preset-edit-row')!.querySelector('input')!;
    expect(input.value).toBe('Confluence');
    await click(q('preset-edit-row'), 'Cancel');
    expect(q('preset-edit-row')).toBeNull();

    await click(q('preset-Confluence'), 'Delete Confluence');
    expect(wails.deleteAppPreset).toHaveBeenCalledWith('Confluence');
    expect(q('preset-Confluence')).toBeNull();
    expect(fixture.componentInstance.settings()!.app_presets).toEqual([]);
  });

  /** Opens a preset's edit row, types a name and saves. */
  async function rename(from: string, to: string): Promise<void> {
    await click(q(`preset-${from}`), `Edit ${from}`);
    const input = q('preset-edit-row')!.querySelector('input')!;
    input.value = to;
    input.dispatchEvent(new Event('input'));
    await settle();
    await click(q('preset-edit-row'), 'Save preset');
  }

  // Roadmap housekeeping: renaming added a second preset next to the old one.
  // Presets now steer Pyramidize's preselection, so the old name kept matching.
  it('renames a preset instead of adding a second one', async () => {
    await render([{ sourceApp: 'Outlok', documentType: 'email' }]);
    await rename('Outlok', 'Outlook');

    expect(stored).toEqual([{ sourceApp: 'Outlook', documentType: 'email' }]);
    expect(q('preset-Outlok')).toBeNull();
    expect(q('preset-Outlook')).not.toBeNull();
    expect(fixture.componentInstance.settings()!.app_presets).toEqual([{ sourceApp: 'Outlook', documentType: 'email' }]);
  });

  it('will not save a preset renamed to nothing', async () => {
    await render([{ sourceApp: 'Slack', documentType: 'chat' }]);
    await click(q('preset-Slack'), 'Edit Slack');
    const input = q('preset-edit-row')!.querySelector('input')!;
    input.value = '';
    input.dispatchEvent(new Event('input'));
    await settle();
    const save = Array.from(q('preset-edit-row')!.querySelectorAll('button')).find(b => b.getAttribute('aria-label') === 'Save preset')!;
    expect(save.disabled).toBe(true);
    expect(stored).toEqual([{ sourceApp: 'Slack', documentType: 'chat' }]);
  });

  it('keeps a preset whose name only changes case', async () => {
    await render([{ sourceApp: 'slack', documentType: 'chat' }]);
    await rename('slack', 'Slack');

    expect(stored).toEqual([{ sourceApp: 'Slack', documentType: 'chat' }]);
  });

  it('reloads when the page hands over new settings, as after a reset', async () => {
    await render([{ sourceApp: 'Confluence', documentType: 'wiki' }]);
    stored = [];
    fixture.componentRef.setInput('settings', { ...defaultSettings, app_presets: [] });
    await settle();
    expect(q('preset-Confluence')).toBeNull();
    expect(q('presets-empty')).not.toBeNull();
  });
});

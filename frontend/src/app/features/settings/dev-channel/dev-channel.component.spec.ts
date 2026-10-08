import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { DevChannelComponent } from './dev-channel.component';
import { WailsService, DevBuild, DevChannel } from '../../../core/wails.service';
import { createWailsMock, defaultDevChannel, defaultBuildIdentity } from '../../../../testing/wails-mock';

function build(over: Partial<DevBuild>): DevBuild {
  return {
    tag: 'v0.0.0-main',
    kind: 'main',
    pr: 0,
    title: '',
    pr_url: '',
    commit: '1111111',
    date: '2026-10-02T12:00:00Z',
    installable: true,
    installed: false,
    newer_build: false,
    ...over,
  };
}

const MAIN = build({});
const PR12 = build({ tag: 'v0.0.0-pr.12', kind: 'pr', pr: 12, title: 'Add dev channel', commit: 'abc1234', pr_url: 'https://github.com/0xMMA/KeyLint/pull/12' });
const PR7 = build({ tag: 'v0.0.0-pr.7', kind: 'pr', pr: 7, title: 'Older change', commit: '7777777' });

describe('DevChannelComponent', () => {
  let fixture: ComponentFixture<DevChannelComponent>;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;

  async function render(channel: Partial<DevChannel>, version = 'v4.4.3-beta'): Promise<void> {
    wailsMock = createWailsMock();
    wailsMock.listDevBuilds.mockResolvedValue({ ...defaultDevChannel, ...channel });
    await TestBed.configureTestingModule({
      imports: [DevChannelComponent],
      providers: [provideAnimationsAsync(), { provide: WailsService, useValue: wailsMock }],
    }).compileComponents();
    fixture = TestBed.createComponent(DevChannelComponent);
    fixture.componentRef.setInput('version', version);
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  const q = (id: string) => el.querySelector(`[data-testid="${id}"]`) as HTMLElement | null;
  const text = (id: string) => q(id)?.textContent?.replace(/\s+/g, ' ').trim() ?? '';
  const click = async (id: string) => {
    (q(id)?.querySelector('button') as HTMLButtonElement).click();
    await fixture.whenStable();
    fixture.detectChanges();
  };

  beforeEach(() => TestBed.resetTestingModule());

  it('lists main first, then every open PR build with title, date and commit', async () => {
    await render({ builds: [MAIN, PR12, PR7] });

    const rows = Array.from(el.querySelectorAll('[data-testid^="dev-build-v0"]'));
    expect(rows.map(r => r.getAttribute('data-testid'))).toEqual([
      'dev-build-v0.0.0-main', 'dev-build-v0.0.0-pr.12', 'dev-build-v0.0.0-pr.7',
    ]);
    expect(text('dev-build-v0.0.0-main')).toContain('main (latest)');
    const pr = text('dev-build-v0.0.0-pr.12');
    expect(pr).toContain('PR #12');
    expect(pr).toContain('Add dev channel');
    expect(pr).toContain('abc1234');
    expect(pr).toContain('2 Oct 2026');
    expect(q('install-v0.0.0-pr.12')).toBeTruthy();
  });

  it('says it is a release build when it is not from the dev channel', async () => {
    await render({ builds: [MAIN] });
    expect(text('dev-identity')).toBe('Running v4.4.3-beta, not a test build.');
    expect(q('dev-build-installed')).toBeNull();
  });

  it('marks the running build and names it', async () => {
    await render({
      current: { is_dev_build: true, kind: 'pr', pr: 12, commit: 'abc1234', tag: 'v0.0.0-pr.12' },
      builds: [MAIN, { ...PR12, installed: true }, PR7],
    }, '0.0.0-pr.12+abc1234');

    expect(text('dev-identity')).toBe('Running the test build of PR #12 (commit abc1234).');
    const row = q('dev-build-v0.0.0-pr.12')!;
    expect(row.querySelector('[data-testid="dev-build-installed"]')).toBeTruthy();
    expect(row.textContent).toContain('Reinstall');
    expect(q('dev-build-v0.0.0-main')!.querySelector('[data-testid="dev-build-installed"]')).toBeNull();
  });

  it('flags a newer build of the running PR', async () => {
    await render({
      current: { is_dev_build: true, kind: 'pr', pr: 12, commit: '0000000', tag: 'v0.0.0-pr.12' },
      builds: [MAIN, { ...PR12, newer_build: true }],
    });
    expect(q('dev-build-v0.0.0-pr.12')!.querySelector('[data-testid="dev-build-newer"]')).toBeTruthy();
  });

  it('installs the build whose button is clicked, through the backend', async () => {
    await render({ builds: [MAIN, PR12] });
    expect(wailsMock.installDevBuild).not.toHaveBeenCalled();

    await click('install-v0.0.0-pr.12');

    expect(wailsMock.installDevBuild).toHaveBeenCalledWith('v0.0.0-pr.12');
    expect(text('dev-install-success')).toContain('the app will close shortly');
  });

  it('shows an install failure instead of breaking', async () => {
    await render({ builds: [MAIN] });
    wailsMock.installDevBuild.mockRejectedValue(new Error('download returned status 404'));
    await click('install-v0.0.0-main');
    expect(text('dev-install-error')).toContain('download returned status 404');
  });

  it('disables Install for a build without an asset for this platform', async () => {
    await render({ builds: [{ ...PR7, installable: false }] });
    expect((q('install-v0.0.0-pr.7')!.querySelector('button') as HTMLButtonElement).disabled).toBe(true);
  });

  it('shows a rate limit as a message next to an empty list, not a broken screen', async () => {
    await render({ builds: [], error: "GitHub's limit for anonymous requests is used up — try again after 14:05" });
    expect(text('dev-channel-error')).toContain('try again after 14:05');
    expect(q('dev-build-empty')).toBeTruthy();
    expect(q('dev-channel-refresh')).toBeTruthy();
  });

  it('Refresh asks past the cache', async () => {
    await render({ builds: [MAIN] });
    await click('dev-channel-refresh');
    expect(wailsMock.listDevBuilds).toHaveBeenLastCalledWith(true);
  });

  describe('return offer', () => {
    const orphaned: Partial<DevChannel> = {
      current: { is_dev_build: true, kind: 'pr', pr: 99, commit: 'abc1234', tag: 'v0.0.0-pr.99' },
      orphaned: true,
      latest_release: '4.4.3-beta',
      builds: [MAIN, PR12],
    };

    it('offers main and the latest release when the PR build is gone, and installs nothing by itself', async () => {
      await render(orphaned);

      expect(text('return-offer')).toContain('Your test build for PR #99 is gone (merged or closed). Install the current main build?');
      expect(text('return-main-btn')).toBe('Install main build');
      expect(text('return-release-btn')).toBe('Install latest release (v4.4.3-beta)');
      expect(wailsMock.installDevBuild).not.toHaveBeenCalled();
      expect(wailsMock.installLatestRelease).not.toHaveBeenCalled();
    });

    it('installs main on one click', async () => {
      await render(orphaned);
      await click('return-main-btn');
      expect(wailsMock.installDevBuild).toHaveBeenCalledWith('v0.0.0-main');
      expect(wailsMock.installLatestRelease).not.toHaveBeenCalled();
    });

    it('installs the latest release on the other', async () => {
      await render(orphaned);
      await click('return-release-btn');
      expect(wailsMock.installLatestRelease).toHaveBeenCalledTimes(1);
      expect(wailsMock.installDevBuild).not.toHaveBeenCalled();
    });

    it('offers only the release when no main build is published', async () => {
      await render({ ...orphaned, builds: [PR12] });
      expect(q('return-main-btn')).toBeNull();
      expect(q('return-release-btn')).toBeTruthy();
    });

    it('stays away while the PR build still exists', async () => {
      await render({ ...orphaned, orphaned: false, current: { ...defaultBuildIdentity } });
      expect(q('return-offer')).toBeNull();
    });
  });
});

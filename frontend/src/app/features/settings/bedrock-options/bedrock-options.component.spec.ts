import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { BedrockOptionsComponent } from './bedrock-options.component';
import type { BedrockConnection } from '../../../core/bedrock';
import { bedrockModelList, bedrockRegions } from '../../../../testing/wails-mock';

// kl-select's options open in the CDK overlay container on <body>; spec files
// share one document, so it is emptied after each test.
afterEach(() => {
  document.querySelector('.cdk-overlay-container')?.replaceChildren();
});

const KEY_MODE: BedrockConnection = { region: '', auth: 'api-key', profile: '', routing: 'geo' };

describe('BedrockOptionsComponent', () => {
  let fixture: ComponentFixture<BedrockOptionsComponent>;
  let el: HTMLElement;
  let emitted: BedrockConnection[];

  async function render(connection: BedrockConnection, extra: Partial<{ fixModel: string; pyramidizeModel: string; envToken: boolean }> = {}) {
    fixture.componentRef.setInput('connection', connection);
    fixture.componentRef.setInput('regions', bedrockRegions);
    fixture.componentRef.setInput('knownModels', bedrockModelList.models);
    fixture.componentRef.setInput('fixModel', extra.fixModel ?? 'anthropic.claude-haiku-5-5');
    fixture.componentRef.setInput('pyramidizeModel', extra.pyramidizeModel ?? 'anthropic.claude-sonnet-5-5');
    fixture.componentRef.setInput('envToken', extra.envToken ?? false);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  const q = (testid: string) => el.querySelector<HTMLElement>(`[data-testid="${testid}"]`);
  const radio = (testid: string) => el.querySelector<HTMLInputElement>(`[data-testid="${testid}"]`)!;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [BedrockOptionsComponent] }).compileComponents();
    fixture = TestBed.createComponent(BedrockOptionsComponent);
    el = fixture.nativeElement;
    emitted = [];
    fixture.componentInstance.connectionChange.subscribe(c => emitted.push(c));
  });

  it('starts on a key, asks for a region, and says nothing about routing before one', async () => {
    await render(KEY_MODE);
    expect(radio('bedrock-auth-key').checked).toBe(true);
    expect(radio('bedrock-auth-signin').checked).toBe(false);
    expect(q('bedrock-profile')).toBeNull();
    expect(q('bedrock-region')!.textContent).toContain('Choose a region');
    expect(q('bedrock-routing')).toBeNull();
    expect(q('bedrock-worldwide-only')).toBeNull();
  });

  it("offers this computer's AWS sign-in, with an optional profile", async () => {
    await render(KEY_MODE);
    radio('bedrock-auth-signin').click();
    fixture.detectChanges();
    expect(emitted.at(-1)).toEqual({ ...KEY_MODE, auth: 'aws-signin' });
    // Shown straight away, before the parent has saved.
    expect(q('bedrock-profile')).not.toBeNull();
    expect(el.textContent).toContain('KeyLint stores nothing');
    expect(el.textContent).toContain('aws sso login');
  });

  it('saves a profile when the field is left, trimmed', async () => {
    await render({ ...KEY_MODE, auth: 'aws-signin' });
    const input = q('bedrock-profile') as HTMLInputElement;
    input.value = '  work  ';
    input.dispatchEvent(new Event('input'));
    input.dispatchEvent(new Event('blur'));
    fixture.detectChanges();
    expect(emitted.at(-1)).toEqual({ ...KEY_MODE, auth: 'aws-signin', profile: 'work' });
    // Leaving it again unchanged saves nothing more.
    input.dispatchEvent(new Event('blur'));
    expect(emitted.length).toBe(1);
  });

  it('picks a region from the list', async () => {
    await render(KEY_MODE);
    q('bedrock-region')!.querySelector<HTMLElement>('button')!.click();
    fixture.detectChanges();
    const frankfurt = Array.from(document.querySelectorAll<HTMLElement>('.cdk-overlay-container [role="option"]'))
      .find(o => o.textContent?.includes('eu-central-1'))!;
    expect(frankfurt.textContent).toContain('Europe (Frankfurt)');
    frankfurt.click();
    fixture.detectChanges();
    expect(emitted.at(-1)).toEqual({ ...KEY_MODE, region: 'eu-central-1' });
  });

  // The owner's decision (2026-10-15): routing is the user's choice, clearly
  // explained, with keeping it in the geography preselected.
  it('explains both routings, keeping the text in the geography preselected', async () => {
    await render({ ...KEY_MODE, region: 'eu-central-1' });
    const routing = q('bedrock-routing')!;
    expect(radio('bedrock-routing-geo').checked).toBe(true);
    expect(routing.textContent).toContain('Keep in the EU');
    expect(routing.textContent).toContain('only in its data centres in the EU');
    expect(routing.textContent).toContain('Worldwide');
    expect(routing.textContent).toContain('outside the EU too');
    expect(routing.textContent).toContain('About 10% cheaper');

    radio('bedrock-routing-global').click();
    fixture.detectChanges();
    expect(emitted.at(-1)).toEqual({ ...KEY_MODE, region: 'eu-central-1', routing: 'global' });
  });

  it('says plainly when a region offers only worldwide routing', async () => {
    await render({ ...KEY_MODE, region: 'ap-south-1' });
    expect(q('bedrock-routing')).toBeNull();
    const note = q('bedrock-worldwide-only')!;
    expect(note.textContent).toContain('Asia Pacific (Mumbai)');
    expect(note.textContent).toContain('only with worldwide routing');
    expect(note.textContent).toContain('any AWS region');
  });

  it('says which model in use cannot stay in the geography', async () => {
    await render({ ...KEY_MODE, region: 'ap-northeast-1' });
    const notes = Array.from(el.querySelectorAll('[data-testid="bedrock-cannot-stay"]')).map(n => n.textContent ?? '');
    expect(notes).toHaveLength(1);
    expect(notes[0]).toContain('Claude Sonnet 5.5');
    expect(notes[0]).toContain('Pyramidize');
    expect(notes[0]).toContain('Japan');

    // Worldwide: nothing to warn about.
    await render({ ...KEY_MODE, region: 'ap-northeast-1', routing: 'global' });
    expect(el.querySelector('[data-testid="bedrock-cannot-stay"]')).toBeNull();
    // An ID the user routed themselves is theirs.
    await render({ ...KEY_MODE, region: 'ap-northeast-1' }, { pyramidizeModel: 'global.anthropic.claude-sonnet-5-5' });
    expect(el.querySelector('[data-testid="bedrock-cannot-stay"]')).toBeNull();
  });

  it('says when AWS_BEARER_TOKEN_BEDROCK will be used instead of the sign-in', async () => {
    await render({ ...KEY_MODE, auth: 'aws-signin' }, { envToken: true });
    expect(q('bedrock-env-token')!.textContent).toContain('AWS_BEARER_TOKEN_BEDROCK');
    await render(KEY_MODE, { envToken: true });
    expect(q('bedrock-env-token')).toBeNull();
  });

  // A save that failed puts the old connection back: the radios follow it,
  // rather than keeping the click that did not stick.
  it('shows the saved connection again when the parent puts it back', async () => {
    await render({ ...KEY_MODE, region: 'eu-central-1' });
    radio('bedrock-routing-global').click();
    fixture.detectChanges();
    expect(radio('bedrock-routing-global').checked).toBe(true);

    await render({ ...KEY_MODE, region: 'eu-central-1' });
    expect(radio('bedrock-routing-geo').checked).toBe(true);
    expect(radio('bedrock-routing-global').checked).toBe(false);
  });
});

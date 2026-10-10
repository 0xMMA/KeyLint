import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { ComponentFixture } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideRouter } from '@angular/router';
import { DevToolsComponent } from './dev-tools.component';
import { WailsService } from '../../core/wails.service';
import { createWailsMock, defaultSettings } from '../../../testing/wails-mock';

describe('DevToolsComponent', () => {
  let fixture: ComponentFixture<DevToolsComponent>;
  let component: DevToolsComponent;
  let wailsMock: ReturnType<typeof createWailsMock>;

  beforeEach(async () => {
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings });

    await TestBed.configureTestingModule({
      imports: [DevToolsComponent],
      providers: [
        provideAnimationsAsync(),
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(DevToolsComponent);
    component = fixture.componentInstance;
  });

  it('creates successfully', () => {
    expect(component).toBeTruthy();
  });

  it('settings is null initially', () => {
    expect(component.settings).toBeNull();
  });

  it('simulate() calls simulateShortcut', async () => {
    await component.simulate();
    expect(wailsMock.simulateShortcut).toHaveBeenCalled();
  });

  it('simulate() resets simulating flag after completion', async () => {
    await component.simulate();
    expect(component.simulating).toBe(false);
  });

  it('loadSettings() populates settings property', async () => {
    await component.loadSettings();
    expect(component.settings).toMatchObject({ active_provider: 'openai' });
  });
});

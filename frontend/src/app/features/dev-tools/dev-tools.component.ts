import { ChangeDetectorRef, Component, ChangeDetectionStrategy } from '@angular/core';
import { JsonPipe } from '@angular/common';
import { RouterLink } from '@angular/router';
import { ButtonComponent, CardComponent } from '../../shared/ui';
import { WailsService, Settings as AppSettings } from '../../core/wails.service';

@Component({
  selector: 'app-dev-tools',
  standalone: true,
  imports: [JsonPipe, RouterLink, ButtonComponent, CardComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <div class="flex max-w-[760px] flex-col gap-6 px-11 py-9">
      <div>
        <h1 class="m-0 text-2xl font-bold tracking-tight text-fg-strong">Dev tools</h1>
        <p class="mt-2 mb-0 text-sm text-muted">This panel is only visible in development mode. <a class="text-link" routerLink="/dev-tools/ui" data-testid="ui-gallery-link">UI primitives</a></p>
      </div>

      <kl-card heading="Shortcut simulation" subheading="Fires a synthetic shortcut event — the same as pressing Ctrl+G on Windows.">
        <div>
          <kl-button data-testid="simulate-shortcut-btn" variant="primary" icon="zap" [loading]="simulating" (click)="simulate()">Simulate shortcut (Ctrl+G)</kl-button>
        </div>
      </kl-card>

      <kl-card heading="Current settings">
        @if (settings) {
          <pre class="m-0 max-h-[28rem] overflow-auto rounded-md bg-field p-4 text-xs text-soft">{{ settings | json }}</pre>
        } @else {
          <div>
            <kl-button data-testid="load-settings-btn" (click)="loadSettings()">Load settings</kl-button>
          </div>
        }
      </kl-card>
    </div>
  `,
})
export class DevToolsComponent {
  simulating = false;
  settings: AppSettings | null = null;

  constructor(
    private readonly wails: WailsService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  async simulate(): Promise<void> {
    this.simulating = true;
    try {
      await this.wails.simulateShortcut();
    } finally {
      this.simulating = false;
      this.cdr.markForCheck();
    }
  }

  async loadSettings(): Promise<void> {
    this.settings = await this.wails.loadSettings();
    this.cdr.markForCheck();
  }
}

import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { WailsService, Settings as AppSettings } from '../../core/wails.service';

@Component({
  selector: 'app-dev-tools',
  standalone: true,
  imports: [CommonModule, ButtonModule, CardModule],
  template: `
    <div class="dev-tools-container">
      <h2>Dev Tools</h2>
      <p class="note">This panel is only visible in development mode.</p>

      <p-card header="Shortcut Simulation">
        <p>Fires a synthetic shortcut event — same as pressing Ctrl+G on Windows.</p>
        <p-button
          label="Simulate Shortcut (Ctrl+G)"
          icon="pi pi-bolt"
          (onClick)="simulate()"
          [loading]="simulating"
        />
      </p-card>

      <p-card header="Current Settings" styleClass="mt-3">
        @if (settings) {
          <pre>{{ settings | json }}</pre>
        } @else {
          <p-button label="Load Settings" severity="secondary" (onClick)="loadSettings()" />
        }
      </p-card>
    </div>
  `,
  styles: [`
    .dev-tools-container {
      padding: 2rem;
      color: var(--p-surface-100, #f4f4f5);
      max-width: 700px;
    }
    h2 { margin: 0 0 0.5rem; color: var(--p-primary-color, #f97316); }
    .note { color: var(--p-surface-400, #a1a1aa); margin-bottom: 1.5rem; }
    pre {
      background: var(--p-surface-800, #27272a);
      padding: 1rem;
      border-radius: 6px;
      font-size: 0.8rem;
      overflow: auto;
    }
    .mt-3 { margin-top: 1rem; }
    code {
      background: var(--p-surface-800, #27272a);
      padding: 2px 5px;
      border-radius: 3px;
      font-family: monospace;
    }
  `],
})
export class DevToolsComponent {
  simulating = false;
  settings: AppSettings | null = null;

  constructor(private readonly wails: WailsService) {}

  async simulate(): Promise<void> {
    this.simulating = true;
    try {
      await this.wails.simulateShortcut();
    } finally {
      this.simulating = false;
    }
  }

  async loadSettings(): Promise<void> {
    this.settings = await this.wails.loadSettings();
  }
}

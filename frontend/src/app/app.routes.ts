import { Routes } from '@angular/router';
import { firstRunGuard } from './core/first-run.guard';

export const routes: Routes = [
  {
    path: 'welcome',
    loadComponent: () =>
      import('./features/welcome-wizard/welcome-wizard.component').then(m => m.WelcomeWizardComponent),
  },
  {
    path: '',
    canActivate: [firstRunGuard],
    loadComponent: () =>
      import('./layout/shell.component').then(m => m.ShellComponent),
    children: [
      {
        path: 'fix',
        loadComponent: () =>
          import('./features/fix/fix.component').then(m => m.FixComponent),
      },
      {
        path: 'enhance',
        loadComponent: () =>
          import('./features/text-enhancement/text-enhancement.component').then(m => m.TextEnhancementComponent),
      },
      {
        path: 'settings',
        loadComponent: () =>
          import('./features/settings/settings.component').then(m => m.SettingsComponent),
      },
      {
        // Every shared UI primitive in its states (E11), for review and screenshots.
        path: 'dev-tools/ui',
        loadComponent: () =>
          import('./features/dev-tools/ui-gallery/ui-gallery.component').then(m => m.UiGalleryComponent),
      },
      {
        path: 'dev-tools',
        loadComponent: () =>
          import('./features/dev-tools/dev-tools.component').then(m => m.DevToolsComponent),
      },
      { path: '', redirectTo: 'fix', pathMatch: 'full' },
    ],
  },
  { path: '**', redirectTo: '' },
];

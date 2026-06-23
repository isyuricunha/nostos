import { mount } from 'svelte';

import './styles.css';
import App from './App.svelte';

applyStoredAppearance();

const target = document.getElementById('app');

if (!target) {
  throw new Error('Application target element was not found.');
}

const app = mount(App, {
  target
});

export default app;

function applyStoredAppearance(): void {
  try {
    const raw = localStorage.getItem('nostos-appearance');
    if (!raw) return;
    const saved = JSON.parse(raw) as Partial<{
      patternEnabled: boolean;
      density: string;
      uiScale: number;
      accentColor: string;
      messageWidth: number;
      reducedMotion: boolean;
    }>;
    const root = document.documentElement;
    root.dataset.pattern = saved.patternEnabled === false ? 'off' : 'on';
    root.dataset.density = saved.density === 'comfortable' ? 'comfortable' : 'compact';
    root.dataset.reducedMotion = saved.reducedMotion ? 'on' : 'off';
    if (saved.uiScale) root.style.setProperty('--ui-scale', String(saved.uiScale / 100));
    if (saved.accentColor) {
      root.style.setProperty('--color-accent', saved.accentColor);
      root.style.setProperty('--color-accent-strong', saved.accentColor);
    }
    if (saved.messageWidth) root.style.setProperty('--chat-max-width', `${saved.messageWidth}px`);
  } catch {
    localStorage.removeItem('nostos-appearance');
  }
}

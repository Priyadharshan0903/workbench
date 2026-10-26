import { query } from './dom.js';
import { renderIcon } from './icons.js';
import { preferences, savePreferences } from './state.js';

export function applyTheme() {
  document.documentElement.dataset.theme = preferences.theme;
  query('#theme-toggle').innerHTML = renderIcon(preferences.theme === 'dark' ? 'sun' : 'moon');
  query('#theme-toggle').setAttribute(
    'aria-label',
    `Switch to ${preferences.theme === 'dark' ? 'light' : 'dark'} mode`,
  );
  query('meta[name="theme-color"]').content = preferences.theme === 'dark' ? '#0c0c0c' : '#f7f7f5';
}

export function setSidebar(isOpen) {
  const mobile = window.matchMedia?.('(max-width: 1000px)')?.matches || false;
  query('#sidebar').classList.toggle('mobile-open', isOpen);
  query('#sidebar').inert = mobile && !isOpen;
  query('#menu-toggle').setAttribute('aria-expanded', String(isOpen));
}

export function toggleTheme() {
  preferences.theme = preferences.theme === 'light' ? 'dark' : 'light';
  applyTheme();
  savePreferences();
}

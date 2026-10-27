import { showNotice } from './dom.js';

export const storedPreferences = readStoredPreferences();

export const preferences = {
  layout: storedPreferences.layout === 'list' ? 'list' : 'board',
  theme:
    storedPreferences.themeVersion === 3 && ['light', 'dark'].includes(storedPreferences.theme)
      ? storedPreferences.theme
      : 'dark',
  themeVersion: 3,
  favorites: Array.isArray(storedPreferences.favorites)
    ? storedPreferences.favorites.filter((entry) => typeof entry === 'string').slice(0, 500)
    : [],
  order: Array.isArray(storedPreferences.order)
    ? storedPreferences.order.filter((entry) => typeof entry === 'string').slice(0, 500)
    : [],
  recent:
    storedPreferences.recent &&
    typeof storedPreferences.recent === 'object' &&
    !Array.isArray(storedPreferences.recent)
      ? storedPreferences.recent
      : {},
};

export const workspaceState = {
  snapshot: null,
  skills: null,
  activeTab: 'tools',
  projectFilter: 'all',
  isRefreshing: false,
  stopTarget: null,
  launchPlan: null,
  editorRevision: null,
  candidates: [],
  detailId: null,
  detailMode: 'overview',
  launchRequestSequence: 0,
  pointerDrag: null,
  lastRefreshAt: 0,
  pendingLaunches: new Map(),
  projectIdEdited: false,
  setupLaunchProjectId: null,
};

export function readStoredPreferences() {
  try {
    const stored = JSON.parse(localStorage.getItem('workbench.workspace.v2') || '{}');
    if (stored && typeof stored === 'object' && !Array.isArray(stored)) {
      return stored;
    }
  } catch {
    // Missing or corrupt browser storage falls back to default workspace preferences.
  }

  return {};
}

export function savePreferences() {
  try {
    localStorage.setItem('workbench.workspace.v2', JSON.stringify(preferences));
  } catch {
    showNotice(
      'Your browser could not save workspace preferences. Project records are still saved on disk.',
    );
  }
}

import { bindArrangementEvents } from './js/projects/arrangement.js';
import { renderTools } from './js/projects/board.js';
import { renderSummary } from './js/projects/summary.js';
import { requestJSON } from './js/shared/api.js';
import { applyTheme, setSidebar } from './js/shared/appearance.js';
import { query, showNotice } from './js/shared/dom.js';
import { bindDialogEvents } from './js/shared/dialogs.js';
import { paintIcons } from './js/shared/icons.js';
import { savePreferences, storedPreferences, workspaceState } from './js/shared/state.js';
import { bindSkillEvents, loadSkills } from './js/skills/catalog.js';
import { redirectLegacyEngine } from './js/workspace/engine.js';
import { bindLogEvents, refreshLogs } from './js/workspace/logs.js';
import { createProjectEditor } from './js/projects/editor.js';
import { createLaunchController } from './js/workspace/launch.js';
import { createServiceController } from './js/workspace/services.js';
import { createNavigation } from './js/workspace/navigation.js';
import { createDashboardEvents } from './js/workspace/events.js';

const launch = createLaunchController({ refresh });
const projectEditor = createProjectEditor({ refresh, prepareLaunch: launch.prepareLaunch });
const services = createServiceController({ refresh });
const navigation = createNavigation({ refresh });
const events = createDashboardEvents({ projectEditor, launch, services });
const { renderServices } = services;
const { settlePending } = launch;

async function refresh(forceSkills = false) {
  if (workspaceState.isRefreshing) return;
  workspaceState.isRefreshing = true;
  query('#refresh').disabled = true;
  try {
    workspaceState.snapshot = await requestJSON('/api/state');
    workspaceState.snapshot.controlWarnings ||= [];
    for (const service of workspaceState.snapshot.services) {
      if (!service.canStop && service.projectId && !service.self && !service.stopUnavailable)
        service.stopUnavailable = 'Process identity is unavailable.';
    }
    renderSummary();
    renderTools();
    renderServices();
    settlePending();
    if (!workspaceState.snapshot.features?.launch)
      showNotice(
        'The workspace UI is updated. Reopen Workbench with its launcher once to enable project starts and discovery.',
      );
    else if (query('#notice').textContent.startsWith('Could not refresh the local engine.'))
      showNotice('');
    if (forceSkills && workspaceState.activeTab === 'skills') await loadSkills(true);
    if (query('#details').open && workspaceState.detailMode === 'logs')
      refreshLogs(workspaceState.detailId);
  } catch (error) {
    showNotice('Could not refresh the local engine. ' + error.message);
    query('#engine-label').textContent = 'Engine unavailable';
    query('#engine-dot').className = 'status-dot warning';
  } finally {
    workspaceState.isRefreshing = false;
    query('#refresh').disabled = false;
  }
}

function refreshWhenVisible() {
  if (!document.hidden) refresh();
}

function pollWorkspace() {
  if (document.hidden) return;

  const hasStartingProject = workspaceState.snapshot?.projects.some(
    (project) => project.status === 'starting',
  );
  const interval = hasStartingProject || workspaceState.pendingLaunches.size ? 3000 : 10000;

  if (Date.now() - workspaceState.lastRefreshAt >= interval) {
    workspaceState.lastRefreshAt = Date.now();
    refresh();
  }
}

function initializeWorkspace() {
  paintIcons();
  applyTheme();
  setSidebar(false);

  window.matchMedia?.('(max-width: 1000px)')?.addEventListener('change', () => setSidebar(false));
  if (storedPreferences.themeVersion !== 3) savePreferences();

  query('#today').textContent = new Date().toLocaleDateString(undefined, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  });

  navigation.bindNavigationEvents();
  bindSkillEvents();
  bindDialogEvents();
  events.bindProjectEvents();
  bindArrangementEvents();
  bindLogEvents();
  navigation.setTab();

  redirectLegacyEngine().then((forwarded) => {
    if (!forwarded) refresh();
  });

  workspaceState.lastRefreshAt = Date.now();
  setInterval(pollWorkspace, 1000);
  document.addEventListener('visibilitychange', refreshWhenVisible);
}

initializeWorkspace();

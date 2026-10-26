import { renderTools } from '../projects/board.js';
import { showProject } from '../projects/details.js';
import { isFavorite } from '../projects/model.js';
import { recordOpen, renderSummary } from '../projects/summary.js';
import { copyText, query, queryAll } from '../shared/dom.js';
import { preferences, savePreferences, workspaceState } from '../shared/state.js';
import { keepSkillOpenIfDirty, renderSkills, showSkill } from '../skills/catalog.js';
import { chooseDiscoveryFolder, discoverProjectsFromForm, openDiscover } from './discovery.js';
import { refreshLogs, showLogs } from './logs.js';

export function createDashboardEvents({ projectEditor, launch, services }) {
  const {
    openProjectEditor,
    trackServiceAsProject,
    saveProjectFromForm,
    updateSuggestedProjectId,
  } = projectEditor;
  const { prepareLaunch, launchReviewedProject } = launch;
  const { showStop, stopReviewedService, renderServices } = services;
  function handleWorkspaceClick(event) {
    const button = event.target.closest('button,a');
    if (!button) return;
    if (button.dataset.close) {
      if (button.dataset.close === 'details' && keepSkillOpenIfDirty()) return;
      query('#' + button.dataset.close).close();
      return;
    }
    if (button.dataset.favorite) {
      const id = button.dataset.favorite;
      preferences.favorites = isFavorite(id)
        ? preferences.favorites.filter((entry) => entry !== id)
        : [...preferences.favorites, id];
      savePreferences();
      renderTools();
      renderSummary();
      return;
    }
    if (button.dataset.open) {
      recordOpen(button.dataset.open);
      return;
    }
    if (button.dataset.setupStart) {
      openProjectEditor(button.dataset.setupStart, null, true);
      return;
    }
    if (button.dataset.start) {
      prepareLaunch(button.dataset.start);
      return;
    }
    if (button.dataset.logs) {
      showLogs(button.dataset.logs);
      return;
    }
    if (button.dataset.refreshLogs) {
      refreshLogs(button.dataset.refreshLogs);
      return;
    }
    if (button.hasAttribute('data-add-project')) {
      openProjectEditor();
      return;
    }
    if (button.hasAttribute('data-reset-filters')) {
      query('#tool-search').value = '';
      query('#tool-status').value = 'all';
      workspaceState.projectFilter = 'all';
      location.hash = 'tools';
      renderTools();
      return;
    }
    if (button.dataset.project) {
      showProject(button.dataset.project);
      return;
    }
    if (button.dataset.edit) {
      openProjectEditor(button.dataset.edit);
      return;
    }
    if (button.dataset.skill) {
      showSkill(button.dataset.skill);
      return;
    }
    if (button.dataset.stop) {
      showStop(Number(button.dataset.stop));
      return;
    }
    if (button.dataset.track) {
      trackServiceAsProject(Number(button.dataset.track));
      return;
    }
    if (button.dataset.copy) {
      copyText(button.dataset.copy, button);
      return;
    }
    if (button.hasAttribute('data-discover')) {
      openDiscover();
      return;
    }
    if (button.dataset.root) {
      query('#discover-path').value = button.dataset.root;
      return;
    }
    if (button.dataset.import !== undefined) {
      const candidate = workspaceState.candidates[Number(button.dataset.import)];
      if (candidate) openProjectEditor(null, candidate.project);
    }
  }

  function bindProjectEvents() {
    document.addEventListener('click', handleWorkspaceClick);
    query('#add-tool').addEventListener('click', () => openProjectEditor());
    query('#discover-button').addEventListener('click', openDiscover);
    query('#tool-form').addEventListener('submit', saveProjectFromForm);
    query('#discover-form').addEventListener('submit', discoverProjectsFromForm);
    query('#choose-folder').addEventListener('click', chooseDiscoveryFolder);
    query('#confirm-launch').addEventListener('click', launchReviewedProject);
    query('#confirm-stop').addEventListener('click', stopReviewedService);

    query('#tool-search').addEventListener('input', renderTools);
    query('#tool-status').addEventListener('change', renderTools);
    query('#service-search').addEventListener('input', renderServices);
    query('#service-filter').addEventListener('change', renderServices);
    query('#skill-search').addEventListener('input', renderSkills);
    query('#skill-provider').addEventListener('change', renderSkills);
    query('#skill-scope').addEventListener('change', renderSkills);

    queryAll('[data-layout]').forEach((button) => {
      button.addEventListener('click', () => {
        preferences.layout = button.dataset.layout;
        savePreferences();
        renderTools();
      });
    });

    queryAll('[data-filter]').forEach((button) => {
      button.addEventListener('click', () => {
        workspaceState.projectFilter = button.dataset.filter;
        renderTools();
      });
    });

    query('#attention-filter').addEventListener('click', () => {
      if (!['tools', 'favorites', 'recent'].includes(workspaceState.activeTab))
        location.hash = 'tools';
      workspaceState.projectFilter = 'attention';
      renderTools();
    });

    const projectForm = query('#tool-form');
    projectForm.elements.namedItem('name').addEventListener('input', updateSuggestedProjectId);
    projectForm.elements.namedItem('id').addEventListener('input', () => {
      workspaceState.projectIdEdited = true;
    });
  }
  return { bindProjectEvents };
}

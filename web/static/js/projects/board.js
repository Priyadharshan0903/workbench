import { renderProjectCard } from './cards.js';
import { isFavorite, orderedProjects, workspaceProjects } from './model.js';
import { renderResume } from './summary.js';
import { query, queryAll } from '../shared/dom.js';
import { renderIcon } from '../shared/icons.js';
import { isProjectReady, needsAttention } from '../shared/project.js';
import { preferences, savePreferences, workspaceState } from '../shared/state.js';

export function renderTools() {
  if (!workspaceState.snapshot) return;
  const search = query('#tool-search').value.trim().toLowerCase();
  const lifecycle = query('#tool-status').value;
  const scopedProjects = orderedProjects().filter(
    (project) =>
      (lifecycle === 'all' ||
        (lifecycle === 'current'
          ? project.lifecycle !== 'archived'
          : project.lifecycle === lifecycle)) &&
      (workspaceState.activeTab !== 'favorites' || isFavorite(project.id)) &&
      (workspaceState.activeTab !== 'recent' || Number.isFinite(preferences.recent[project.id])) &&
      [project.name, project.description, project.path, project.owner, ...project.ports]
        .join(' ')
        .toLowerCase()
        .includes(search),
  );
  const projects = scopedProjects.filter(
    (project) =>
      workspaceState.projectFilter === 'all' ||
      (workspaceState.projectFilter === 'attention'
        ? needsAttention(project)
        : project.status === workspaceState.projectFilter),
  );
  query('#tools-count').textContent = scopedProjects.length;
  query('#ready-count').textContent = scopedProjects.filter(isProjectReady).length;
  query('#stopped-count').textContent = scopedProjects.filter(
    (project) => project.status === 'stopped',
  ).length;
  query('#attention-count').textContent = scopedProjects.filter(needsAttention).length;
  if (workspaceState.activeTab === 'recent')
    projects.sort(
      (firstProject, secondProject) =>
        preferences.recent[secondProject.id] - preferences.recent[firstProject.id],
    );
  queryAll('.project-tab').forEach((element) => {
    const selected = element.dataset.filter === workspaceState.projectFilter;
    element.classList.toggle('active', selected);
    element.setAttribute('aria-selected', selected);
    element.tabIndex = selected ? 0 : -1;
  });
  const list = query('#tool-list');
  list.classList.toggle('list-layout', preferences.layout === 'list');
  queryAll('[data-layout]').forEach((element) => {
    const selected = element.dataset.layout === preferences.layout;
    element.classList.toggle('selected', selected);
    element.setAttribute('aria-pressed', selected);
  });
  query('#arrange-hint').hidden =
    preferences.layout === 'list' || workspaceState.activeTab === 'recent' || projects.length < 2;
  list.dataset.count = projects.length;
  list.dataset.remainder = projects.length % 3;
  const total = workspaceProjects().length;
  const count = scopedProjects.length;
  query('#tools-view').classList.toggle(
    'is-first-run',
    total === 0 && workspaceState.activeTab === 'tools',
  );
  query('#visible-count').textContent =
    projects.length === count
      ? `${count} project${count === 1 ? '' : 's'}`
      : `${projects.length} of ${count} projects`;
  list.innerHTML = projects.length
    ? projects.map(renderProjectCard).join('')
    : emptyWorkspace(search, total);

  renderResume();
}

export function emptyWorkspace(search, total) {
  const firstRun = total === 0 && workspaceState.activeTab === 'tools' && !search;
  if (firstRun)
    return `<div class="empty first-run">
  <div class="first-run-copy">
    <span class="empty-label">A place for your projects</span>
    <h2>Add your first project.</h2>
    <p>
      Bring in a project folder. Workbench keeps its launch command, local address and service
      status together.
    </p>
    <div class="empty-actions">
      <button class="button" data-discover>${renderIcon('folder')}Choose a folder</button>
      <button class="text-button" data-add-project>Add manually ${renderIcon('arrow')}</button>
    </div>
  </div>
  <div class="first-run-steps">
    <div>
      <strong>Find your project</strong>
      <span>Browse a folder or paste its path.</span>
    </div>
    <div>
      <strong>Review the launch</strong>
      <span>Check the command and local port.</span>
    </div>
    <div>
      <strong>Pick up your work</strong>
      <span>Start it here, then open when ready.</span>
    </div>
  </div>
</div>`;
  const title =
    workspaceState.activeTab === 'favorites' && !search
      ? 'Your go-to projects belong here.'
      : workspaceState.activeTab === 'recent' && !search
        ? 'Pick up where you left off.'
        : 'No projects match.';
  const message =
    workspaceState.activeTab === 'favorites' && !search
      ? 'Star a project to keep it within reach.'
      : workspaceState.activeTab === 'recent' && !search
        ? 'Projects you open from Workbench will appear here.'
        : 'Try another search or clear the current filters.';
  return `<div class="empty">
  <h2>${title}</h2>
  <p>${message}</p>
  <button class="button" data-reset-filters>View all projects ${renderIcon('arrow')}</button>
</div>`;
}

export function reorderProject(id, target) {
  const order = orderedProjects().map((project) => project.id);
  const from = order.indexOf(id);
  const to = order.indexOf(target);
  if (from < 0 || to < 0 || from === to) return;
  order.splice(from, 1);
  order.splice(to, 0, id);
  preferences.order = order;
  savePreferences();
  renderTools();
  query('#announcement').textContent = `Project moved to position ${to + 1} of ${order.length}.`;
}

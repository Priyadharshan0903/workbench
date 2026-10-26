import { renderResumeItem } from './cards.js';
import { isFavorite, workspaceProjects } from './model.js';
import { query } from '../shared/dom.js';
import { isProjectReady, needsAttention } from '../shared/project.js';
import { preferences, savePreferences, workspaceState } from '../shared/state.js';

export function recordOpen(id) {
  preferences.recent[id] = Date.now();
  savePreferences();
  renderResume();
}

export function renderSummary() {
  if (!workspaceState.snapshot) return;
  const { services, inspectionError } = workspaceState.snapshot;
  const projects = workspaceProjects();
  const readyCount = projects.filter(isProjectReady).length;
  const stoppedCount = projects.filter((project) => project.status === 'stopped').length;
  const attentionCount = projects.filter(needsAttention).length;
  query('#stat-tools').textContent = projects.length;
  query('#stat-running').textContent = inspectionError ? '—' : readyCount;
  query('#stat-stopped').textContent = inspectionError ? '—' : stoppedCount;
  query('#stat-attention').textContent = inspectionError ? '—' : attentionCount;
  query('#nav-tools').textContent = projects.length;
  query('#nav-favorites').textContent = projects.filter((project) => isFavorite(project.id)).length;
  query('#nav-services').textContent = inspectionError
    ? '—'
    : services.filter((service) => service.projectId).length;
  if (workspaceState.skills) query('#nav-skills').textContent = workspaceState.skills.skills.length;
  query('#tools-count').textContent = projects.length;
  query('#ready-count').textContent = readyCount;
  query('#stopped-count').textContent = stoppedCount;
  query('#attention-count').textContent = attentionCount;
  const checkedAt = new Date(workspaceState.snapshot.scannedAt).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
  const refreshSeconds = projects.some((project) => project.status === 'starting') ? 3 : 10;
  query('#last-sync').textContent = `Checked ${checkedAt} · checks every ${refreshSeconds} seconds`;
  query('#engine-label').textContent = inspectionError
    ? 'Inspection unavailable'
    : 'Engine connected';
  query('#engine-address').textContent = location.port ? ':' + location.port : '';
  query('#engine-details').disabled = !workspaceState.snapshot.projects.some(
    (project) => project.id === 'workbench',
  );
  query('#engine-dot').className = 'status-dot ' + (inspectionError ? 'warning' : 'ready');
}

export function renderResume() {
  const section = query('#resume-section');
  if (!workspaceState.snapshot || !['tools'].includes(workspaceState.activeTab)) {
    section.hidden = true;
    return;
  }
  const items = workspaceProjects()
    .filter((project) => Number.isFinite(preferences.recent[project.id]))
    .sort(
      (firstProject, secondProject) =>
        preferences.recent[secondProject.id] - preferences.recent[firstProject.id],
    )
    .slice(0, 3);
  section.hidden = !items.length;
  query('#resume-list').innerHTML = items.map(renderResumeItem).join('');
}

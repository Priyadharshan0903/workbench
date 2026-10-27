import { isFavorite } from './model.js';
import { escapeHTML, renderStatusBadge, shortenPath } from '../shared/dom.js';
import { renderIcon } from '../shared/icons.js';
import { kindLabels, needsAttention, statusLabels } from '../shared/project.js';
import { workspaceState } from '../shared/state.js';

export function renderProjectAction(project) {
  const id = escapeHTML(project.id);
  if (['ready', 'running'].includes(project.status) && project.url)
    return `<a
  class="button project-primary"
  data-open="${id}"
  href="${escapeHTML(project.url)}"
  target="_blank"
  rel="noopener noreferrer"
>
  Open ${renderIcon('external')}
</a>`;
  if (project.status === 'starting')
    return `<button class="button project-primary" data-logs="${id}">
  <span class="mini-loader"></span>
  Starting…
</button>`;
  if (
    ['available', 'stopped', 'failed'].includes(project.status) &&
    project.kind === 'web' &&
    project.lifecycle === 'active' &&
    project.pathExists &&
    project.launch?.trim() &&
    project.canLaunch &&
    workspaceState.snapshot?.features?.launch &&
    (project.launchSetupRequired ?? (!project.url || !project.ports.length))
  )
    return `<button class="button project-primary" data-setup-start="${id}">
  ${renderIcon('play')}Set up & start
</button>`;
  if (
    ['stopped', 'failed'].includes(project.status) &&
    project.lifecycle === 'active' &&
    workspaceState.snapshot?.features?.launch &&
    project.canLaunch
  )
    return `<button class="button project-primary" data-start="${id}">
  ${renderIcon('play')}Start
</button>`;
  return `<button class="button project-primary secondary" data-project="${id}">
  ${project.status === 'stopped' ? 'How to start' : needsAttention(project) ? 'Inspect' : 'How to open'}${renderIcon('arrow')}
</button>`;
}

export function renderResumeItem(project) {
  const projectId = escapeHTML(project.id);
  const canOpen = ['ready', 'running'].includes(project.status) && project.url;
  const secondaryAction = canOpen
    ? `<a data-open="${projectId}" href="${escapeHTML(project.url)}" target="_blank"
          rel="noopener noreferrer" aria-label="Open ${escapeHTML(project.name)}">${renderIcon('arrow')}</a>`
    : `<button data-project="${projectId}" aria-label="Inspect ${escapeHTML(project.name)}">${renderIcon('arrow')}</button>`;

  return `
    <div class="resume-item">
      <button data-project="${projectId}">
        <span class="status-dot ${escapeHTML(project.status)}"></span>
        <strong>${escapeHTML(project.name)}</strong>
        <small>${escapeHTML(statusLabels[project.status])}</small>
      </button>
      ${secondaryAction}
    </div>
  `;
}

export function renderProjectCard(project) {
  const projectId = escapeHTML(project.id);
  const projectName = escapeHTML(project.name);
  const isProjectFavorite = isFavorite(project.id);
  const ports = [
    ...new Set([...project.ports, ...project.services.flatMap((service) => service.ports)]),
  ];
  const portBadges = ports.map((port) => `<span class="port">:${port}</span>`).join('');
  const lifecycleBadge =
    project.lifecycle !== 'active'
      ? `<span class="lifecycle-tag">${escapeHTML(project.lifecycle)}</span>`
      : '';
  const alert =
    needsAttention(project) && project.statusMessage
      ? `<p class="card-alert">${escapeHTML(project.statusMessage)}</p>`
      : '';
  const logsButton = project.run
    ? `<button class="icon-button" data-logs="${projectId}" aria-label="View ${projectName} logs">${renderIcon('log')}</button>`
    : '';

  return `
    <article class="tool-card" data-id="${projectId}" data-status="${escapeHTML(project.status)}">
      <div class="card-top">
        <span class="project-kind">${escapeHTML(kindLabels[project.kind])}</span>
        <div class="card-tools">
          <button class="icon-button favorite-button ${isProjectFavorite ? 'is-favorite' : ''}"
                  data-favorite="${projectId}" aria-pressed="${isProjectFavorite}"
                  aria-label="${isProjectFavorite ? 'Unfavourite' : 'Favourite'} ${projectName}">
            ${renderIcon('star')}
          </button>
          <button class="icon-button drag-handle" data-drag="${projectId}"
                  ${workspaceState.activeTab === 'recent' ? 'hidden' : ''}
                  aria-label="Move ${projectName}; use arrow keys to arrange"
                  title="Drag to arrange · arrow keys also work">
            ${renderIcon('dots')}
          </button>
        </div>
      </div>
      <div class="card-content">
        <h3><button data-project="${projectId}">${projectName}</button></h3>
        <p class="tool-description">${escapeHTML(project.description || 'A local project in your workspace.')}</p>
        <div class="project-location" title="${escapeHTML(project.path)}">
          ${renderIcon('folder')}<span>${escapeHTML(shortenPath(project.path) || 'Folder not recorded')}</span>
        </div>
        <div class="launch-command" title="${escapeHTML(project.launch)}">
          ${renderIcon('terminal')}<code>${escapeHTML(project.launch || 'Launch command not recorded')}</code>
        </div>
      </div>
      <div class="card-meta">
        ${renderStatusBadge(statusLabels[project.status] || project.status, project.status)}
        ${lifecycleBadge}
        <span class="card-ports">${portBadges}</span>
      </div>
      ${alert}
      <div class="card-bottom">
        <button class="text-button" data-project="${projectId}">Details ${renderIcon('chevron')}</button>
        <div class="project-actions">${logsButton}${renderProjectAction(project)}</div>
      </div>
    </article>
  `;
}

import { renderProjectAction } from './cards.js';
import {
  escapeHTML,
  query,
  renderDetailField,
  renderStatusBadge,
  showDialog,
} from '../shared/dom.js';
import { renderIcon } from '../shared/icons.js';
import { kindLabels, statusLabels } from '../shared/project.js';
import { workspaceState } from '../shared/state.js';

export function createAgentHandoff(project) {
  return [
    `${project.name} (${project.id})`,
    project.description,
    '',
    `Path: ${project.path}`,
    `URL: ${project.url}`,
    `Expected ports: ${project.ports.join(', ')}`,
    `Lifecycle: ${project.lifecycle}`,
    `Owner: ${project.owner}`,
    `Launch from project folder: ${project.launch}`,
    `Docs: ${project.docs}`,
    `Maintenance: ${project.maintenance}`,
    '',
    'Read the project instructions, check shared Workbench records and live ports, and update the registry after changes.',
  ].join('\n');
}

export function showProject(projectId) {
  const project = workspaceState.snapshot?.projects.find((project) => project.id === projectId);
  if (!project) return;

  workspaceState.detailId = projectId;
  workspaceState.detailMode = 'overview';
  query('#detail-eyebrow').textContent = 'Project details';

  const escapedId = escapeHTML(projectId);
  const warning =
    project.statusMessage ||
    (!project.pathExists
      ? 'The recorded project folder could not be found. Update the path in settings.'
      : '');
  const warningNotice = warning ? `<div class="alert">${escapeHTML(warning)}</div>` : '';
  const stopButtons = project.services
    .filter((service) => service.canStop)
    .map(
      (service) =>
        `<button class="button quiet" data-stop="${service.pid}">${renderIcon('stop')}Stop</button>`,
    )
    .join('');
  const websiteCheck = project.health
    ? renderDetailField('Website check', project.health.message)
    : '';
  const openAddress = project.url
    ? `<div class="detail-block">
         <span class="detail-label">Open address</span>
         <a class="detail-url" data-open="${escapedId}" href="${escapeHTML(project.url)}"
            target="_blank" rel="noopener noreferrer">${escapeHTML(project.url)} ${renderIcon('external')}</a>
       </div>`
    : '';
  const conflicts = project.conflicts
    .map((service) => `PID ${service.pid} · ${service.name} · ${service.cwd || 'folder unknown'}`)
    .join('\n');
  const conflictingListeners = conflicts
    ? renderDetailField('Listeners to inspect', conflicts, true)
    : '';
  const expectedPorts = project.ports.map((port) => ':' + port).join(', ') || 'None recorded';

  query('#detail-body').innerHTML = `
    <h2 id="detail-title">${escapeHTML(project.name)}</h2>
    <p class="dialog-intro">${escapeHTML(project.description)}</p>
    <div class="detail-badges">
      ${renderStatusBadge(statusLabels[project.status], project.status)}
      <span class="kind-label">${escapeHTML(kindLabels[project.kind])}</span>
    </div>
    ${warningNotice}
    <div class="detail-actions">
      ${renderProjectAction(project)}${stopButtons}
      <button class="button quiet" data-edit="${escapedId}">${renderIcon('settings')}Settings</button>
    </div>
    <div class="detail-tabs">
      <button class="active" data-project="${escapedId}">Overview</button>
      <button data-logs="${escapedId}">Launch logs</button>
    </div>
    ${websiteCheck}
    ${openAddress}
    ${renderDetailField('Project folder', project.path, true, true)}
    ${renderDetailField('Launch command', project.launch, true, true)}
    ${renderDetailField('Expected ports', expectedPorts)}
    ${conflictingListeners}
    <details class="advanced-settings">
      <summary>Maintenance & agent handoff</summary>
      ${renderDetailField('Maintenance', project.maintenance)}
      ${renderDetailField('Documentation', project.docs, true)}
      ${renderDetailField('Owner', project.owner)}
      <button class="button quiet" data-copy="${escapeHTML(createAgentHandoff(project))}">Copy agent handoff</button>
    </details>
    ${projectId === 'workbench' ? '' : `
      <div class="project-removal-action">
        <button class="text-button danger" data-remove-project="${escapedId}">Remove project</button>
      </div>
    `}
  `;

  showDialog('details');
}

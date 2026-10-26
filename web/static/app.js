'use strict';

// The previous engine cannot reload itself. Keep its old bookmark useful locally.
async function redirectLegacyEngine() {
  if (location.port !== '8730') return false;
  try {
    const response = await fetch('/api/health');
    const health = await response.json();
    if (health.app === 'workbench' && !health.version) {
      location.replace('http://127.0.0.1:8731/' + location.hash);
      return true;
    }
  } catch {}
  return false;
}

const query = (selector) => document.querySelector(selector);
const queryAll = (selector) => [...document.querySelectorAll(selector)];
const htmlEntities = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>"']/g, (character) => htmlEntities[character]);
}

const iconPaths = {
  grid: 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z',
  list: 'M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01',
  star: 'm12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2-5.6-3-5.6 3 1.1-6.2L3 9.6l6.2-.9Z',
  clock: 'M12 8v4l3 2',
  activity: 'm2 12 5 0 3-8 4 16 3-8h5',
  layers: 'm12 3 10 6-10 6L2 9Zm-9 12 9 6 9-6M3 12l9 6 9-6',
  terminal: 'm5 7 5 5-5 5M13 17h6',
  download: 'M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5',
  menu: 'M4 6h16M4 12h16M4 18h16',
  moon: 'M21 13A9 9 0 0 1 11 3a9 9 0 1 0 10 10Z',
  sun: 'M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1 1M18 18l1 1M5 19l1-1M18 6l1-1',
  refresh: 'M20 7v5h-5M4 17v-5h5M6 7a7 7 0 0 1 12-2l2 2M18 17a7 7 0 0 1-12 2l-2-2',
  plus: 'M12 5v14M5 12h14',
  folder: 'M3 7V5h6l2 2h10v13H3Z',
  search: 'm16 16 5 5M10 17a7 7 0 1 0 0-14 7 7 0 0 0 0 14',
  close: 'm6 6 12 12M6 18 18 6',
  arrow: 'M5 12h14m-5-5 5 5-5 5',
  external: 'M14 3h7v7m0-7L10 14M10 3H3v18h18v-7',
  play: 'm8 4 13 8-13 8Z',
  stop: 'M6 6h12v12H6Z',
  dots: 'M9 5h.01M15 5h.01M9 12h.01M15 12h.01M9 19h.01M15 19h.01',
  code: 'm8 6-6 6 6 6m8-12 6 6-6 6m-3-16-2 20',
  check: 'm5 12 4 4L19 6',
  chevron: 'm9 5 7 7-7 7',
  log: 'M4 4h16v16H4zM8 8h8M8 12h8M8 16h5',
  settings:
    'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8M12 2v3m0 14v3M2 12h3m14 0h3M5 5l2 2m10 10 2 2M5 19l2-2M17 7l2-2',
};
function renderIcon(name) {
  const circleRadius = name === 'clock' ? 9 : name === 'sun' ? 4 : 0;
  const circle = circleRadius ? `<circle cx="12" cy="12" r="${circleRadius}"/>` : '';

  return `<svg
  viewBox="0 0 24 24"
  width="18"
  height="18"
  fill="none"
  stroke="currentColor"
  stroke-width="1.6"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  ${circle}
  <path d="${iconPaths[name] || iconPaths.code}" />
</svg>`;
}

function paintIcons(root = document) {
  root.querySelectorAll('[data-icon]').forEach((element) => {
    element.innerHTML = renderIcon(element.dataset.icon);
  });
}
const statusLabels = {
  ready: 'Ready',
  running: 'Running',
  starting: 'Starting',
  stopped: 'Stopped',
  failed: 'Launch failed',
  unhealthy: 'Not ready',
  unverified: 'Ownership unverified',
  installed: 'Installed',
  available: 'Available',
  unknown: 'Unknown',
  conflict: 'Port conflict',
  unlocated: 'Folder missing',
};
const kindLabels = {
  web: 'Web project',
  desktop: 'Desktop app',
  cli: 'Command line',
  library: 'Reference',
};
function renderStatusBadge(text, statusClass = '') {
  const escapedClass = escapeHTML(statusClass);

  return `
    <span class="badge ${escapedClass}">
      <span class="status-dot ${escapedClass}" aria-hidden="true"></span>
      ${escapeHTML(text)}
    </span>
  `;
}

const shortenPath = (path) => String(path || '').replace(/^\/Users\/[^/]+/, '~');
const projectFields = [
  'id',
  'name',
  'description',
  'kind',
  'path',
  'url',
  'ports',
  'lifecycle',
  'owner',
  'launch',
  'maintenance',
  'docs',
  'updatedAt',
];
const needsAttention = (project) =>
  ['failed', 'unhealthy', 'unverified', 'conflict', 'unknown', 'unlocated'].includes(
    project.status,
  ) ||
  project.extraServices ||
  (project.services?.length && project.lifecycle !== 'active');
const isProjectReady = (project) => project.status === 'ready';
const showNotice = (message) => {
  query('#notice').textContent = message;
  query('#notice').hidden = !message;
};
const storedPreferences = readStoredPreferences();
const preferences = {
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
const workspaceState = {
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
  draggedProjectId: null,
  pointerDrag: null,
  lastRefreshAt: 0,
  pendingLaunches: new Map(),
  projectIdEdited: false,
  setupLaunchProjectId: null,
};

function readStoredPreferences() {
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

function savePreferences() {
  try {
    localStorage.setItem('workbench.workspace.v2', JSON.stringify(preferences));
  } catch {
    showNotice(
      'Your browser could not save workspace preferences. Project records are still saved on disk.',
    );
  }
}

function applyTheme() {
  document.documentElement.dataset.theme = preferences.theme;
  query('#theme-toggle').innerHTML = renderIcon(preferences.theme === 'dark' ? 'sun' : 'moon');
  query('#theme-toggle').setAttribute(
    'aria-label',
    `Switch to ${preferences.theme === 'dark' ? 'light' : 'dark'} mode`,
  );
  query('meta[name="theme-color"]').content = preferences.theme === 'dark' ? '#0c0c0c' : '#f7f7f5';
}

function setSidebar(isOpen) {
  const mobile = window.matchMedia?.('(max-width: 1000px)')?.matches || false;
  query('#sidebar').classList.toggle('mobile-open', isOpen);
  query('#sidebar').inert = mobile && !isOpen;
  query('#menu-toggle').setAttribute('aria-expanded', String(isOpen));
}

function showDialog(id) {
  const element = query('#' + id);
  if (!element.open) element.showModal();
}

async function requestJSON(path, body) {
  const response = await fetch(
    path,
    body
      ? {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Workbench-Token': workspaceState.snapshot?.csrfToken || '',
          },
          body: JSON.stringify(body),
        }
      : {},
  );
  let data;
  try {
    data = await response.json();
  } catch {
    throw new Error('The local engine did not return a valid response. Refresh and retry.');
  }
  if (!response.ok) throw new Error(data.error || 'The local request failed.');
  return data;
}
const workspaceProjects = () =>
  workspaceState.snapshot?.projects.filter((project) => project.id !== 'workbench') || [];
function orderedProjects() {
  const projectRanks = new Map(preferences.order.map((id, index) => [id, index]));
  return [...workspaceProjects()].sort(
    (firstProject, secondProject) =>
      (projectRanks.get(firstProject.id) ?? 10000) - (projectRanks.get(secondProject.id) ?? 10000),
  );
}

function isFavorite(id) {
  return preferences.favorites.includes(id);
}

function recordOpen(id) {
  preferences.recent[id] = Date.now();
  savePreferences();
  renderResume();
}

function renderProjectAction(project) {
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

function renderSummary() {
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

function renderResume() {
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

function renderTools() {
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

function emptyWorkspace(search, total) {
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

function renderServices() {
  if (!workspaceState.snapshot) return;
  const { inspectionError } = workspaceState.snapshot;
  const filter = query('#service-filter').value;
  const search = query('#service-search').value.trim().toLowerCase();
  const services = workspaceState.snapshot.services.filter(
    (service) =>
      (filter === 'all' || (filter === 'projects' ? service.projectId : !service.projectId)) &&
      [service.name, service.pid, service.projectName, service.cwd, ...service.ports]
        .join(' ')
        .toLowerCase()
        .includes(search),
  );
  query('#services-count').textContent = services.length;
  query('#service-alerts').innerHTML = [
    ...(workspaceState.snapshot.controlWarnings || []),
    ...(inspectionError ? ['Process inspection unavailable: ' + inspectionError] : []),
  ]
    .map((warning) => `<div class="alert">${escapeHTML(warning)}</div>`)
    .join('');
  query('#service-list').innerHTML = services.length
    ? services.map(renderServiceRow).join('')
    : '<tr><td colspan="5"><div class="empty">No matching listeners.</div></td></tr>';
}

function renderSkills() {
  if (!workspaceState.skills) return;
  const search = query('#skill-search').value.trim().toLowerCase();
  const provider = query('#skill-provider').value;
  const scope = query('#skill-scope').value;
  const nonProjectScopes = new Set(['Personal', 'Shared', 'Plugin cache', 'Synced plugin']);
  const skills = workspaceState.skills.skills.filter(
    (skill) =>
      (!provider || skill.providers.includes(provider)) &&
      (!scope ||
        (scope === 'project'
          ? skill.scopes.some((entry) => !nonProjectScopes.has(entry))
          : skill.scopes.includes(scope))) &&
      [skill.name, skill.description, skill.path, ...skill.providers]
        .join(' ')
        .toLowerCase()
        .includes(search),
  );
  query('#skills-count').textContent = skills.length;
  query('#skill-list').innerHTML = skills.length
    ? skills.map(renderSkillCard).join('')
    : '<div class="empty">No skills match these filters.</div>';
}

function setTab() {
  const tab = location.hash.slice(1);
  workspaceState.activeTab = ['tools', 'favorites', 'recent', 'services', 'skills'].includes(tab)
    ? tab
    : 'tools';
  const isProjectTab = ['tools', 'favorites', 'recent'].includes(workspaceState.activeTab);
  queryAll('.nav').forEach((element) => {
    const active = element.dataset.tab === workspaceState.activeTab;
    element.classList.toggle('active', active);
    if (active) element.setAttribute('aria-current', 'page');
    else element.removeAttribute('aria-current');
  });
  query('#tools-view').hidden = !isProjectTab;
  query('#workspace-summary').hidden = !isProjectTab;
  query('#services-view').hidden = workspaceState.activeTab !== 'services';
  query('#skills-view').hidden = workspaceState.activeTab !== 'skills';
  const titles = {
    tools: ['Projects', 'Projects', 'Your local work, ready to pick up.'],
    favorites: ['Favourites', 'Your go-to projects', 'The projects you want within reach.'],
    recent: [
      'Recently opened',
      'Pick up where you left off',
      'A way back into the work you opened here.',
    ],
    services: [
      'Diagnostics',
      'Workspace diagnostics',
      'Real listeners, project ownership, and the details to resolve a problem.',
    ],
    skills: [
      'Agent skills',
      'Your installed skills',
      'Know-how from Codex, Claude, and your project folders.',
    ],
  };
  const [breadcrumbTitle, title, subtitle] = titles[workspaceState.activeTab];
  query('#breadcrumb').textContent = breadcrumbTitle;
  query('#page-title').textContent = title;
  query('#page-subtitle').textContent = subtitle;
  query('#tools-title').textContent =
    workspaceState.activeTab === 'recent'
      ? 'Recent projects'
      : workspaceState.activeTab === 'favorites'
        ? 'Favourite projects'
        : 'Project workspace';
  setSidebar(false);
  renderTools();
  if (workspaceState.activeTab === 'skills' && !workspaceState.skills) loadSkills();
  renderResume();
}

function renderDetailField(label, value, code = false, copy = false) {
  const valueElement = code ? 'code' : 'p';
  const valueClass = code ? 'code-line' : 'detail-value';
  const copyButton =
    copy && value
      ? `<button class="text-button" data-copy="${escapeHTML(value)}">Copy ${escapeHTML(label.toLowerCase())}</button>`
      : '';

  return `
    <div class="detail-block">
      <span class="detail-label">${escapeHTML(label)}</span>
      <${valueElement} class="${valueClass}">${escapeHTML(value || 'Not recorded yet.')}</${valueElement}>
      ${copyButton}
    </div>
  `;
}

function createAgentHandoff(project) {
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

function showProject(projectId) {
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
  `;

  showDialog('details');
}

async function showLogs(id) {
  const project = workspaceState.snapshot?.projects.find((project) => project.id === id);
  if (!project) return;
  workspaceState.detailId = id;
  workspaceState.detailMode = 'logs';
  query('#detail-eyebrow').textContent = 'Launch output';
  query('#detail-body').innerHTML = `<h2 id="detail-title">${escapeHTML(project.name)}</h2>
<p class="dialog-intro">Output from the latest launch started through Workbench.</p>
<div class="detail-tabs">
  <button data-project="${escapeHTML(id)}">Overview</button>
  <button class="active" data-logs="${escapeHTML(id)}">Launch logs</button>
</div>
<div class="logs-toolbar">
  <span>Latest launch output</span>
  <button class="text-button" data-refresh-logs="${escapeHTML(id)}">Refresh logs</button>
</div>
<div id="log-fragment">
  <p id="log-caption" class="footnote">Loading output…</p>
  <pre id="log-output" class="log-output">Reading launch output…</pre>
</div>
<button class="button quiet" data-edit="${escapeHTML(id)}">Edit launch settings</button>`;
  showDialog('details');
  const fragment = query('#log-fragment');
  if (workspaceState.snapshot?.features?.htmlLogs && window.htmx) {
    fragment.setAttribute('hx-get', '/fragments/logs?id=' + encodeURIComponent(id));
    fragment.setAttribute('hx-trigger', 'load, refresh-logs');
    fragment.setAttribute('hx-swap', 'innerHTML');
    window.htmx.process(fragment);
  } else await refreshLogs(id);
}

async function refreshLogs(id) {
  const fragment = query('#log-fragment');
  if (fragment?.hasAttribute('hx-get') && window.htmx) {
    window.htmx.trigger(fragment, 'refresh-logs');
    return;
  }
  try {
    const data = await requestJSON('/api/logs?id=' + encodeURIComponent(id));
    if (
      workspaceState.detailId !== id ||
      workspaceState.detailMode !== 'logs' ||
      !query('#log-output')
    )
      return;
    query('#log-output').textContent = data.text || data.message;
    query('#log-caption').textContent = data.message;
  } catch (error) {
    if (query('#log-output')) query('#log-output').textContent = error.message;
  }
}

function showSkill(id) {
  const skill = workspaceState.skills?.skills.find((skill) => skill.id === id);
  if (!skill) return;
  workspaceState.detailId = null;
  query('#detail-eyebrow').textContent = 'Installed skill';
  query('#detail-body').innerHTML = `<h2 id="detail-title">${escapeHTML(skill.name)}</h2>
<p class="dialog-intro">${escapeHTML(skill.description)}</p>
${renderDetailField('Skill file', skill.path, true, true)}
${renderDetailField('Found through', skill.locations.map(shortenPath).join('\n'), true)}
${renderDetailField('Location', skill.scopes.join(' · '))}
<p class="footnote">${escapeHTML(workspaceState.skills.note)}</p>`;
  showDialog('details');
}

function openProjectEditor(id, prefill = null, setupLaunch = false) {
  const project =
    prefill || (id ? workspaceState.snapshot?.projects.find((project) => project.id === id) : null);
  if (!workspaceState.snapshot) {
    showNotice('Wait for the local registry to load.');
    return;
  }
  query('#details').close();
  query('#discover-dialog').close();
  query('#tool-form').reset();
  workspaceState.setupLaunchProjectId = setupLaunch ? id : null;
  query('#save-tool').textContent = setupLaunch ? 'Save & review start' : 'Save project';
  query('#editor-intro').textContent = setupLaunch
    ? 'Add the local website URL and its expected port. You’ll review the command before it runs.'
    : 'Give it a name and a way to run. You can update the details whenever they change.';
  query('#editor-title').textContent = id
    ? 'Project settings'
    : prefill
      ? 'Review discovered project'
      : 'Add a project';
  for (const field of projectFields) {
    const input = query('#tool-form').elements.namedItem(field);
    if (input)
      input.value =
        field === 'ports' ? (project?.ports || []).join(', ') : project?.[field] || input.value;
  }
  if (setupLaunch) query('#editor-title').textContent = 'Set up ' + project.name + ' to start';
  query('#tool-form').elements.namedItem('id').readOnly = !!id;
  workspaceState.projectIdEdited = !!id;
  workspaceState.editorRevision = workspaceState.snapshot.revision;
  query('#form-error').hidden = true;
  showDialog('editor');
}

async function prepareLaunch(id) {
  const requestSequence = ++workspaceState.launchRequestSequence;
  const project = workspaceState.snapshot?.projects.find((project) => project.id === id);
  if (!project) return;
  workspaceState.launchPlan = null;
  query('#launch-title').textContent = 'Start ' + project.name + '?';
  query('#launch-description').textContent = 'Runs in the project folder using the command below.';
  query('#launch-body').innerHTML = '<p class="footnote">Checking the launch details…</p>';
  query('#launch-error').hidden = true;
  query('#confirm-launch').disabled = true;
  showDialog('launch-dialog');
  try {
    const plan = await requestJSON('/api/launch-plan?id=' + encodeURIComponent(id));
    if (!query('#launch-dialog').open || requestSequence !== workspaceState.launchRequestSequence)
      return;
    workspaceState.launchPlan = plan;
    query('#launch-body').innerHTML =
      renderDetailField('Working folder', plan.cwd, true) +
      renderDetailField('Launch command', plan.command, true);
    query('#confirm-launch').disabled = false;
  } catch (error) {
    if (requestSequence !== workspaceState.launchRequestSequence) return;
    query('#launch-body').innerHTML = '';
    query('#launch-error').textContent = error.message;
    query('#launch-error').hidden = false;
  }
}

function showStop(pid) {
  const service = workspaceState.snapshot?.services.find(
    (service) => service.pid === pid && service.canStop,
  );
  if (!service) return;
  workspaceState.stopTarget = { ...service };
  query('#stop-body').innerHTML =
    `<p class="dialog-intro">
  Stop
  <strong>${escapeHTML(service.projectName)}</strong>
  ? Pages using this service will disconnect.
</p>` +
    renderDetailField('Process', `${service.name} · PID ${service.pid}`) +
    renderDetailField('Listening addresses', service.endpoints.join('\n'), true) +
    renderDetailField('Working folder', service.cwd, true) +
    '<p class="footnote">The process identity is checked again before a graceful stop is sent.</p>';
  query('#stop-error').hidden = true;
  showDialog('stop-dialog');
}

function trackServiceAsProject(pid) {
  const service = workspaceState.snapshot?.services.find((service) => service.pid === pid);
  if (!service) return;
  openProjectEditor();
  const form = query('#tool-form').elements;
  form.namedItem('name').value = service.name;
  form.namedItem('id').value = service.name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');
  form.namedItem('path').value = service.cwd === '/' ? '' : service.cwd;
  form.namedItem('ports').value = service.ports.join(', ');
  form.namedItem('maintenance').value =
    `Found ${service.name} listening on ${service.endpoints.join(', ')}. Confirm the project folder and launch command.`;
}

function openDiscover() {
  if (!workspaceState.snapshot?.features?.discovery) {
    showNotice(
      'Project discovery needs the updated local engine. Reopen Workbench with its launcher.',
    );
    return;
  }
  query('#discover-error').hidden = true;
  query('#discovery-roots').innerHTML = (workspaceState.snapshot.discoveryRoots || [])
    .map(
      (folderPath) =>
        `<button class="root-chip" data-root="${escapeHTML(folderPath)}">${renderIcon('folder')}${escapeHTML(shortenPath(folderPath))}</button>`,
    )
    .join('');
  showDialog('discover-dialog');
}

function renderDiscovery(discovery) {
  const { candidates, warnings, scanned, limited } = discovery;
  workspaceState.candidates = candidates;

  const limitNotice = limited
    ? '<p class="footnote">Scan limit reached. Choose a smaller folder to find the rest.</p>'
    : '';
  const results = candidates.length
    ? candidates.map(renderDiscoveryCandidate).join('')
    : `<div class="empty">
         <strong>No new projects found.</strong>
         <p>Try a folder containing package.json, go.mod, a Python launcher or index.html. Already registered projects are skipped.</p>
       </div>`;
  const warningNotices = warnings
    .map((warning) => `<p class="footnote">${escapeHTML(warning)}</p>`)
    .join('');

  query('#discovery-results').innerHTML = `
    <div class="discovery-caption">
      <strong>${candidates.length} project${candidates.length === 1 ? '' : 's'} found</strong>
      <span>${scanned} folders checked · existing projects skipped</span>
    </div>
    ${limitNotice}
    ${results}
    ${warningNotices}
  `;
}

function reorderProject(id, target) {
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

async function copyText(text, button) {
  try {
    await navigator.clipboard.writeText(text);
    const original = button.textContent;
    button.textContent = 'Copied';
    setTimeout(() => (button.textContent = original), 1200);
  } catch {
    showNotice('Clipboard access is unavailable. Select and copy the displayed text.');
  }
}

async function loadSkills(force = false) {
  try {
    workspaceState.skills = await requestJSON('/api/skills' + (force ? '?refresh=1' : ''));
    renderSkills();
    renderSummary();
  } catch (error) {
    query('#skill-list').innerHTML =
      '<div class="empty">Could not discover skills. Use Refresh to try again.</div>';
    showNotice(error.message);
  }
}

function settlePending() {
  for (const [id, pending] of workspaceState.pendingLaunches) {
    const project = workspaceState.snapshot?.projects.find((project) => project.id === id);
    if (!project) continue;
    if (project.status === 'ready') {
      if (pending.tab && !pending.tab.closed) {
        pending.tab.location.replace(project.url);
        recordOpen(id);
      } else showNotice(project.name + ' is ready. Use Open to enter the project.');
      workspaceState.pendingLaunches.delete(id);
    } else if (
      ['failed', 'conflict', 'unverified'].includes(project.status) ||
      Date.now() - pending.at > 120000
    ) {
      if (pending.tab && !pending.tab.closed) pending.tab.close();
      workspaceState.pendingLaunches.delete(id);
      showNotice(
        project.name +
          ': ' +
          (project.statusMessage ||
            'The website is taking longer to start. Inspect the project logs.'),
      );
    }
  }
}

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

// Card rendering and user actions.
function renderResumeItem(project) {
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

function renderProjectCard(project) {
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
          <button class="icon-button drag-handle" data-drag="${projectId}" draggable="true"
                  aria-label="Move ${projectName}; use left or right arrow keys"
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

function renderServiceAction(service) {
  if (service.canStop) {
    return `<button class="stop-button" data-stop="${service.pid}">Stop…</button>`;
  }

  if (!service.projectId) {
    return `<button class="text-button" data-track="${service.pid}">Track project</button>`;
  }

  let label = 'Inspect only';
  if (service.self) label = 'Current engine';
  else if (service.stopUnavailable) label = 'Stop unavailable';

  return `<small title="${escapeHTML(service.stopUnavailable || '')}">${label}</small>`;
}

function renderServiceRow(service) {
  const endpoints = service.endpoints
    .map((endpoint) => `<span class="port">${escapeHTML(endpoint)}</span>`)
    .join('');
  const engineLabel = service.self ? ' · Workbench engine' : '';
  const folderLabel = service.cwd ? shortenPath(service.cwd) : 'Folder unavailable';

  return `
    <tr>
      <td>
        <strong>${escapeHTML(service.name)}</strong>
        <small>${escapeHTML(service.projectName || 'Ownership unverified')}${engineLabel}</small>
      </td>
      <td class="numeric"><span class="port">${service.pid}</span></td>
      <td><div class="ports">${endpoints}</div></td>
      <td><div class="process-path" title="${escapeHTML(service.cwd)}">${escapeHTML(folderLabel)}</div></td>
      <td>${renderServiceAction(service)}</td>
    </tr>
  `;
}

function renderSkillCard(skill) {
  const providerTags = skill.providers
    .map((provider) => {
      const label = provider === 'codex' ? 'Codex' : 'Claude';
      return `<span class="provider-tag">${label}</span>`;
    })
    .join('');

  return `
    <button class="skill-card" data-skill="${escapeHTML(skill.id)}">
      <span class="skill-badges">${providerTags}</span>
      <h3>${escapeHTML(skill.name)}</h3>
      <p>${escapeHTML(skill.description)}</p>
      <span class="skill-location">${escapeHTML(skill.scopes.join(' · '))}${renderIcon('arrow')}</span>
    </button>
  `;
}

function renderDiscoveryCandidate(candidate, index) {
  const { project, reasons, needsPort } = candidate;
  const portHint = needsPort ? ' · Add a URL and port before starting.' : '';

  return `
    <article class="discovered-project">
      <div>
        <h3>${escapeHTML(project.name)}</h3>
        <p class="project-location">${escapeHTML(shortenPath(project.path))}</p>
        <code>${escapeHTML(project.launch)}</code>
        <p class="footnote">${escapeHTML(reasons.join(' · '))}${portHint}</p>
      </div>
      <button class="button quiet" data-import="${index}">Review & add ${renderIcon('arrow')}</button>
    </article>
  `;
}

function toggleTheme() {
  preferences.theme = preferences.theme === 'light' ? 'dark' : 'light';
  applyTheme();
  savePreferences();
}

function handleWorkspaceClick(event) {
  const button = event.target.closest('button,a');
  if (!button) return;
  if (button.dataset.close) {
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

function handleProjectDragStart(event) {
  const handle = event.target.closest('[data-drag]');
  if (!handle) return;
  workspaceState.draggedProjectId = handle.dataset.drag;
  event.dataTransfer?.setData('text/plain', workspaceState.draggedProjectId);
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
  handle.closest('.tool-card').classList.add('dragging');
}

function handleProjectDragOver(event) {
  if (!workspaceState.draggedProjectId) return;
  const card = event.target.closest('.tool-card');
  if (card) {
    event.preventDefault();
    queryAll('.drop-target').forEach((card) => card.classList.remove('drop-target'));
    card.classList.add('drop-target');
  }
}

function handleProjectDrop(event) {
  if (!workspaceState.draggedProjectId) return;
  event.preventDefault();
  const target = event.target.closest('.tool-card');
  if (target) reorderProject(workspaceState.draggedProjectId, target.dataset.id);
  workspaceState.draggedProjectId = null;
}

function clearProjectDrag() {
  workspaceState.draggedProjectId = null;
  queryAll('.dragging,.drop-target').forEach((card) =>
    card.classList.remove('dragging', 'drop-target'),
  );
}

function handleProjectMoveKeydown(event) {
  const handle = event.target.closest('[data-drag]');
  if (!handle || !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return;
  event.preventDefault();
  const order = orderedProjects();
  const index = order.findIndex((project) => project.id === handle.dataset.drag);
  const next = index + (['ArrowRight', 'ArrowDown'].includes(event.key) ? 1 : -1);
  if (order[next]) {
    reorderProject(handle.dataset.drag, order[next].id);
    query(`[data-drag="${handle.dataset.drag}"]`)?.focus();
  }
}

function handleProjectPointerDown(event) {
  if (event.pointerType === 'mouse') return;
  const handle = event.target.closest('[data-drag]');
  if (handle)
    workspaceState.pointerDrag = { id: handle.dataset.drag, x: event.clientX, y: event.clientY };
}

function handleProjectPointerUp(event) {
  if (!workspaceState.pointerDrag) return;
  const target = document.elementFromPoint(event.clientX, event.clientY)?.closest('.tool-card');
  if (
    target &&
    Math.hypot(
      event.clientX - workspaceState.pointerDrag.x,
      event.clientY - workspaceState.pointerDrag.y,
    ) > 15
  )
    reorderProject(workspaceState.pointerDrag.id, target.dataset.id);
  workspaceState.pointerDrag = null;
}

function clearProjectPointerDrag() {
  workspaceState.pointerDrag = null;
}

async function saveProjectFromForm(event) {
  event.preventDefault();
  query('#save-tool').disabled = true;
  query('#form-error').hidden = true;
  try {
    const project = readProjectForm(event.target);
    if (workspaceState.setupLaunchProjectId) {
      validateLocalWebsite(project);
    }
    if (
      !query('#tool-form').elements.namedItem('id').readOnly &&
      workspaceState.snapshot.projects.some((existingProject) => existingProject.id === project.id)
    )
      throw new Error(
        'That ID already belongs to another project. Choose a different ID under More details.',
      );
    const reviewAfterSave = workspaceState.setupLaunchProjectId === project.id;
    const editing = query('#tool-form').elements.namedItem('id').readOnly;
    await requestJSON('/api/project', { project, revision: workspaceState.editorRevision });
    workspaceState.setupLaunchProjectId = null;
    query('#editor').close();
    await refresh();
    showNotice(project.name + (editing ? ' updated.' : ' added to your workspace.'));
    if (reviewAfterSave) await prepareLaunch(project.id);
  } catch (error) {
    query('#form-error').textContent = error.message;
    query('#form-error').hidden = false;
  } finally {
    query('#save-tool').disabled = false;
  }
}

function readProjectForm(formElement) {
  const formData = new FormData(formElement);
  const project = {};

  for (const field of projectFields) {
    if (field !== 'updatedAt' && field !== 'ports') {
      project[field] = String(formData.get(field) || '').trim();
    }
  }

  project.ports = parseExpectedPorts(String(formData.get('ports') || '').trim());
  project.description ||= 'Local project in ' + project.path.split('/').filter(Boolean).pop() + '.';

  return project;
}

function parseExpectedPorts(value) {
  if (value && !/^\d+(\s*,\s*\d+)*$/.test(value)) {
    throw new Error('Use comma-separated port numbers, such as 3000, 8080.');
  }

  return value ? value.split(',').map(Number) : [];
}

function validateLocalWebsite(project) {
  let address;

  try {
    address = new URL(project.url);
  } catch {
    throw new Error('Add the local website URL before reviewing the launch.');
  }

  const isWebAddress = ['http:', 'https:'].includes(address.protocol);
  const isLoopback = ['localhost', '127.0.0.1', '[::1]'].includes(address.hostname);
  const hasCredentials = Boolean(address.username || address.password);
  const port = Number(address.port || (address.protocol === 'https:' ? 443 : 80));

  if (!isWebAddress || !isLoopback || hasCredentials || !project.ports.includes(port)) {
    throw new Error('Use a local website URL and include its port in Expected ports.');
  }
}

async function discoverProjectsFromForm(event) {
  event.preventDefault();
  query('#scan-projects').disabled = true;
  query('#discover-error').hidden = true;
  query('#discovery-results').innerHTML =
    '<div class="empty loading"><span class="loader"></span><strong>Looking for projects…</strong><p>Checking launch scripts and package files.</p></div>';
  try {
    renderDiscovery(
      await requestJSON('/api/discover', { path: query('#discover-path').value.trim() }),
    );
  } catch (error) {
    query('#discover-error').textContent = error.message;
    query('#discover-error').hidden = false;
    query('#discovery-results').innerHTML = '';
  } finally {
    query('#scan-projects').disabled = false;
  }
}

async function chooseDiscoveryFolder() {
  query('#choose-folder').disabled = true;
  try {
    const result = await requestJSON('/api/choose-folder', {});
    query('#discover-path').value = result.path;
  } catch (error) {
    query('#discover-error').textContent = error.message;
    query('#discover-error').hidden = false;
  } finally {
    query('#choose-folder').disabled = false;
  }
}

async function launchReviewedProject() {
  const plan = workspaceState.launchPlan;
  if (!plan) return;
  query('#confirm-launch').disabled = true;
  query('#launch-error').hidden = true;
  let tab = null;
  if (query('#open-when-ready').checked) {
    tab = window.open('about:blank', '_blank');
    if (tab) {
      tab.opener = null;
      tab.document.title = 'Starting ' + plan.name;
      tab.document.body.textContent =
        'Starting ' +
        plan.name +
        '… This tab will open the project when its local website is ready.';
    }
  }
  try {
    const result = await requestJSON('/api/start', {
      id: plan.id,
      fingerprint: plan.fingerprint,
      revision: plan.revision,
    });
    if (query('#open-when-ready').checked)
      workspaceState.pendingLaunches.set(plan.id, { tab, at: Date.now() });
    query('#launch-dialog').close();
    showNotice(result.message);
    await refresh();
  } catch (error) {
    if (tab && !tab.closed) tab.close();
    query('#launch-error').textContent = error.message;
    query('#launch-error').hidden = false;
  } finally {
    query('#confirm-launch').disabled = false;
  }
}

async function stopReviewedService() {
  if (!workspaceState.stopTarget) return;
  query('#confirm-stop').disabled = true;
  query('#stop-error').hidden = true;
  try {
    const result = await requestJSON('/api/stop', {
      pid: workspaceState.stopTarget.pid,
      stopToken: workspaceState.stopTarget.stopToken,
    });
    query('#stop-dialog').close();
    workspaceState.stopTarget = null;
    await new Promise((resolve) => setTimeout(resolve, 400));
    await refresh();
    showNotice(result.message);
  } catch (error) {
    query('#stop-error').textContent = error.message;
    query('#stop-error').hidden = false;
  } finally {
    query('#confirm-stop').disabled = false;
  }
}

async function exportRegistry() {
  try {
    const data = await requestJSON('/api/registry');
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(data, null, 2) + '\n'], { type: 'application/json' }),
    );
    const downloadLink = document.createElement('a');
    downloadLink.href = url;
    downloadLink.download = 'workbench-registry.json';
    downloadLink.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  } catch (error) {
    showNotice(error.message);
  }
}

function showAgentGuide() {
  workspaceState.detailId = null;
  const root =
    workspaceState.snapshot?.registryPath.replace(/\/registry\.json$/, '') || '/path/to/workbench';
  const cli = workspaceState.snapshot?.features?.htmlLogs
    ? `'${root}/workbench'`
    : `python3 '${root}/registry.py'`;
  query('#detail-eyebrow').textContent = 'Human + agent';
  query('#detail-body').innerHTML =
    '<h2 id="detail-title">One shared workspace.</h2><p class="dialog-intro">You and your coding agents read and update the same project records.</p>' +
    renderDetailField('List projects', `${cli} list --json`, true, true) +
    renderDetailField('Inspect services', `${cli} status --json`, true, true) +
    renderDetailField('Check a port', `${cli} check-port 3000`, true, true) +
    renderDetailField(
      'Register a project',
      `${cli} register --file /path/to/project.json`,
      true,
      true,
    ) +
    renderDetailField('Discover skills', `${cli} skills --json`, true, true) +
    renderDetailField('Registry file', workspaceState.snapshot?.registryPath || '', true, true);
  showDialog('details');
}

function handleProjectTabKeydown(event) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
  const tabs = queryAll('.project-tab');
  const index = tabs.indexOf(event.target);
  if (index < 0) return;
  event.preventDefault();
  const next =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? tabs.length - 1
        : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length;
  tabs[next].click();
  tabs[next].focus();
}

function handleSidebarEscape(event) {
  if (event.key === 'Escape' && query('#sidebar').classList.contains('mobile-open')) {
    setSidebar(false);
    query('#menu-toggle').focus();
  }
}

function handleNavigationChange() {
  workspaceState.projectFilter = 'all';
  setTab();
}

function handleSearchShortcut(event) {
  if (
    event.key === '/' &&
    !event.metaKey &&
    !event.ctrlKey &&
    !event.altKey &&
    !['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement.tagName) &&
    !queryAll('dialog[open]').length
  ) {
    event.preventDefault();
    query(
      { services: '#service-search', skills: '#skill-search' }[workspaceState.activeTab] ||
        '#tool-search',
    ).focus();
  }
}

function refreshWhenVisible() {
  if (!document.hidden) refresh();
}

// Initialization and event registration.
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

  bindNavigationEvents();
  bindProjectEvents();
  bindArrangementEvents();
  bindLogEvents();
  setTab();

  redirectLegacyEngine().then((forwarded) => {
    if (!forwarded) refresh();
  });

  workspaceState.lastRefreshAt = Date.now();
  setInterval(pollWorkspace, 1000);
  document.addEventListener('visibilitychange', refreshWhenVisible);
}

function bindNavigationEvents() {
  query('#theme-toggle').addEventListener('click', toggleTheme);
  query('#menu-toggle').addEventListener('click', () => {
    setSidebar(!query('#sidebar').classList.contains('mobile-open'));
  });
  query('#sidebar-close').addEventListener('click', () => {
    setSidebar(false);
    query('#menu-toggle').focus();
  });

  query('#engine-details').addEventListener('click', () => showProject('workbench'));
  query('#refresh').addEventListener('click', () => refresh(true));
  query('#export').addEventListener('click', exportRegistry);
  query('#agent-guide').addEventListener('click', showAgentGuide);

  query('.project-tabs').addEventListener('keydown', handleProjectTabKeydown);
  document.addEventListener('keydown', handleSidebarEscape);
  document.addEventListener('keydown', handleSearchShortcut);
  window.addEventListener('hashchange', handleNavigationChange);
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

function bindArrangementEvents() {
  const projectList = query('#tool-list');
  projectList.addEventListener('dragstart', handleProjectDragStart);
  projectList.addEventListener('dragover', handleProjectDragOver);
  projectList.addEventListener('drop', handleProjectDrop);
  projectList.addEventListener('dragend', clearProjectDrag);
  projectList.addEventListener('keydown', handleProjectMoveKeydown);
  projectList.addEventListener('pointerdown', handleProjectPointerDown);
  projectList.addEventListener('pointerup', handleProjectPointerUp);
  projectList.addEventListener('pointercancel', clearProjectPointerDrag);
}

function updateSuggestedProjectId(event) {
  if (workspaceState.projectIdEdited) return;

  query('#tool-form').elements.namedItem('id').value = event.target.value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 64);
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

function bindLogEvents() {
  // Failed fragment requests retain the previous output and offer a manual retry.
  document.addEventListener('htmx:responseError', (event) => {
    if (event.detail.elt?.id === 'log-fragment' && query('#log-caption')) {
      query('#log-caption').textContent =
        'Could not refresh output. Use Refresh logs to try again.';
    }
  });

  document.addEventListener('htmx:sendError', (event) => {
    if (event.detail.elt?.id === 'log-fragment' && query('#log-caption')) {
      query('#log-caption').textContent =
        'The local engine is unavailable. Use Refresh logs to retry.';
    }
  });
}

initializeWorkspace();

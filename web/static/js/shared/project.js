export const statusLabels = {
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

export const kindLabels = {
  web: 'Web project',
  desktop: 'Desktop app',
  cli: 'Command line',
  library: 'Reference',
};

export const projectFields = [
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

export const needsAttention = (project) =>
  ['failed', 'unhealthy', 'unverified', 'conflict', 'unknown', 'unlocated'].includes(
    project.status,
  ) ||
  project.extraServices ||
  (project.services?.length && project.lifecycle !== 'active');

export const isProjectReady = (project) => project.status === 'ready';

import { requestJSON } from '../shared/api.js';
import { query, renderDetailField, showDialog, showNotice } from '../shared/dom.js';
import { workspaceState } from '../shared/state.js';

export async function exportRegistry() {
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

export function showAgentGuide() {
  workspaceState.detailId = null;
  const root =
    workspaceState.snapshot?.registryPath.replace(/\/registry\.json$/, '') || '/path/to/workbench';
  const executable = workspaceState.snapshot?.executable || `${root}/workbench`;
  const quoteArgument = (value) => "'" + value.replaceAll("'", "'\\''") + "'";
  const cli = `${quoteArgument(executable)} --root ${quoteArgument(root)}`;
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

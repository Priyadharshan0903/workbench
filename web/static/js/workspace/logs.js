import { requestJSON } from '../shared/api.js';
import { escapeHTML, query, showDialog } from '../shared/dom.js';
import { workspaceState } from '../shared/state.js';

export async function showLogs(id) {
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

export async function refreshLogs(id) {
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

export function bindLogEvents() {
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

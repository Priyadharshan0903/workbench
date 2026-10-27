import { requestJSON } from '../shared/api.js';
import {
  escapeHTML,
  query,
  renderDetailField,
  shortenPath,
  showDialog,
  showNotice,
} from '../shared/dom.js';
import { workspaceState } from '../shared/state.js';

export function createServiceController({ refresh }) {
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

  function renderServiceAction(service) {
    if (service.self) {
      return '<small>Current engine</small>';
    }

    const actions = [];
    if (!service.projectId) {
      actions.push(
        `<button class="text-button" data-track="${service.pid}">Track project</button>`,
      );
    }

    if (service.canStop) {
      actions.push(`<button class="stop-button" data-stop="${service.pid}">Stop…</button>`);
    } else if (service.stopUnavailable) {
      actions.push(
        `<small title="${escapeHTML(service.stopUnavailable)}">Stop unavailable</small>`,
      );
    } else if (service.projectId) {
      actions.push('<small>Inspect only</small>');
    }

    return `<div class="heading-actions">${actions.join('')}</div>`;
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

  function showStop(pid) {
    const service = workspaceState.snapshot?.services.find(
      (service) => service.pid === pid && service.canStop,
    );
    if (!service) return;

    workspaceState.stopTarget = { ...service };
    const processLabel = service.projectName || service.name;
    const registrationNote = service.projectId
      ? ''
      : '<p class="footnote">This process is not registered in Workbench. Confirm its folder and ports before stopping it.</p>';

    query('#stop-body').innerHTML =
      `<p class="dialog-intro">
  Stop
  <strong>${escapeHTML(processLabel)}</strong>
  ? Pages using this service will disconnect.
</p>` +
      registrationNote +
      renderDetailField('Process', `${service.name} · PID ${service.pid}`) +
      renderDetailField('Listening addresses', service.endpoints.join('\n'), true) +
      renderDetailField('Working folder', service.cwd, true) +
      '<p class="footnote">Stop sends a graceful termination request to this PID and affects all its listed ports. The process identity is checked again before sending it.</p>';
    query('#stop-error').hidden = true;
    showDialog('stop-dialog');
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
  return { renderServices, showStop, stopReviewedService };
}

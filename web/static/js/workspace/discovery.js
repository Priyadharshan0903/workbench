import { requestJSON } from '../shared/api.js';
import { escapeHTML, query, shortenPath, showDialog, showNotice } from '../shared/dom.js';
import { renderIcon } from '../shared/icons.js';
import { workspaceState } from '../shared/state.js';

export function openDiscover() {
  if (!workspaceState.snapshot?.features?.discovery) {
    showNotice(
      'Project discovery is unavailable in this engine. Rebuild Workbench and run ./workbench start.',
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

export function renderDiscovery(discovery) {
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

export function renderDiscoveryCandidate(candidate, index) {
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

export async function discoverProjectsFromForm(event) {
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

export async function chooseDiscoveryFolder() {
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

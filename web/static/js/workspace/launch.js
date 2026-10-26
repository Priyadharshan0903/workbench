import { recordOpen } from '../projects/summary.js';
import { requestJSON } from '../shared/api.js';
import { query, renderDetailField, showDialog, showNotice } from '../shared/dom.js';
import { workspaceState } from '../shared/state.js';

export function createLaunchController({ refresh }) {
  async function prepareLaunch(id) {
    const requestSequence = ++workspaceState.launchRequestSequence;
    const project = workspaceState.snapshot?.projects.find((project) => project.id === id);
    if (!project) return;
    workspaceState.launchPlan = null;
    query('#launch-title').textContent = 'Start ' + project.name + '?';
    query('#launch-description').textContent =
      'Runs in the project folder using the command below.';
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
  return { prepareLaunch, launchReviewedProject, settlePending };
}

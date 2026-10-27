import { requestJSON } from '../shared/api.js';
import { query, renderDetailField, showDialog, showNotice } from '../shared/dom.js';
import { preferences, savePreferences, workspaceState } from '../shared/state.js';

export function createProjectRemoval({ refresh }) {
  let reviewedProject = null;
  let removingProject = false;

  function showRemoval(projectId) {
    const snapshot = workspaceState.snapshot;
    const project = snapshot?.projects.find((project) => project.id === projectId);
    if (!project || projectId === 'workbench' || removingProject) return;

    reviewedProject = { id: project.id, name: project.name, revision: snapshot.revision };
    query('#remove-project-name').textContent = project.name;
    query('#remove-project-body').innerHTML = renderDetailField('Project folder', project.path, true);
    query('#remove-project-running').hidden =
      !project.services.length && project.status !== 'starting';
    query('#remove-project-error').hidden = true;
    showDialog('remove-project-dialog');
    query('#keep-project').focus();
  }

  function forgetProjectPreferences(projectId) {
    preferences.favorites = preferences.favorites.filter((id) => id !== projectId);
    preferences.order = preferences.order.filter((id) => id !== projectId);
    delete preferences.recent[projectId];
    savePreferences();

    const pendingLaunch = workspaceState.pendingLaunches.get(projectId);
    try {
      if (pendingLaunch?.tab?.location.href === 'about:blank') pendingLaunch.tab.close();
    } catch {
      // An already-open project website can be cross-origin; leave it open.
    }
    workspaceState.pendingLaunches.delete(projectId);
  }

  function closeRemovedProjectViews(projectId) {
    if (workspaceState.detailId === projectId) {
      query('#details').close();
      workspaceState.detailId = null;
    }

    const idField = query('#tool-form').elements.namedItem('id');
    if (idField.readOnly && idField.value === projectId) {
      query('#editor').close();
      workspaceState.setupLaunchProjectId = null;
    }
  }

  async function removeReviewedProject() {
    if (!reviewedProject || removingProject) return;

    const project = reviewedProject;
    removingProject = true;
    query('#confirm-remove-project').disabled = true;
    query('#remove-project-error').hidden = true;
    try {
      await requestJSON('/api/project/remove', { id: project.id, revision: project.revision });
      forgetProjectPreferences(project.id);
      query('#remove-project-dialog').close();
      closeRemovedProjectViews(project.id);
      await refresh();
      showNotice(project.name + ' removed from Workbench. Files and running processes were kept.');
    } catch (error) {
      if (query('#remove-project-dialog').open) {
        query('#remove-project-error').textContent = error.message;
        query('#remove-project-error').hidden = false;
      } else {
        showNotice('Could not remove project. ' + error.message);
      }
    } finally {
      removingProject = false;
      query('#confirm-remove-project').disabled = false;
    }
  }

  function bindRemovalEvents() {
    query('#confirm-remove-project').addEventListener('click', removeReviewedProject);
    query('#remove-project-dialog').addEventListener('close', () => {
      reviewedProject = null;
    });
  }

  return { showRemoval, bindRemovalEvents };
}

import { requestJSON } from '../shared/api.js';
import { query, showDialog, showNotice } from '../shared/dom.js';
import { projectFields } from '../shared/project.js';
import { workspaceState } from '../shared/state.js';

export function createProjectEditor({ refresh, prepareLaunch }) {
  function openProjectEditor(id, prefill = null, setupLaunch = false) {
    const project =
      prefill ||
      (id ? workspaceState.snapshot?.projects.find((project) => project.id === id) : null);
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
    const removeButton = query('#remove-project');
    removeButton.hidden = !id || id === 'workbench';
    removeButton.dataset.removeProject = id || '';
    query('#form-error').hidden = true;
    showDialog('editor');
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
        workspaceState.snapshot.projects.some(
          (existingProject) => existingProject.id === project.id,
        )
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
    project.description ||=
      'Local project in ' + project.path.split('/').filter(Boolean).pop() + '.';

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

  function updateSuggestedProjectId(event) {
    if (workspaceState.projectIdEdited) return;

    query('#tool-form').elements.namedItem('id').value = event.target.value
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '')
      .slice(0, 64);
  }
  return {
    openProjectEditor,
    trackServiceAsProject,
    saveProjectFromForm,
    updateSuggestedProjectId,
  };
}

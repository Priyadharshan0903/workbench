import { query as find, showDialog } from '../shared/dom.js';
import { createSkillState } from './state.js';
import { createSkillRenderer } from './views.js';
export function createSkillWorkspace() {
  const state = createSkillState();
  const { render, currentFilePath } = createSkillRenderer(state);
  async function open(metadata, context) {
    state.active = true;
    state.metadata = metadata;
    state.document = null;
    state.instructions = null;
    state.context = context;
    state.dirty = false;
    state.error = '';
    state.message = '';
    state.discardAction = null;
    state.previewHTML = '';
    find('#details').classList.add('skill-drawer');
    find('#detail-eyebrow').textContent = 'Skill workspace';
    showDialog('details');
    await loadDocument(metadata.id);
  }

  async function loadDocument(id, filename = 'SKILL.md') {
    const sequence = ++state.sequence;
    state.loading = true;
    state.error = '';
    render();

    try {
      const parameters = new URLSearchParams({ id, file: filename });
      const document = await state.context.requestJSON('/api/skill?' + parameters);
      if (!state.active || sequence !== state.sequence) return;

      state.document = document;
      state.metadata = document.skill;
      state.mode = filename.toLowerCase().endsWith('.md') ? 'preview' : 'raw';
      if (filename === 'SKILL.md') {
        state.instructions = document;
        state.draft = document.content;
        state.dirty = false;
        state.previewHTML = '';
      }
    } catch (error) {
      if (state.active && sequence === state.sequence) state.error = error.message;
    } finally {
      if (state.active && sequence === state.sequence) {
        state.loading = false;
        render();
      }
    }
  }

  function keepOpenIfDirty() {
    if (!state.active) return false;
    if (state.busy) return true;
    if (!state.dirty) return false;

    state.discardAction = 'close';
    render();
    find('[data-skill-action="keep-draft"]')?.focus();
    return true;
  }

  async function save() {
    if (state.mode !== 'review' || !state.dirty || !state.instructions.editable) return;
    state.busy = true;
    state.error = '';
    render();

    try {
      const result = await state.context.requestJSON('/api/skill/save', {
        id: state.instructions.skill.id,
        revision: state.instructions.revision,
        content: state.draft,
      });
      state.message = 'Saved. Backup: ' + result.backupPath;
      state.dirty = false;
      await state.context.onSaved();
      await loadDocument(state.instructions.skill.id);
    } catch (error) {
      state.error = error.message;
    } finally {
      state.busy = false;
      render();
    }
  }

  async function duplicate(event) {
    event.preventDefault();
    if (state.busy) return;
    state.destination = find('#skill-destination').value;
    state.copyName = find('#skill-copy-name').value;
    state.busy = true;
    state.error = '';
    render();

    try {
      const result = await state.context.requestJSON('/api/skill/duplicate', {
        id: state.instructions.skill.id,
        revision: state.instructions.bundleRevision,
        destination: state.destination,
        name: state.copyName,
      });
      state.message = 'Created an editable copy: ' + result.path;
      await state.context.onSaved();
      await loadDocument(result.id);
    } catch (error) {
      state.error = error.message;
    } finally {
      state.busy = false;
      render();
    }
  }

  async function handleAction(event) {
    const button = event.target.closest('[data-skill-action]');
    if (!state.active || !button || state.busy) return;
    event.preventDefault();
    const action = button.dataset.skillAction;

    if (action === 'reload') return loadDocument(state.metadata.id);
    if (action === 'copy-instructions')
      return state.context.copyText(state.instructions.content, button);
    if (action === 'copy-path') return state.context.copyText(currentFilePath(), button);
    if (action === 'copy-draft') return state.context.copyText(state.draft, button);
    if (action === 'save') return save();

    if (action === 'preview-draft') {
      const sequence = state.sequence;
      const draft = state.draft;
      try {
        const preview = await state.context.requestJSON('/api/skill/preview', { content: draft });
        if (
          !state.active ||
          state.sequence !== sequence ||
          state.draft !== draft ||
          state.mode !== 'edit'
        )
          return;
        state.previewHTML = preview.html;
      } catch (error) {
        state.error = error.message;
      }
    } else if (action === 'edit') {
      if (!state.instructions.editable) return;
      state.mode = 'edit';
    } else if (action === 'review') {
      if (!state.dirty) return;
      state.mode = 'review';
    } else if (action === 'cancel-edit') {
      if (state.dirty) state.discardAction = 'cancel';
      else state.mode = 'preview';
    } else if (action === 'keep-draft') {
      state.discardAction = null;
    } else if (action === 'discard-draft') {
      const close = state.discardAction === 'close';
      state.draft = state.instructions.content;
      state.dirty = false;
      state.discardAction = null;
      state.mode = 'preview';
      if (close) find('#details').close();
    } else if (action === 'duplicate') {
      if (!state.instructions.bundleComplete) return;
      state.mode = 'duplicate';
      state.destination = state.instructions.destinations[0]?.id || '';
      state.copyName =
        (state.metadata.name
          .toLowerCase()
          .replace(/[^a-z0-9]+/g, '-')
          .replace(/^-|-$/g, '')
          .slice(0, 58) || 'skill') + '-copy';
    } else if (action === 'cancel-copy') state.mode = 'preview';
    else if (action === 'preview' || action === 'raw') state.mode = action;

    render();
  }

  function handleInput(event) {
    if (!state.active) return;
    if (event.target.id === 'skill-editor') {
      state.draft = event.target.value;
      state.dirty = state.draft !== state.instructions.content;
      const review = find('[data-skill-action="review"]');
      if (review) review.disabled = !state.dirty;
    }
    if (event.target.id === 'skill-copy-name') state.copyName = event.target.value;
    if (event.target.id === 'skill-destination') state.destination = event.target.value;
    if (['skill-copy-name', 'skill-destination'].includes(event.target.id)) {
      const destination = state.instructions.destinations.find(
        (item) => item.id === state.destination,
      );
      find('#skill-copy-target').textContent = (destination?.path || '') + '/' + state.copyName;
    }
  }
  function bindEvents() {
    document.addEventListener('click', handleAction);
    document.addEventListener('input', handleInput);
    document.addEventListener('change', (event) => {
      handleInput(event);
      if (state.active && event.target.id === 'skill-file' && !state.dirty) {
        loadDocument(state.metadata.id, event.target.value);
      }
    });
    document.addEventListener('submit', (event) => {
      if (event.target.id === 'skill-copy-form') duplicate(event);
    });
    document.addEventListener('click', (event) => {
      const link = event.target.closest('#skill-preview a, #skill-draft-preview a');
      if (!state.active || !link) return;
      event.preventDefault();
      const href = link.getAttribute('href');
      if (!href) return;

      if (/^https?:\/\//i.test(href)) {
        window.open(href, '_blank', 'noopener,noreferrer');
        return;
      }
      if (state.dirty) {
        state.error = 'Finish editing or copy your draft before opening another skill file.';
        render();
        return;
      }
      let filename;
      try {
        filename = decodeURIComponent(
          new URL(href, 'https://skill.local/' + state.document.file).pathname.slice(1),
        );
      } catch {
        state.error = 'This reference has an invalid file address.';
        render();
        return;
      }
      if (state.document.files.some((file) => file.path === filename))
        loadDocument(state.metadata.id, filename);
      else {
        state.error = 'This reference is not included in the skill bundle.';
        render();
      }
    });
    find('#details').addEventListener('cancel', (event) => {
      if (keepOpenIfDirty()) event.preventDefault();
    });
    find('#details').addEventListener('close', () => {
      state.active = false;
      state.sequence++;
      find('#details').classList.remove('skill-drawer');
    });
  }
  return { open, keepOpenIfDirty, bindEvents };
}

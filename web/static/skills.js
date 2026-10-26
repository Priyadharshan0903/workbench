'use strict';

// The skill workspace owns its draft and requests; the dashboard owns discovery.
window.WorkbenchSkills = (() => {
  const find = (selector) => document.querySelector(selector);
  const entities = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
  const escapeHTML = (value) =>
    String(value ?? '').replace(/[&<>"']/g, (character) => entities[character]);
  const state = {
    active: false,
    metadata: null,
    document: null,
    instructions: null,
    mode: 'preview',
    draft: '',
    dirty: false,
    loading: false,
    busy: false,
    sequence: 0,
    error: '',
    message: '',
    discardAction: null,
    previewHTML: '',
    destination: '',
    copyName: '',
    context: null,
  };

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
    if (!find('#details').open) find('#details').showModal();
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

  function actionButton(action, label, primary = false, disabled = false) {
    return `<button type="button" class="button ${primary ? 'primary' : 'quiet'}"
      data-skill-action="${action}" ${disabled || state.busy ? 'disabled' : ''}>${label}</button>`;
  }

  function render() {
    if (!state.active) return;
    const document = state.document;
    const heading = `<h2 id="detail-title">${escapeHTML(state.metadata.name)}</h2>
      <p class="dialog-intro">${escapeHTML(state.metadata.description)}</p>`;
    const notices = `${state.error ? `<p class="notice" role="alert">${escapeHTML(state.error)}</p>` : ''}
      ${state.message ? `<p class="skill-message" role="status">${escapeHTML(state.message)}</p>` : ''}`;

    if (state.loading || !document) {
      find('#detail-body').innerHTML =
        heading +
        notices +
        (state.loading
          ? '<p class="footnote" role="status">Reading skill files…</p>'
          : actionButton('reload', 'Retry'));
      return;
    }

    let content;
    if (state.mode === 'edit') content = renderEditor();
    else if (state.mode === 'review') content = renderReview();
    else if (state.mode === 'duplicate') content = renderCopyForm();
    else content = renderViewer();

    find('#detail-body').innerHTML = `
      ${heading}
      <p class="skill-path"><code>${escapeHTML(currentFilePath())}</code></p>
      ${
        document.skill.locations.length > 1
          ? `<details class="skill-locations"><summary>Shared through ${document.skill.locations.length} locations</summary>
             <p class="footnote">Edits affect the same underlying file.</p>
             <pre>${escapeHTML(document.skill.locations.join('\n'))}</pre></details>`
          : ''
      }
      ${notices}
      ${state.discardAction ? renderDiscardQuestion() : ''}
      ${content}
    `;

    const editor = find('#skill-editor');
    if (editor) editor.value = state.draft;
    const before = find('#skill-before');
    if (before) before.textContent = state.instructions.content;
    const after = find('#skill-after');
    if (after) after.textContent = state.draft;
    const raw = find('#skill-raw');
    if (raw) raw.textContent = document.content;
    const preview = find('#skill-preview');
    if (preview) preview.replaceChildren(safePreview(document.previewHtml));
    const draftPreview = find('#skill-draft-preview');
    if (draftPreview && state.previewHTML)
      draftPreview.replaceChildren(safePreview(state.previewHTML));
  }

  function renderViewer() {
    const document = state.document;
    const isMarkdown = document.file.toLowerCase().endsWith('.md');
    const selected = document.files.find((file) => file.path === document.file);
    const fileOptions = document.files
      .map(
        (file) =>
          `<option value="${escapeHTML(file.path)}" ${file.path === document.file ? 'selected' : ''}>
        ${escapeHTML(file.path)}${file.readable ? '' : ' (binary)'}</option>`,
      )
      .join('');
    const canEdit = document.file === 'SKILL.md' && document.editable;
    const exportURL =
      '/api/skill/export?' +
      new URLSearchParams({
        id: document.skill.id,
        revision: document.bundleRevision,
      });

    return `
      <div class="skill-actions">
        ${canEdit ? actionButton('edit', 'Edit instructions', true) : actionButton('duplicate', 'Make editable copy', true, !document.bundleComplete)}
        ${actionButton('copy-instructions', 'Copy instructions')}
        <details class="skill-copy-menu">
          <summary class="button quiet">Copy & export</summary>
          <div>
            ${actionButton('copy-path', 'Copy file path')}
            ${actionButton('duplicate', 'Duplicate skill', false, !document.bundleComplete)}
            ${document.bundleComplete ? `<a class="button quiet" href="${escapeHTML(exportURL)}" download="skill.zip">Download bundle</a>` : ''}
          </div>
        </details>
      </div>
      ${!document.editable ? `<p class="footnote">${escapeHTML(document.readOnlyReason)}</p>` : ''}
      <div class="skill-toolbar">
        <label for="skill-file">File <select id="skill-file">${fileOptions}</select></label>
        <div class="skill-view-toggle" aria-label="Skill content view">
          <button type="button" data-skill-action="preview" aria-pressed="${state.mode === 'preview'}" ${!isMarkdown ? 'disabled' : ''}>Preview</button>
          <button type="button" data-skill-action="raw" aria-pressed="${state.mode === 'raw'}">Raw text</button>
        </div>
      </div>
      ${
        !selected?.readable
          ? '<div class="skill-content"><p>This supporting file is binary or too large for text preview. Download the bundle to use it.</p></div>'
          : state.mode === 'preview'
            ? '<article id="skill-preview" class="skill-content skill-markdown" aria-label="Skill instructions preview"></article>'
            : '<pre id="skill-raw" class="skill-content skill-raw" aria-label="Raw skill file"></pre>'
      }
      <p class="footnote">${document.files.length} files in this skill. Copying does not activate it in an agent.</p>
      ${renderWarnings()}
    `;
  }

  function renderEditor() {
    return `
      <label class="skill-editor-label" for="skill-editor">SKILL.md instructions</label>
      <textarea id="skill-editor" class="skill-editor" spellcheck="false" maxlength="512000" aria-describedby="skill-edit-help"></textarea>
      <p id="skill-edit-help" class="footnote">Review your changes before saving. A backup is kept for each save.</p>
      <div class="skill-actions">
        ${actionButton('review', 'Review changes', true, !state.dirty)}
        ${actionButton('preview-draft', 'Preview draft')}
        ${actionButton('copy-draft', 'Copy draft')}
        ${actionButton('cancel-edit', 'Cancel edit')}
      </div>
      ${state.previewHTML ? '<article id="skill-draft-preview" class="skill-content skill-markdown" aria-label="Draft Markdown preview"></article>' : ''}
    `;
  }

  function renderReview() {
    return `
      <p class="dialog-intro">Review the opened version and your changes. Saving updates SKILL.md at the path above.</p>
      <div class="skill-review">
        <section><h3>Opened version</h3><pre id="skill-before" class="skill-raw"></pre></section>
        <section><h3>Your changes</h3><pre id="skill-after" class="skill-raw"></pre></section>
      </div>
      <div class="skill-actions">
        ${actionButton('save', state.busy ? 'Saving…' : 'Save changes', true)}
        ${actionButton('edit', 'Back to editing')}
        ${actionButton('copy-draft', 'Copy draft')}
      </div>
    `;
  }

  function renderCopyForm() {
    const document = state.instructions;
    const destinations = document.destinations;
    const selected = destinations.find((destination) => destination.id === state.destination);
    const options = destinations
      .map(
        (destination) =>
          `<option value="${escapeHTML(destination.id)}" ${destination.id === state.destination ? 'selected' : ''}>${escapeHTML(destination.label)}</option>`,
      )
      .join('');

    return `
      <h3 class="skill-section-title">Create an editable copy</h3>
      <p class="footnote">Copies instructions and files inside this skill folder. Existing skills are never replaced.</p>
      <form id="skill-copy-form">
        <label for="skill-destination">Destination<select id="skill-destination" required>${options}</select></label>
        <label for="skill-copy-name">Folder name<input id="skill-copy-name" value="${escapeHTML(state.copyName)}"
          pattern="[a-z0-9][a-z0-9-]{0,63}" maxlength="64" required autocomplete="off"></label>
        <p class="skill-path" id="skill-copy-target">${escapeHTML((selected?.path || '') + '/' + state.copyName)}</p>
        <p class="footnote">${document.files.length} files. Contents and external references stay unchanged.</p>
        ${renderWarnings()}
        <div class="skill-actions">
          <button class="button primary" type="submit" ${state.busy ? 'disabled' : ''}>${state.busy ? 'Creating…' : 'Create copy'}</button>
          ${actionButton('cancel-copy', 'Cancel')}
        </div>
      </form>
    `;
  }

  function renderWarnings() {
    const warnings = state.document.warnings;
    if (!warnings.length) return '';

    return `<details class="skill-exclusions"><summary>Copy exclusions and limits</summary><ul>
      ${warnings.map((warning) => `<li>${escapeHTML(warning)}</li>`).join('')}</ul></details>`;
  }

  function currentFilePath() {
    const document = state.document;
    const folder = document.skill.path.slice(0, document.skill.path.lastIndexOf('/') + 1);
    return folder + document.file;
  }

  function renderDiscardQuestion() {
    return `<div class="notice" role="alert">
      <p>You have unsaved changes. Keep editing or discard this draft?</p>
      <div class="skill-actions">${actionButton('keep-draft', 'Keep editing', true)}${actionButton('discard-draft', 'Discard draft')}</div>
    </div>`;
  }

  function keepOpenIfDirty() {
    if (!state.active) return false;
    if (state.busy) return true;
    if (!state.dirty) return false;

    state.discardAction = 'close';
    render();
    return true;
  }

  function safePreview(html) {
    const template = document.createElement('template');
    template.innerHTML = html || '<p>No instructions yet.</p>';
    const allowed = new Set([
      'P',
      'H1',
      'H2',
      'H3',
      'H4',
      'H5',
      'H6',
      'UL',
      'OL',
      'LI',
      'PRE',
      'CODE',
      'BLOCKQUOTE',
      'A',
      'HR',
      'BR',
      'EM',
      'STRONG',
      'DEL',
      'TABLE',
      'THEAD',
      'TBODY',
      'TR',
      'TH',
      'TD',
    ]);

    for (const element of [...template.content.querySelectorAll('*')]) {
      if (element.tagName === 'IMG') {
        element.replaceWith(document.createTextNode(element.getAttribute('alt') || 'Image'));
        continue;
      }
      if (!allowed.has(element.tagName)) {
        element.remove();
        continue;
      }

      for (const attribute of [...element.attributes]) {
        if (!(element.tagName === 'A' && attribute.name === 'href'))
          element.removeAttribute(attribute.name);
      }
      if (element.tagName === 'A') {
        const href = element.getAttribute('href') || '';
        try {
          const address = new URL(href, 'https://skill.local/');
          if (!['http:', 'https:'].includes(address.protocol)) element.removeAttribute('href');
        } catch {
          element.removeAttribute('href');
        }
      }
    }

    return template.content;
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

  return { open, keepOpenIfDirty };
})();

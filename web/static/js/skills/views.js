import { query as find, escapeHTML } from '../shared/dom.js';
import { safePreview } from './preview.js';
export function createSkillRenderer(state) {
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
  return { render, currentFilePath };
}

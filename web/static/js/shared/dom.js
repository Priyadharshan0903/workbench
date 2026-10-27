export const query = (selector) => document.querySelector(selector);

export const queryAll = (selector) => [...document.querySelectorAll(selector)];

const htmlEntities = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };

export function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>"']/g, (character) => htmlEntities[character]);
}

export function renderStatusBadge(text, statusClass = '') {
  const escapedClass = escapeHTML(statusClass);

  return `
    <span class="badge ${escapedClass}">
      <span class="status-dot ${escapedClass}" aria-hidden="true"></span>
      ${escapeHTML(text)}
    </span>
  `;
}

export const shortenPath = (path) => String(path || '').replace(/^\/Users\/[^/]+/, '~');

export const showNotice = (message) => {
  query('#notice').textContent = message;
  query('#notice').hidden = !message;
};

export { requestDialogClose, showDialog } from './dialogs.js';

export function renderDetailField(label, value, code = false, copy = false) {
  const valueElement = code ? 'code' : 'p';
  const valueClass = code ? 'code-line' : 'detail-value';
  const copyButton =
    copy && value
      ? `<button class="text-button" data-copy="${escapeHTML(value)}">Copy ${escapeHTML(label.toLowerCase())}</button>`
      : '';

  return `
    <div class="detail-block">
      <span class="detail-label">${escapeHTML(label)}</span>
      <${valueElement} class="${valueClass}">${escapeHTML(value || 'Not recorded yet.')}</${valueElement}>
      ${copyButton}
    </div>
  `;
}

export async function copyText(text, button) {
  try {
    await navigator.clipboard.writeText(text);
    const original = button.textContent;
    button.textContent = 'Copied';
    setTimeout(() => (button.textContent = original), 1200);
  } catch {
    showNotice('Clipboard access is unavailable. Select and copy the displayed text.');
  }
}

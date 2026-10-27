import Sortable from '../../vendor/sortable.esm.js';
import { renderTools, saveProjectOrder } from './board.js';
import { query } from '../shared/dom.js';
import { workspaceState } from '../shared/state.js';

const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
let projectSorter;

function announce(message) {
  query('#announcement').textContent = message;
}

function focusHandle(projectId) {
  query(`[data-drag="${CSS.escape(projectId)}"]`)?.focus({ preventScroll: true });
}

function finishDrag() {
  workspaceState.pointerDrag = null;
  document.documentElement.classList.remove('is-sorting-projects');
}

function createProjectSorter() {
  return Sortable.create(query('#tool-list'), {
    draggable: '.tool-card',
    handle: '.drag-handle',
    dataIdAttr: 'data-id',
    animation: reducedMotion.matches ? 0 : 200,
    easing: 'cubic-bezier(0.2, 0.7, 0.2, 1)',
    forceFallback: true,
    fallbackOnBody: true,
    fallbackTolerance: 5,
    ghostClass: 'project-drag-placeholder',
    fallbackClass: 'project-drag-preview',
    chosenClass: 'project-drag-chosen',
    filter: () => workspaceState.activeTab === 'recent',
    preventOnFilter: false,

    onChoose(event) {
      // Pause polling renders before the library starts moving DOM nodes.
      workspaceState.pointerDrag = { id: event.item.dataset.id, active: false };
    },

    onStart() {
      workspaceState.pointerDrag.active = true;
      document.documentElement.classList.add('is-sorting-projects');
      announce('Moving project. Drop to save its position, or press Escape to cancel.');
    },

    onUnchoose() {
      if (!workspaceState.pointerDrag?.active) finishDrag();
    },

    onEnd(event) {
      const drag = workspaceState.pointerDrag;
      if (!drag || drag.cancelled) return;

      const order = projectSorter.toArray();
      saveProjectOrder(order);
      finishDrag();
      renderTools();
      focusHandle(drag.id);
      announce(`Project moved to position ${event.newDraggableIndex + 1} of ${order.length}.`);
    },
  });
}

function cancelProjectDrag() {
  const drag = workspaceState.pointerDrag;
  if (!drag) return;

  // Destroy cancels listeners without firing a drop; discard the fallback preview too.
  drag.cancelled = true;
  projectSorter.destroy();
  document.querySelectorAll('.project-drag-preview').forEach((preview) => preview.remove());
  finishDrag();
  renderTools();
  projectSorter = createProjectSorter();
  focusHandle(drag.id);
  announce('Move cancelled. Project order was kept.');
}

function handleProjectMoveKeydown(event) {
  if (event.key === 'Escape' && workspaceState.pointerDrag) {
    event.preventDefault();
    cancelProjectDrag();
    return;
  }

  const handle = event.target.closest('[data-drag]');
  const directions = ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'];
  if (!handle || !directions.includes(event.key) || workspaceState.pointerDrag) return;
  if (workspaceState.activeTab === 'recent') return;

  event.preventDefault();
  const grid = query('#tool-list');
  const order = projectSorter.toArray();
  const columns = getComputedStyle(grid).gridTemplateColumns.split(' ').length;
  const distance = ['ArrowUp', 'ArrowDown'].includes(event.key) ? columns : 1;
  const direction = ['ArrowRight', 'ArrowDown'].includes(event.key) ? 1 : -1;
  const previousIndex = order.indexOf(handle.dataset.drag);
  const nextIndex = previousIndex + direction * distance;
  if (nextIndex < 0 || nextIndex >= order.length) return;

  const previousPositions = new Map(
    [...grid.children].map((card) => [card.dataset.id, card.getBoundingClientRect()]),
  );
  order.splice(previousIndex, 1);
  order.splice(nextIndex, 0, handle.dataset.drag);
  saveProjectOrder(order);
  renderTools();

  if (!reducedMotion.matches) {
    for (const card of grid.children) {
      const previous = previousPositions.get(card.dataset.id);
      const current = card.getBoundingClientRect();
      if (!previous) continue;

      card.animate(
        [
          { transform: `translate(${previous.left - current.left}px, ${previous.top - current.top}px)` },
          { transform: 'translate(0, 0)' },
        ],
        { duration: 200, easing: 'cubic-bezier(0.2, 0.7, 0.2, 1)' },
      );
    }
  }

  focusHandle(handle.dataset.drag);
  announce(`Project moved to position ${nextIndex + 1} of ${order.length}.`);
}

export function bindArrangementEvents() {
  projectSorter = createProjectSorter();
  document.addEventListener('keydown', handleProjectMoveKeydown);
  window.addEventListener('blur', cancelProjectDrag);
  window.addEventListener('resize', cancelProjectDrag);
  window.addEventListener('hashchange', cancelProjectDrag);
  document.addEventListener('pointercancel', cancelProjectDrag);
  document.addEventListener('touchcancel', cancelProjectDrag);
  reducedMotion.addEventListener('change', () => {
    projectSorter.option('animation', reducedMotion.matches ? 0 : 200);
  });
}

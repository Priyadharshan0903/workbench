import { reorderProject } from './board.js';
import { orderedProjects } from './model.js';
import { query, queryAll } from '../shared/dom.js';
import { workspaceState } from '../shared/state.js';

export function handleProjectDragStart(event) {
  const handle = event.target.closest('[data-drag]');
  if (!handle) return;
  workspaceState.draggedProjectId = handle.dataset.drag;
  event.dataTransfer?.setData('text/plain', workspaceState.draggedProjectId);
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
  handle.closest('.tool-card').classList.add('dragging');
}

export function handleProjectDragOver(event) {
  if (!workspaceState.draggedProjectId) return;
  const card = event.target.closest('.tool-card');
  if (card) {
    event.preventDefault();
    queryAll('.drop-target').forEach((card) => card.classList.remove('drop-target'));
    card.classList.add('drop-target');
  }
}

export function handleProjectDrop(event) {
  if (!workspaceState.draggedProjectId) return;
  event.preventDefault();
  const target = event.target.closest('.tool-card');
  if (target) reorderProject(workspaceState.draggedProjectId, target.dataset.id);
  workspaceState.draggedProjectId = null;
}

export function clearProjectDrag() {
  workspaceState.draggedProjectId = null;
  queryAll('.dragging,.drop-target').forEach((card) =>
    card.classList.remove('dragging', 'drop-target'),
  );
}

export function handleProjectMoveKeydown(event) {
  const handle = event.target.closest('[data-drag]');
  if (!handle || !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return;
  event.preventDefault();
  const order = orderedProjects();
  const index = order.findIndex((project) => project.id === handle.dataset.drag);
  const next = index + (['ArrowRight', 'ArrowDown'].includes(event.key) ? 1 : -1);
  if (order[next]) {
    reorderProject(handle.dataset.drag, order[next].id);
    query(`[data-drag="${handle.dataset.drag}"]`)?.focus();
  }
}

export function handleProjectPointerDown(event) {
  if (event.pointerType === 'mouse') return;
  const handle = event.target.closest('[data-drag]');
  if (handle)
    workspaceState.pointerDrag = { id: handle.dataset.drag, x: event.clientX, y: event.clientY };
}

export function handleProjectPointerUp(event) {
  if (!workspaceState.pointerDrag) return;
  const target = document.elementFromPoint(event.clientX, event.clientY)?.closest('.tool-card');
  if (
    target &&
    Math.hypot(
      event.clientX - workspaceState.pointerDrag.x,
      event.clientY - workspaceState.pointerDrag.y,
    ) > 15
  )
    reorderProject(workspaceState.pointerDrag.id, target.dataset.id);
  workspaceState.pointerDrag = null;
}

export function clearProjectPointerDrag() {
  workspaceState.pointerDrag = null;
}

export function bindArrangementEvents() {
  const projectList = query('#tool-list');
  projectList.addEventListener('dragstart', handleProjectDragStart);
  projectList.addEventListener('dragover', handleProjectDragOver);
  projectList.addEventListener('drop', handleProjectDrop);
  projectList.addEventListener('dragend', clearProjectDrag);
  projectList.addEventListener('keydown', handleProjectMoveKeydown);
  projectList.addEventListener('pointerdown', handleProjectPointerDown);
  projectList.addEventListener('pointerup', handleProjectPointerUp);
  projectList.addEventListener('pointercancel', clearProjectPointerDrag);
}

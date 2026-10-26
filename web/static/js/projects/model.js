import { preferences, workspaceState } from '../shared/state.js';

export const workspaceProjects = () =>
  workspaceState.snapshot?.projects.filter((project) => project.id !== 'workbench') || [];

export function orderedProjects() {
  const projectRanks = new Map(preferences.order.map((id, index) => [id, index]));
  return [...workspaceProjects()].sort(
    (firstProject, secondProject) =>
      (projectRanks.get(firstProject.id) ?? 10000) - (projectRanks.get(secondProject.id) ?? 10000),
  );
}

export function isFavorite(id) {
  return preferences.favorites.includes(id);
}

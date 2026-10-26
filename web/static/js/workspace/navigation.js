import { renderTools } from '../projects/board.js';
import { showProject } from '../projects/details.js';
import { renderResume } from '../projects/summary.js';
import { setSidebar, toggleTheme } from '../shared/appearance.js';
import { query, queryAll } from '../shared/dom.js';
import { workspaceState } from '../shared/state.js';
import { loadSkills } from '../skills/catalog.js';
import { exportRegistry, showAgentGuide } from './registry.js';

export function createNavigation({ refresh }) {
  function setTab() {
    const tab = location.hash.slice(1);
    workspaceState.activeTab = ['tools', 'favorites', 'recent', 'services', 'skills'].includes(tab)
      ? tab
      : 'tools';
    const isProjectTab = ['tools', 'favorites', 'recent'].includes(workspaceState.activeTab);
    queryAll('.nav').forEach((element) => {
      const active = element.dataset.tab === workspaceState.activeTab;
      element.classList.toggle('active', active);
      if (active) element.setAttribute('aria-current', 'page');
      else element.removeAttribute('aria-current');
    });
    query('#tools-view').hidden = !isProjectTab;
    query('#workspace-summary').hidden = !isProjectTab;
    query('#services-view').hidden = workspaceState.activeTab !== 'services';
    query('#skills-view').hidden = workspaceState.activeTab !== 'skills';
    const titles = {
      tools: ['Projects', 'Projects', 'Your local work, ready to pick up.'],
      favorites: ['Favourites', 'Your go-to projects', 'The projects you want within reach.'],
      recent: [
        'Recently opened',
        'Pick up where you left off',
        'A way back into the work you opened here.',
      ],
      services: [
        'Diagnostics',
        'Workspace diagnostics',
        'Real listeners, project ownership, and the details to resolve a problem.',
      ],
      skills: [
        'Agent skills',
        'Your installed skills',
        'Know-how from Codex, Claude, and your project folders.',
      ],
    };
    const [breadcrumbTitle, title, subtitle] = titles[workspaceState.activeTab];
    query('#breadcrumb').textContent = breadcrumbTitle;
    query('#page-title').textContent = title;
    query('#page-subtitle').textContent = subtitle;
    query('#tools-title').textContent =
      workspaceState.activeTab === 'recent'
        ? 'Recent projects'
        : workspaceState.activeTab === 'favorites'
          ? 'Favourite projects'
          : 'Project workspace';
    setSidebar(false);
    renderTools();
    if (workspaceState.activeTab === 'skills' && !workspaceState.skills) loadSkills();
    renderResume();
  }

  function handleProjectTabKeydown(event) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    const tabs = queryAll('.project-tab');
    const index = tabs.indexOf(event.target);
    if (index < 0) return;
    event.preventDefault();
    const next =
      event.key === 'Home'
        ? 0
        : event.key === 'End'
          ? tabs.length - 1
          : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length;
    tabs[next].click();
    tabs[next].focus();
  }

  function handleSidebarEscape(event) {
    if (event.key === 'Escape' && query('#sidebar').classList.contains('mobile-open')) {
      setSidebar(false);
      query('#menu-toggle').focus();
    }
  }

  function handleNavigationChange() {
    workspaceState.projectFilter = 'all';
    setTab();
  }

  function handleSearchShortcut(event) {
    if (
      event.key === '/' &&
      !event.metaKey &&
      !event.ctrlKey &&
      !event.altKey &&
      !['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement.tagName) &&
      !queryAll('dialog[open]').length
    ) {
      event.preventDefault();
      query(
        { services: '#service-search', skills: '#skill-search' }[workspaceState.activeTab] ||
          '#tool-search',
      ).focus();
    }
  }

  function bindNavigationEvents() {
    query('#theme-toggle').addEventListener('click', toggleTheme);
    query('#menu-toggle').addEventListener('click', () => {
      setSidebar(!query('#sidebar').classList.contains('mobile-open'));
    });
    query('#sidebar-close').addEventListener('click', () => {
      setSidebar(false);
      query('#menu-toggle').focus();
    });

    query('#engine-details').addEventListener('click', () => showProject('workbench'));
    query('#refresh').addEventListener('click', () => refresh(true));
    query('#export').addEventListener('click', exportRegistry);
    query('#agent-guide').addEventListener('click', showAgentGuide);

    query('.project-tabs').addEventListener('keydown', handleProjectTabKeydown);
    document.addEventListener('keydown', handleSidebarEscape);
    document.addEventListener('keydown', handleSearchShortcut);
    window.addEventListener('hashchange', handleNavigationChange);
  }
  return { setTab, bindNavigationEvents };
}

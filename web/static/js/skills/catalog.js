import { renderSummary } from '../projects/summary.js';
import { requestJSON } from '../shared/api.js';
import {
  copyText,
  escapeHTML,
  query,
  renderDetailField,
  shortenPath,
  showDialog,
  showNotice,
} from '../shared/dom.js';
import { renderIcon } from '../shared/icons.js';
import { workspaceState } from '../shared/state.js';
import { createSkillWorkspace } from '../../skills.js';

const skillWorkspace = createSkillWorkspace();

export function renderSkills() {
  if (!workspaceState.skills) return;
  const search = query('#skill-search').value.trim().toLowerCase();
  const provider = query('#skill-provider').value;
  const scope = query('#skill-scope').value;
  const nonProjectScopes = new Set(['Personal', 'Shared', 'Plugin cache', 'Synced plugin']);
  const skills = workspaceState.skills.skills.filter(
    (skill) =>
      (!provider || skill.providers.includes(provider)) &&
      (!scope ||
        (scope === 'project'
          ? skill.scopes.some((entry) => !nonProjectScopes.has(entry))
          : skill.scopes.includes(scope))) &&
      [skill.name, skill.description, skill.path, ...skill.providers]
        .join(' ')
        .toLowerCase()
        .includes(search),
  );
  query('#skills-count').textContent = skills.length;
  query('#skill-list').innerHTML = skills.length
    ? skills.map(renderSkillCard).join('')
    : '<div class="empty">No skills match these filters.</div>';
}

export function renderSkillCard(skill) {
  const providerTags = skill.providers
    .map((provider) => {
      const label = provider === 'codex' ? 'Codex' : 'Claude';
      return `<span class="provider-tag">${label}</span>`;
    })
    .join('');

  return `
    <button class="skill-card" data-skill="${escapeHTML(skill.id)}">
      <span class="skill-badges">${providerTags}</span>
      <h3>${escapeHTML(skill.name)}</h3>
      <p>${escapeHTML(skill.description)}</p>
      <span class="skill-location">${escapeHTML(skill.scopes.join(' · '))}${renderIcon('arrow')}</span>
    </button>
  `;
}

export function showSkill(id) {
  const skill = workspaceState.skills?.skills.find((skill) => skill.id === id);
  if (!skill) return;
  workspaceState.detailId = null;
  workspaceState.detailMode = 'skill';
  if (workspaceState.snapshot?.features?.skillFiles) {
    skillWorkspace.open(skill, {
      requestJSON,
      copyText,
      onSaved: () => loadSkills(true),
    });
    return;
  }
  query('#detail-eyebrow').textContent = 'Installed skill';
  query('#detail-body').innerHTML = `<h2 id="detail-title">${escapeHTML(skill.name)}</h2>
<p class="dialog-intro">${escapeHTML(skill.description)}</p>
${renderDetailField('Skill file', skill.path, true, true)}
${renderDetailField('Found through', skill.locations.map(shortenPath).join('\n'), true)}
${renderDetailField('Location', skill.scopes.join(' · '))}
<p class="footnote">${escapeHTML(workspaceState.skills.note)}</p>`;
  showDialog('details');
}

export async function loadSkills(force = false) {
  try {
    workspaceState.skills = await requestJSON('/api/skills' + (force ? '?refresh=1' : ''));
    renderSkills();
    renderSummary();
  } catch (error) {
    query('#skill-list').innerHTML =
      '<div class="empty">Could not discover skills. Use Refresh to try again.</div>';
    showNotice(error.message);
  }
}

export function bindSkillEvents() {
  skillWorkspace.bindEvents();
}

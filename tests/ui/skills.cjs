const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { JSDOM } = require(process.env.JSDOM_MODULE || 'jsdom');

const root = path.resolve(__dirname, '../..');
const calls = [];
const clipboard = [];
const original = '# Instructions\n\n<script>example source</script>\n';
const personal = {
  id: 'a'.repeat(24),
  name: 'Personal <img src=x>',
  description: 'Maintain local instructions.',
  path: '/home/test/.agents/skills/personal/SKILL.md',
  providers: ['codex'],
  scopes: ['Personal'],
  locations: [
    '/home/test/.agents/skills/personal/SKILL.md',
    '/home/test/.codex/skills/personal/SKILL.md',
  ],
};
const managed = {
  ...personal,
  id: 'b'.repeat(24),
  name: 'Managed skill',
  scopes: ['Plugin cache'],
  path: '/home/test/.codex/plugins/cache/plugin/skills/managed/SKILL.md',
  locations: [],
};
const contents = new Map([
  [personal.id, original],
  [managed.id, original],
]);
const metadata = [personal, managed];
const versions = new Map([
  [personal.id, 'v1'],
  [managed.id, 'v1'],
]);
const state = {
  projects: [],
  services: [],
  revision: 1,
  inspectionError: null,
  scannedAt: new Date().toISOString(),
  registryPath: '/tmp/workbench/registry.json',
  csrfToken: 'fixture-token',
  discoveryRoots: [],
  controlWarnings: [],
  features: { launch: true, discovery: true, logs: true, skillFiles: true },
};
let staleSave = true;
let delayPersonal = false;
let releasePersonal;

function documentFor(id, filename = 'SKILL.md') {
  const skill = metadata.find((item) => item.id === id);
  return {
    skill,
    file: filename,
    content: filename === 'SKILL.md' ? contents.get(id) : '# Reference\n\nSupporting material.\n',
    previewHtml:
      filename === 'SKILL.md'
        ? '<h1>Instructions</h1><script>window.injected=true</script><img src="https://example.com/image" onerror="window.injected=true"><a href="javascript:alert(1)">Unsafe</a><a href="references/guide.md">Reference</a>'
        : '<h1>Reference</h1><p>Supporting material.</p>',
    revision: versions.get(id),
    bundleRevision: 'bundle-' + versions.get(id),
    editable: id !== managed.id && filename === 'SKILL.md',
    readOnlyReason: 'Managed originals are read-only; make an editable copy.',
    files: [
      { path: 'SKILL.md', size: 120, readable: true },
      { path: 'references/guide.md', size: 40, readable: true },
    ],
    destinations: [
      { id: 'destination', label: 'Personal · Codex', path: '/home/test/.codex/skills' },
    ],
    warnings: ['Excluded from copies: .env'],
    bundleComplete: true,
  };
}

const dom = new JSDOM(fs.readFileSync(path.join(root, 'web/static/index.html'), 'utf8'), {
  url: 'http://127.0.0.1:8732/#skills',
  runScripts: 'outside-only',
  pretendToBeVisual: true,
});
const window = dom.window;
const query = (selector) => window.document.querySelector(selector);
window.HTMLDialogElement.prototype.showModal = function () {
  this.setAttribute('open', '');
};
window.HTMLDialogElement.prototype.close = function () {
  this.removeAttribute('open');
  this.dispatchEvent(new window.Event('close'));
};
window.navigator.clipboard = { writeText: async (text) => clipboard.push(text) };
window.open = () => null;
const reply = (payload, ok = true) => ({ ok, json: async () => structuredClone(payload) });

window.fetch = async (address, options) => {
  calls.push({ address, options });
  const url = new URL(address, window.location.href);
  if (url.pathname === '/api/state') return reply(state);
  if (url.pathname === '/api/skills')
    return reply({ skills: metadata, note: 'Discovery is not activation.' });
  if (url.pathname === '/api/skill') {
    const id = url.searchParams.get('id');
    const document = documentFor(id, url.searchParams.get('file'));
    if (delayPersonal && id === personal.id) {
      delayPersonal = false;
      return new Promise((resolve) => {
        releasePersonal = () => resolve(reply(document));
      });
    }
    return reply(document);
  }
  const body = JSON.parse(options.body);
  assert.equal(options.headers['X-Workbench-Token'], 'fixture-token');
  if (url.pathname === '/api/skill/preview') {
    return reply({ html: '<p>Draft preview</p><a href="references/guide.md">Reference</a>' });
  }
  if (url.pathname === '/api/skill/save') {
    if (staleSave)
      return reply({ error: 'The skill changed on disk. Your draft is still available.' }, false);
    assert.equal(body.revision, versions.get(body.id));
    contents.set(body.id, body.content);
    versions.set(body.id, 'v2');
    return reply({
      ok: true,
      revision: 'v2',
      backupPath: '/tmp/workbench/.runtime/skill-backups/snapshot.md',
    });
  }
  if (url.pathname === '/api/skill/duplicate') {
    assert.equal(body.id, managed.id);
    assert.equal(body.destination, 'destination');
    const copy = {
      ...personal,
      id: 'c'.repeat(24),
      name: 'Editable copy',
      path: '/home/test/.codex/skills/' + body.name + '/SKILL.md',
      locations: [],
    };
    metadata.push(copy);
    contents.set(copy.id, contents.get(managed.id));
    versions.set(copy.id, 'v1');
    return reply({ ok: true, id: copy.id, path: copy.path.replace('/SKILL.md', '') });
  }
  throw new Error('Unexpected request ' + address);
};

function action(name) {
  return query(`[data-skill-action="${name}"]`);
}
function change(selector, value, type = 'input') {
  const element = query(selector);
  element.value = value;
  element.dispatchEvent(new window.Event(type, { bubbles: true }));
}
async function waitFor(predicate, label) {
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline) {
    if (predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  throw new Error('Timed out: ' + label);
}

(async () => {
  try {
    for (const filename of ['skills.js', 'app.js']) {
      window.eval(fs.readFileSync(path.join(root, 'web/static', filename), 'utf8'));
    }
    await waitFor(
      () => query('.skill-card') && query('#engine-label').textContent === 'Engine connected',
      'skill discovery',
    );
    query(`[data-skill="${personal.id}"]`).click();
    await waitFor(() => query('#skill-preview'), 'skill viewer');
    assert.ok(query('#details').classList.contains('skill-drawer'));
    assert.equal(query('#detail-body img'), null);
    assert.equal(query('#detail-body script'), null);
    assert.equal(query('#skill-preview a').getAttribute('href'), null);
    assert.equal(window.injected, undefined);
    action('copy-instructions').click();
    await waitFor(() => clipboard.length === 1, 'copy instructions');
    assert.equal(clipboard[0], original);
    action('copy-path').click();
    await waitFor(() => clipboard.length === 2, 'copy path');
    assert.equal(clipboard[1], personal.path);
    action('raw').click();
    assert.equal(query('#skill-raw').textContent, original);
    action('preview').click();
    query('#skill-preview a[href="references/guide.md"]').click();
    await waitFor(
      () => query('#skill-file')?.value === 'references/guide.md',
      'supporting reference',
    );
    assert.ok(query('#skill-preview').textContent.includes('Supporting material'));
    change('#skill-file', 'SKILL.md', 'change');
    await waitFor(() => action('edit'), 'return to instructions');

    action('edit').click();
    assert.equal(action('review').disabled, true);
    const draft = '# Updated instructions\n\n</textarea><script>example source</script>\n';
    change('#skill-editor', draft);
    query('[data-close="details"]').click();
    assert.ok(query('#details').open);
    assert.ok(action('discard-draft'));
    action('keep-draft').click();
    assert.equal(query('#skill-editor').value, draft);
    const cancel = new window.Event('cancel', { cancelable: true });
    query('#details').dispatchEvent(cancel);
    assert.equal(cancel.defaultPrevented, true);
    action('keep-draft').click();
    action('preview-draft').click();
    await waitFor(() => query('#skill-draft-preview'), 'draft preview');
    query('#skill-draft-preview a').click();
    assert.equal(
      query('#skill-editor').value,
      draft,
      'reference navigation lost the unsaved draft',
    );
    action('review').click();
    assert.equal(query('#skill-before').textContent, original);
    assert.equal(query('#skill-after').textContent, draft);
    assert.equal(query('#skill-after script'), null);
    assert.equal(calls.filter((call) => call.address === '/api/skill/save').length, 0);
    action('save').click();
    await waitFor(
      () => query('[role="alert"]')?.textContent.includes('changed on disk'),
      'stale write error',
    );
    assert.equal(contents.get(personal.id), original);
    assert.equal(query('#skill-after').textContent, draft);
    staleSave = false;
    action('save').click();
    await waitFor(
      () => query('#skill-preview') && query('.skill-message')?.textContent.includes('Backup:'),
      'explicit save',
    );
    assert.equal(contents.get(personal.id), draft);
    query('[data-close="details"]').click();
    assert.equal(query('#details').classList.contains('skill-drawer'), false);

    query(`[data-skill="${managed.id}"]`).click();
    await waitFor(() => action('duplicate') && query('#skill-preview'), 'managed skill');
    assert.equal(action('edit'), null);
    action('duplicate').click();
    change('#skill-copy-name', 'reviewed-copy');
    assert.ok(query('#skill-copy-target').textContent.endsWith('/reviewed-copy'));
    assert.equal(calls.filter((call) => call.address === '/api/skill/duplicate').length, 0);
    query('#skill-copy-form').dispatchEvent(
      new window.Event('submit', { bubbles: true, cancelable: true }),
    );
    await waitFor(
      () => action('edit') && query('#detail-title').textContent === 'Editable copy',
      'new editable copy',
    );
    assert.equal(contents.get(managed.id), original);
    query('[data-close="details"]').click();

    delayPersonal = true;
    query(`[data-skill="${personal.id}"]`).click();
    await waitFor(() => releasePersonal, 'pending viewer request');
    query(`[data-skill="${managed.id}"]`).click();
    await waitFor(
      () => query('#skill-preview') && query('#detail-title').textContent === managed.name,
      'newer selection',
    );
    releasePersonal();
    await new Promise((resolve) => setTimeout(resolve, 25));
    assert.equal(
      query('#detail-title').textContent,
      managed.name,
      'an old response replaced the selected skill',
    );
    console.log(
      'Skills checks passed: safe preview/raw text, references, clipboard, drafts, explicit save, stale writes, managed originals, reviewed copies and stale-response protection.',
    );
  } finally {
    window.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

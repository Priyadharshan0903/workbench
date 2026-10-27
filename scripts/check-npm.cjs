const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { digest } = require('./build-npm.cjs');
const { platformFor } = require('../npm/platforms.cjs');
const packageInfo = require('../package.json');

function main() {
  const artifacts = path.resolve(__dirname, '../dist/npm');
  const manifest = JSON.parse(fs.readFileSync(path.join(artifacts, 'release-manifest.json'), 'utf8'));
  assert.equal(manifest.name, packageInfo.name);
  assert.equal(manifest.version, packageInfo.version);
  for (const entry of manifest.files) {
    assert.equal(entry.file, path.basename(entry.file));
    assert.equal(digest(path.join(artifacts, entry.file)), entry.sha256, entry.file);
  }
  const platform = platformFor(process.platform, process.arch);
  const native = manifest.files.find((entry) => entry.kind === 'platform' && entry.target === platform.id);
  const launcher = manifest.files.find((entry) => entry.kind === 'launcher');
  assert.ok(native && launcher, 'The current platform and launcher archives are required.');
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'workbench-installed-check-'));
  const data = path.join(temporary, 'persistent data');
  const installation = path.join(temporary, 'installation');
  const environment = {
    ...process.env,
    PATH: path.dirname(process.execPath) + path.delimiter + process.env.PATH,
    WORKBENCH_HOME: data,
  };
  function run(command, args, options = {}) {
    const result = spawnSync(command, args, {
      env: environment, encoding: 'utf8', timeout: 60000, ...options,
    });
    assert.ifError(result.error);
    assert.equal(result.status, 0, result.stderr || result.stdout);
    return result.stdout;
  }

  try {
    for (const entry of manifest.files.filter((file) => file.file.endsWith('.tgz'))) {
      const filenames = run('tar', ['-tzf', path.join(artifacts, entry.file)]).trim().split('\n');
      const allowed = entry.kind === 'launcher'
        ? /^package\/(?:package\.json|README\.md|LICENSE|npm\/(?:cli|runtime|platforms)\.cjs)$/
        : /^package\/(?:package\.json|README\.md|LICENSE|bin\/(?:workbench|SHA256SUM)|licenses\/(?:cobra|pflag|mousetrap|goldmark|htmx|inter)\.txt)$/;
      for (const filename of filenames.filter((name) => !name.endsWith('/'))) {
        assert.match(filename, allowed, `Unexpected public package file in ${entry.file}`);
      }
    }
    // Local tarballs satisfy the native dependency; omitted platforms need no network.
    run('npm', [
      'install', '--prefix', installation, '--offline', '--ignore-scripts', '--no-audit', '--no-fund',
      '--cache', path.join(temporary, 'npm-cache'),
      path.join(artifacts, native.file), path.join(artifacts, launcher.file),
    ]);
    const installedPackage = path.join(installation, 'node_modules', ...packageInfo.name.split('/'));
    const cli = path.join(installedPackage, 'npm', 'cli.cjs');
    const installedManifest = JSON.parse(fs.readFileSync(path.join(installedPackage, 'package.json'), 'utf8'));
    assert.equal(installedManifest.private, undefined);
    assert.equal(Object.keys(installedManifest.optionalDependencies).length, 4);
    assert.equal(installedManifest.scripts, undefined, 'The public package must not run install hooks.');
    assert.match(run(process.execPath, [cli, '--version']), new RegExp(packageInfo.version.replaceAll('.', '\\.')));
    assert.match(run(process.execPath, [cli, '--help']), /Manage your local project workspace/);
    assert.equal(fs.existsSync(path.join(data, 'registry.json')), false, 'Help/version must not initialize a catalog.');

    fs.writeFileSync(path.join(data, 'registry.json'), JSON.stringify({ version: 1, revision: 1, projects: [] }));
    const protocol = [
      { jsonrpc: '2.0', id: 1, method: 'initialize', params: { protocolVersion: '2025-11-25' } },
      { jsonrpc: '2.0', method: 'notifications/initialized' },
      { jsonrpc: '2.0', id: 2, method: 'tools/list', params: {} },
    ].map((message) => JSON.stringify(message)).join('\n') + '\n';
    const messages = run(process.execPath, [cli, 'mcp'], { input: protocol })
      .trim().split('\n').map((line) => JSON.parse(line));
    assert.ok(messages.some((message) => message.id === 2 && message.result.tools.some((tool) => tool.name === 'list_projects')));
    const customRoot = path.join(temporary, 'custom catalog');
    fs.mkdirSync(customRoot);
    fs.copyFileSync(path.join(data, 'registry.json'), path.join(customRoot, 'registry.json'));
    assert.equal(JSON.parse(run(process.execPath, [cli, '--root', customRoot, 'list', '--json'])).revision, 1);

    fs.rmSync(installation, { recursive: true });
    assert.match(run(path.join(data, 'workbench'), ['--version']), /workbench version/);
    assert.equal(JSON.parse(fs.readFileSync(path.join(data, 'registry.json'), 'utf8')).revision, 1);
    console.log(`Installed-package checks passed for ${platform.id}: version, help, MCP, custom catalog and cache removal.`);
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true });
  }
}

try { main(); } catch (error) {
  console.error(error.stack);
  process.exitCode = 1;
}

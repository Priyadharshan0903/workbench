const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');
const { once } = require('node:events');
const { test } = require('node:test');
const { dataDirectory, fileDigest, installedBinary, nativeArguments } = require('./runtime.cjs');
const { platformFor, platformPackageName } = require('./platforms.cjs');
const packageInfo = require('../package.json');

function fixture(t, content = 'first binary') {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'workbench-npm-test-'));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  const native = path.join(directory, 'native');
  const home = path.join(directory, 'home');
  fs.mkdirSync(path.join(native, 'bin'), { recursive: true });
  const metadata = { name: packageInfo.name, version: packageInfo.version };
  const platform = platformFor('darwin', 'arm64');
  const manifest = path.join(native, 'package.json');
  fs.writeFileSync(manifest, JSON.stringify({
    name: platformPackageName(metadata.name, platform), version: metadata.version,
  }));
  const source = path.join(native, 'bin', 'workbench');
  function replaceSource(value) {
    fs.writeFileSync(source, value);
    fs.writeFileSync(path.join(native, 'bin', 'SHA256SUM'), fileDigest(source) + '\n');
  }
  replaceSource(content);
  const options = { platform: 'darwin', arch: 'arm64', home, env: {}, resolvePackage: () => manifest };
  return { directory, native, source, metadata, manifest, options, replaceSource };
}

test('uses stable user directories rather than the npm cache or current project', () => {
  assert.equal(dataDirectory({ platform: 'darwin', home: '/users/example', env: {} }), '/users/example/Library/Application Support/Workbench');
  assert.equal(dataDirectory({ platform: 'linux', home: '/home/example', env: {} }), '/home/example/.local/share/workbench');
  assert.equal(dataDirectory({ platform: 'linux', home: '/home/example', env: { XDG_DATA_HOME: '/data' } }), '/data/workbench');
  assert.equal(dataDirectory({ platform: 'linux', home: '/home/example', env: { XDG_DATA_HOME: 'relative' } }), '/home/example/.local/share/workbench');
  assert.equal(dataDirectory({ env: { WORKBENCH_HOME: '/custom/workspace' } }), '/custom/workspace');
  assert.throws(() => platformFor('win32', 'x64'), /supports macOS and Linux/);
});

test('installs an executable once and repairs corrupted cached binaries', (t) => {
  const setup = fixture(t);
  const installed = installedBinary(setup.metadata, setup.options);
  const firstInode = fs.statSync(installed.executable).ino;
  assert.equal(fs.readFileSync(installed.executable, 'utf8'), 'first binary');
  assert.equal(fs.statSync(installed.executable).mode & 0o777, 0o755);
  assert.deepEqual(installedBinary(setup.metadata, setup.options), installed);
  assert.equal(fs.statSync(installed.executable).ino, firstInode);
  fs.writeFileSync(installed.executable, 'corrupted');
  installedBinary(setup.metadata, setup.options);
  assert.equal(fs.readFileSync(installed.executable, 'utf8'), 'first binary');
});

test('upgrades the stable binary atomically and preserves catalog and open files', (t) => {
  const setup = fixture(t);
  const first = installedBinary(setup.metadata, setup.options);
  const catalog = path.join(first.root, 'registry.json');
  fs.writeFileSync(catalog, 'personal catalog');
  const opened = fs.openSync(first.executable, 'r');
  try {
    setup.replaceSource('second binary');
    const upgraded = installedBinary(setup.metadata, setup.options);
    assert.equal(upgraded.executable, first.executable);
    assert.equal(fs.readFileSync(upgraded.executable, 'utf8'), 'second binary');
    assert.equal(fs.readFileSync(opened, 'utf8'), 'first binary');
    assert.equal(fs.readFileSync(catalog, 'utf8'), 'personal catalog');
  } finally { fs.closeSync(opened); }
  fs.rmSync(setup.native, { recursive: true });
  assert.equal(fs.readFileSync(first.executable, 'utf8'), 'second binary');
});

test('rejects missing, mismatched and modified native packages before installation', (t) => {
  const setup = fixture(t);
  assert.throws(() => installedBinary(setup.metadata, { ...setup.options, resolvePackage: () => {
    const error = new Error('missing'); error.code = 'MODULE_NOT_FOUND'; throw error;
  } }), /optional dependencies enabled/);
  fs.writeFileSync(setup.manifest, JSON.stringify({ name: 'wrong-package', version: setup.metadata.version }));
  assert.throws(() => installedBinary(setup.metadata, setup.options), /must match/);
  fs.writeFileSync(setup.manifest, JSON.stringify({ name: platformPackageName(setup.metadata.name, platformFor('darwin', 'arm64')), version: '9.9.9' }));
  assert.throws(() => installedBinary(setup.metadata, setup.options), /must match/);
  fs.writeFileSync(setup.manifest, JSON.stringify({ name: platformPackageName(setup.metadata.name, platformFor('darwin', 'arm64')), version: setup.metadata.version }));
  fs.writeFileSync(setup.source, 'modified after packing');
  assert.throws(() => installedBinary(setup.metadata, setup.options), /checksum check/);
  assert.equal(fs.existsSync(dataDirectory(setup.options)), false);
});

test('refuses binary symlinks without changing their target', (t) => {
  const setup = fixture(t);
  const root = dataDirectory(setup.options);
  fs.mkdirSync(root, { recursive: true });
  const target = path.join(setup.directory, 'unrelated-file');
  fs.writeFileSync(target, 'leave alone');
  fs.symlinkSync(target, path.join(root, 'workbench'));
  assert.throws(() => installedBinary(setup.metadata, setup.options), /non-regular file/);
  assert.equal(fs.readFileSync(target, 'utf8'), 'leave alone');
});

test('defaults to start and preserves explicit root flags, spaces and CLI arguments', () => {
  assert.deepEqual(nativeArguments([], '/data root'), ['--root', '/data root', 'start']);
  const args = ['mcp', '--root', '/another root', '--allow-writes'];
  assert.deepEqual(nativeArguments(args, '/default'), ['--root', '/default', ...args]);
  assert.deepEqual(nativeArguments(['--version'], '/data'), ['--root', '/data', '--version']);
});

function launcherFixture(t, program) {
  const setup = fixture(t);
  const installation = path.join(setup.directory, 'installed');
  const platform = platformFor(process.platform, process.arch);
  const nativeName = platformPackageName(packageInfo.name, platform);
  const nativeDirectory = path.join(installation, 'node_modules', ...nativeName.split('/'));
  fs.mkdirSync(path.join(installation, 'npm'), { recursive: true });
  fs.mkdirSync(path.join(nativeDirectory, 'bin'), { recursive: true });
  for (const filename of ['cli.cjs', 'runtime.cjs', 'platforms.cjs']) {
    fs.copyFileSync(path.join(__dirname, filename), path.join(installation, 'npm', filename));
  }
  fs.writeFileSync(path.join(installation, 'package.json'), JSON.stringify(packageInfo));
  fs.writeFileSync(path.join(nativeDirectory, 'package.json'), JSON.stringify({ name: nativeName, version: packageInfo.version }));
  const executable = path.join(nativeDirectory, 'bin', 'workbench');
  const fakeNative = `#!${process.execPath}\n${program}\n`;
  fs.writeFileSync(executable, fakeNative);
  fs.writeFileSync(path.join(nativeDirectory, 'bin', 'SHA256SUM'), crypto.createHash('sha256').update(fakeNative).digest('hex'));
  return {
    cli: path.join(installation, 'npm', 'cli.cjs'),
    env: { ...process.env, WORKBENCH_HOME: path.join(setup.directory, 'data') },
  };
}

test('launcher preserves MCP stdin/stdout and the native exit code', (t) => {
  const setup = launcherFixture(t, "process.stdin.pipe(process.stdout); process.stdin.on('end', () => { process.exitCode = 7; });");
  const protocol = '{"jsonrpc":"2.0","id":1}\n';
  const result = spawnSync(process.execPath, [setup.cli, 'mcp'], {
    env: setup.env,
    input: protocol, encoding: 'utf8', timeout: 10000,
  });
  assert.ifError(result.error);
  assert.equal(result.status, 7, result.stderr);
  assert.equal(result.stdout, protocol);
  assert.equal(result.stderr, '');
});

test('launcher forwards termination to its native child', { timeout: 10000 }, async (t) => {
  const setup = launcherFixture(t, `
    process.on('SIGTERM', () => process.stdout.write('terminated\\n', () => process.exit(0)));
    process.stdout.write('ready\\n');
    setTimeout(() => process.exit(23), 3000);
  `);
  const child = spawn(process.execPath, [setup.cli, 'serve'], { env: setup.env, stdio: ['ignore', 'pipe', 'pipe'] });
  t.after(() => { if (child.exitCode === null) child.kill('SIGTERM'); });
  let output = '';
  child.stdout.on('data', (chunk) => { output += chunk; });
  const closed = once(child, 'close');
  await once(child.stdout, 'data');
  assert.match(output, /ready/);
  child.kill('SIGTERM');
  const [code, signal] = await closed;
  assert.equal(code, 0);
  assert.equal(signal, null);
  assert.match(output, /terminated/);
});

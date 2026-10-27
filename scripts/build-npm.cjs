const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const packageInfo = require('../package.json');
const { platforms, platformPackageName } = require('../npm/platforms.cjs');

const repository = path.resolve(__dirname, '..');
const output = path.join(repository, 'dist', 'npm');
const releaseFiles = [];
const subprocessEnvironment = {
  ...process.env,
  PATH: path.dirname(process.execPath) + path.delimiter + process.env.PATH,
};

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: repository,
    env: subprocessEnvironment,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
    ...options,
  });
  if (result.error || result.status !== 0) {
    throw new Error(`${command} failed: ${result.error?.message || result.stderr || result.stdout}`);
  }
  return result.stdout;
}

function copy(source, destination) {
  fs.mkdirSync(path.dirname(destination), { recursive: true });
  fs.copyFileSync(path.join(repository, source), destination);
}

function digest(filename, algorithm = 'sha256', encoding = 'hex') {
  return crypto.createHash(algorithm).update(fs.readFileSync(filename)).digest(encoding);
}

function remember(filename, metadata) {
  const absolute = path.join(output, filename);
  releaseFiles.push({
    ...metadata,
    file: filename,
    sha256: digest(absolute),
    integrity: 'sha512-' + digest(absolute, 'sha512', 'base64'),
  });
}

function pack(directory, metadata) {
  const [packed] = JSON.parse(run('npm', [
    'pack', directory, '--json', '--ignore-scripts', '--pack-destination', output,
  ]));
  remember(packed.filename, metadata);
}

function writeManifest(directory, manifest) {
  fs.writeFileSync(path.join(directory, 'package.json'), JSON.stringify(manifest, null, 2) + '\n');
}

function copyLicenses(directory) {
  copy('LICENSE', path.join(directory, 'LICENSE'));
  for (const dependency of ['cobra', 'pflag', 'mousetrap', 'goldmark']) {
    copy(`third_party/licenses/${dependency}.txt`, path.join(directory, 'licenses', `${dependency}.txt`));
  }
  copy('web/static/vendor/HTMX-LICENSE', path.join(directory, 'licenses', 'htmx.txt'));
  copy('web/static/fonts/INTER-LICENSE.txt', path.join(directory, 'licenses', 'inter.txt'));
}

function buildPlatform(temporary, platform) {
  const directory = path.join(temporary, platform.id);
  const binary = path.join(directory, 'bin', 'workbench');
  fs.mkdirSync(path.dirname(binary), { recursive: true });
  run('go', [
    'build', '-trimpath', '-buildvcs=false',
    '-ldflags', `-s -w -X workbench/internal/app.Version=${packageInfo.version}`,
    '-o', binary, './cmd/workbench',
  ], { env: {
    ...subprocessEnvironment,
    GOTOOLCHAIN: `go${packageInfo.config.goVersion}`,
    CGO_ENABLED: '0', GOOS: platform.os, GOARCH: platform.goArch,
  } });
  fs.chmodSync(binary, 0o755);
  fs.writeFileSync(path.join(directory, 'bin', 'SHA256SUM'), digest(binary) + '\n');
  copyLicenses(directory);

  const name = platformPackageName(packageInfo.name, platform);
  writeManifest(directory, {
    name,
    version: packageInfo.version,
    description: `Workbench native runtime for ${platform.os}/${platform.arch}.`,
    license: packageInfo.license,
    repository: packageInfo.repository,
    os: [platform.os],
    cpu: [platform.arch],
    files: ['bin/', 'licenses/', 'LICENSE', 'README.md'],
    publishConfig: { access: 'public', registry: 'https://registry.npmjs.org/' },
  });
  fs.writeFileSync(path.join(directory, 'README.md'),
    `# ${name}\n\nNative runtime for ${packageInfo.name}@${packageInfo.version}. ` +
    'The main package installs the matching runtime automatically.\n');
  pack(directory, { kind: 'platform', target: platform.id, name, version: packageInfo.version });

  const portable = path.join(temporary, `portable-${platform.id}`, 'Workbench');
  fs.mkdirSync(portable, { recursive: true });
  fs.copyFileSync(binary, path.join(portable, 'workbench'));
  fs.chmodSync(path.join(portable, 'workbench'), 0o755);
  copyLicenses(portable);
  for (const filename of ['README.md', 'docs/setup.md', 'docs/agents.md', 'docs/architecture.md', 'docs/design.md', 'docs/releasing.md']) {
    copy(filename, path.join(portable, filename));
  }
  const archive = `workbench-${packageInfo.version}-${platform.id}.tar.gz`;
  run('tar', ['-czf', path.join(output, archive), '-C', path.dirname(portable), 'Workbench']);
  remember(archive, { kind: 'binary', target: platform.id, version: packageInfo.version });
  console.log(`Packaged ${name}@${packageInfo.version}`);
}

function buildLauncher(temporary) {
  const directory = path.join(temporary, 'launcher');
  fs.mkdirSync(directory, { recursive: true });
  const files = ['npm/cli.cjs', 'npm/runtime.cjs', 'npm/platforms.cjs', 'LICENSE', 'README.md'];
  for (const filename of files) copy(filename, path.join(directory, filename));
  const repositoryURL = packageInfo.repository.url.replace(/^git\+/, '').replace(/\.git$/, '');
  const readme = fs.readFileSync(path.join(directory, 'README.md'), 'utf8');
  fs.writeFileSync(path.join(directory, 'README.md'), readme.replace(
    /\]\((docs\/[^)]+)\)/g,
    `](${repositoryURL}/blob/v${packageInfo.version}/$1)`,
  ));
  fs.chmodSync(path.join(directory, 'npm', 'cli.cjs'), 0o755);
  writeManifest(directory, {
    name: packageInfo.name,
    version: packageInfo.version,
    description: packageInfo.description,
    license: packageInfo.license,
    author: packageInfo.author,
    repository: packageInfo.repository,
    homepage: packageInfo.homepage,
    bugs: packageInfo.bugs,
    engines: packageInfo.engines,
    bin: { workbench: 'npm/cli.cjs' },
    os: ['darwin', 'linux'],
    cpu: ['arm64', 'x64'],
    files,
    optionalDependencies: Object.fromEntries(platforms.map((platform) => [
      platformPackageName(packageInfo.name, platform), packageInfo.version,
    ])),
    publishConfig: { access: 'public', registry: 'https://registry.npmjs.org/' },
  });
  pack(directory, { kind: 'launcher', name: packageInfo.name, version: packageInfo.version });
}

function main() {
  if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(packageInfo.version)) {
    throw new Error('Use a release version such as 0.1.0 or 0.1.0-beta.1.');
  }
  if (process.argv.length !== 2) throw new Error('Usage: node scripts/build-npm.cjs');
  fs.mkdirSync(output, { recursive: true });
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'workbench-release-'));
  try {
    for (const platform of platforms) buildPlatform(temporary, platform);
    buildLauncher(temporary);
    const manifest = path.join(output, 'release-manifest.json');
    fs.writeFileSync(manifest, JSON.stringify({
      name: packageInfo.name, version: packageInfo.version,
      goVersion: packageInfo.config.goVersion, files: releaseFiles,
    }, null, 2) + '\n');
    const checksums = releaseFiles.map((file) => `${file.sha256}  ${file.file}`);
    checksums.push(`${digest(manifest)}  release-manifest.json`);
    fs.writeFileSync(path.join(output, 'SHA256SUMS'), checksums.join('\n') + '\n');
    console.log(`Release artifacts: ${output}`);
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true });
  }
}

if (require.main === module) {
  try { main(); } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}

module.exports = { digest };

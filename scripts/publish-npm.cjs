const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { digest } = require('./build-npm.cjs');
const packageInfo = require('../package.json');
const { platforms, platformPackageName } = require('../npm/platforms.cjs');

const directory = path.resolve(__dirname, '../dist/npm');

function runNpm(args) {
  const result = spawnSync('npm', args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
  if (result.error) throw result.error;
  return result;
}

function publishedIntegrity(name, version) {
  const result = runNpm(['view', `${name}@${version}`, 'dist.integrity', '--json', '--registry=https://registry.npmjs.org/']);
  if (result.status === 0) return JSON.parse(result.stdout);
  let error;
  try { error = JSON.parse(result.stdout).error; } catch {}
  if (error?.code === 'E404') return null;
  throw new Error(`Could not inspect ${name}@${version}: ${result.stderr || result.stdout}`);
}

function main() {
  const manifest = JSON.parse(fs.readFileSync(path.join(directory, 'release-manifest.json'), 'utf8'));
  if (manifest.name !== packageInfo.name || manifest.version !== packageInfo.version) {
    throw new Error('Release artifacts must match the checked-out package name and version.');
  }
  const expectedNames = [...platforms.map((platform) => platformPackageName(manifest.name, platform)), manifest.name];
  const packages = expectedNames.map((name) => {
    const matches = manifest.files.filter((entry) => entry.name === name);
    if (matches.length !== 1) throw new Error(`Expected exactly one release archive for ${name}.`);
    const entry = matches[0];
    if (entry.file !== path.basename(entry.file) || !entry.file.endsWith('.tgz') || entry.version !== manifest.version) {
      throw new Error(`Invalid release entry for ${name}.`);
    }
    const filename = path.join(directory, entry.file);
    if (digest(filename) !== entry.sha256 || 'sha512-' + digest(filename, 'sha512', 'base64') !== entry.integrity) {
      throw new Error(`Release checksum failed for ${name}.`);
    }
    return { ...entry, filename };
  });

  const tag = manifest.version.includes('-') ? 'next' : 'latest';
  for (const entry of packages) {
    const existing = publishedIntegrity(entry.name, entry.version);
    if (existing) {
      if (existing !== entry.integrity) {
        throw new Error(`${entry.name}@${entry.version} already exists with different contents. Use a new version.`);
      }
      console.log(`Already published with matching integrity: ${entry.name}@${entry.version}`);
      continue;
    }
    // Platform packages are ordered before the launcher so optional dependencies exist.
    const result = runNpm([
      'publish', entry.filename, '--access', 'public', '--ignore-scripts', '--provenance',
      '--tag', tag, '--registry=https://registry.npmjs.org/',
    ]);
    if (result.status !== 0) throw new Error(result.stderr || result.stdout);
    console.log(`Published ${entry.name}@${entry.version} (${tag})`);
  }
}

try { main(); } catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}

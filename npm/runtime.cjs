const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { platformFor, platformPackageName } = require('./platforms.cjs');

function dataDirectory({ platform = process.platform, home = os.homedir(), env = process.env } = {}) {
  if (env.WORKBENCH_HOME) return path.resolve(env.WORKBENCH_HOME);
  if (platform === 'darwin') return path.join(home, 'Library', 'Application Support', 'Workbench');
  if (platform === 'linux') {
    const base = env.XDG_DATA_HOME && path.isAbsolute(env.XDG_DATA_HOME)
      ? env.XDG_DATA_HOME
      : path.join(home, '.local', 'share');
    return path.join(base, 'workbench');
  }
  throw new Error('Workbench supports macOS and Linux.');
}

function fileDigest(filename) {
  return crypto.createHash('sha256').update(fs.readFileSync(filename)).digest('hex');
}

function installedBinary(packageInfo, options = {}) {
  const platform = platformFor(options.platform || process.platform, options.arch || process.arch);
  const name = platformPackageName(packageInfo.name, platform);
  const resolvePackage = options.resolvePackage || require.resolve;
  let manifestPath;
  try {
    manifestPath = resolvePackage(`${name}/package.json`);
  } catch (error) {
    if (error.code !== 'MODULE_NOT_FOUND') throw error;
    throw new Error(
      `Missing native package ${name}@${packageInfo.version}. Install with optional dependencies enabled, ` +
      'or download a native binary from the GitHub release.',
    );
  }

  const nativePackage = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  if (nativePackage.name !== name || nativePackage.version !== packageInfo.version) {
    throw new Error(`Native package must match ${name}@${packageInfo.version}. Reinstall Workbench.`);
  }

  const source = path.join(path.dirname(manifestPath), 'bin', 'workbench');
  const expectedDigest = fs.readFileSync(path.join(path.dirname(manifestPath), 'bin', 'SHA256SUM'), 'utf8').trim();
  if (!/^[a-f0-9]{64}$/.test(expectedDigest) || fileDigest(source) !== expectedDigest) {
    throw new Error('The packaged Workbench binary failed its checksum check. Reinstall Workbench.');
  }

  const root = dataDirectory(options);
  fs.mkdirSync(root, { recursive: true, mode: 0o700 });
  // The stable path lets the Go engine reload a new build after npm upgrades.
  const destination = path.join(root, 'workbench');
  let existing;
  try {
    existing = fs.lstatSync(destination);
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
  }
  if (existing && !existing.isFile()) {
    throw new Error(`Refusing to replace a non-regular file at ${destination}. Choose another WORKBENCH_HOME.`);
  }
  if (existing && fileDigest(destination) === expectedDigest) {
    fs.chmodSync(destination, 0o755);
    return { executable: destination, root };
  }

  const temporaryDirectory = fs.mkdtempSync(path.join(root, '.workbench-install-'));
  try {
    const temporaryBinary = path.join(temporaryDirectory, 'workbench');
    fs.copyFileSync(source, temporaryBinary, fs.constants.COPYFILE_EXCL);
    fs.chmodSync(temporaryBinary, 0o755);
    if (fileDigest(temporaryBinary) !== expectedDigest) {
      throw new Error('The Workbench binary changed during installation. Reinstall Workbench.');
    }
    // Atomic replacement preserves already-running engines and detached runners.
    fs.renameSync(temporaryBinary, destination);
  } finally {
    fs.rmSync(temporaryDirectory, { recursive: true, force: true });
  }

  return { executable: destination, root };
}

function nativeArguments(cliArguments, root) {
  const requested = cliArguments.length ? cliArguments : ['start'];
  return ['--root', root, ...requested];
}

module.exports = { dataDirectory, fileDigest, installedBinary, nativeArguments };

const platforms = [
  { id: 'darwin-arm64', os: 'darwin', arch: 'arm64', goArch: 'arm64' },
  { id: 'darwin-x64', os: 'darwin', arch: 'x64', goArch: 'amd64' },
  { id: 'linux-arm64', os: 'linux', arch: 'arm64', goArch: 'arm64' },
  { id: 'linux-x64', os: 'linux', arch: 'x64', goArch: 'amd64' },
];

function platformFor(os, arch) {
  const platform = platforms.find((candidate) => candidate.os === os && candidate.arch === arch);
  if (!platform) {
    throw new Error(`Workbench supports macOS and Linux on arm64 and x64; received ${os}/${arch}.`);
  }
  return platform;
}

function platformPackageName(name, platform) {
  return `${name}-${platform.id}`;
}

module.exports = { platforms, platformFor, platformPackageName };

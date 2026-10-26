const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const rootDirectory = path.resolve(__dirname, '..');

function javascriptFiles(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) return javascriptFiles(filename);
    return entry.name.endsWith('.js') ? [filename] : [];
  });
}

const filenames = [
  path.join(rootDirectory, 'web/static/app.js'),
  path.join(rootDirectory, 'web/static/skills.js'),
  ...javascriptFiles(path.join(rootDirectory, 'web/static/js')),
];

for (const filename of filenames) {
  const result = spawnSync(process.execPath, ['--input-type=module', '--check'], {
    input: fs.readFileSync(filename, 'utf8'),
    encoding: 'utf8',
  });

  if (result.error || result.status !== 0) {
    console.error(path.relative(rootDirectory, filename));
    console.error(result.error || result.stderr);
    process.exit(1);
  }
}

console.log(`JavaScript syntax passed for ${filenames.length} browser modules.`);

const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const rootDirectory = path.resolve(__dirname, '../../..');

function publicAssets() {
  const source = fs.readFileSync(path.join(rootDirectory, 'web/assets.go'), 'utf8');
  const entries = [...source.matchAll(/^\s*"([^"]+)":\s*"([^"]+)",/gm)];
  return Object.fromEntries(entries.map((entry) => [entry[1], entry[2]]));
}

// jsdom does not execute module scripts. Evaluate the same native import graph in
// its browser context; the HTTP suite also retrieves every module over real HTTP.
async function loadDashboard(dom, { overHTTP = false } = {}) {
  if (!vm.SourceTextModule) {
    throw new Error('Browser module tests need --experimental-vm-modules. Use make check-ui.');
  }

  const assets = publicAssets();
  const origin = new URL(dom.window.location.href).origin;
  const modules = new Map();
  const context = dom.getInternalVMContext();

  function loadModule(address) {
    const url = new URL(address);
    if (url.origin !== origin || !assets[url.pathname]?.endsWith('.js')) {
      throw new Error('Unexpected browser module: ' + url.href);
    }

    if (!modules.has(url.href)) {
      modules.set(url.href, (async () => {
        let source;
        if (overHTTP) {
          const response = await fetch(url);
          if (!response.ok) throw new Error(`Module HTTP ${response.status}: ${url.href}`);
          source = await response.text();
        } else {
          source = fs.readFileSync(path.join(rootDirectory, 'web/static', assets[url.pathname]), 'utf8');
        }

        return new vm.SourceTextModule(source, {
          context,
          identifier: url.href,
          initializeImportMeta(metadata) { metadata.url = url.href; },
        });
      })());
    }

    return modules.get(url.href);
  }

  const entry = await loadModule(new URL('/app.js', origin));
  await entry.link((specifier, referencingModule) => loadModule(new URL(specifier, referencingModule.identifier)));
  await entry.evaluate();
}

module.exports = { loadDashboard, publicAssets };

export async function redirectLegacyEngine() {
  if (location.port !== '8730') return false;
  try {
    const response = await fetch('/api/health');
    const health = await response.json();
    if (health.app === 'workbench' && !health.version) {
      location.replace('http://127.0.0.1:8731/' + location.hash);
      return true;
    }
  } catch {}
  return false;
}

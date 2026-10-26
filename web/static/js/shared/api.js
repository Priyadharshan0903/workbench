import { workspaceState } from './state.js';

export async function requestJSON(path, body) {
  const response = await fetch(
    path,
    body
      ? {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Workbench-Token': workspaceState.snapshot?.csrfToken || '',
          },
          body: JSON.stringify(body),
        }
      : {},
  );
  let data;
  try {
    data = await response.json();
  } catch {
    throw new Error('The local engine did not return a valid response. Refresh and retry.');
  }
  if (!response.ok) throw new Error(data.error || 'The local request failed.');
  return data;
}

export function safePreview(html) {
  const template = document.createElement('template');
  template.innerHTML = html || '<p>No instructions yet.</p>';
  const allowed = new Set([
    'P',
    'H1',
    'H2',
    'H3',
    'H4',
    'H5',
    'H6',
    'UL',
    'OL',
    'LI',
    'PRE',
    'CODE',
    'BLOCKQUOTE',
    'A',
    'HR',
    'BR',
    'EM',
    'STRONG',
    'DEL',
    'TABLE',
    'THEAD',
    'TBODY',
    'TR',
    'TH',
    'TD',
  ]);

  for (const element of [...template.content.querySelectorAll('*')]) {
    if (element.tagName === 'IMG') {
      element.replaceWith(document.createTextNode(element.getAttribute('alt') || 'Image'));
      continue;
    }
    if (!allowed.has(element.tagName)) {
      element.remove();
      continue;
    }

    for (const attribute of [...element.attributes]) {
      if (!(element.tagName === 'A' && attribute.name === 'href'))
        element.removeAttribute(attribute.name);
    }
    if (element.tagName === 'A') {
      const href = element.getAttribute('href') || '';
      try {
        const address = new URL(href, 'https://skill.local/');
        if (!['http:', 'https:'].includes(address.protocol)) element.removeAttribute('href');
      } catch {
        element.removeAttribute('href');
      }
    }
  }

  return template.content;
}

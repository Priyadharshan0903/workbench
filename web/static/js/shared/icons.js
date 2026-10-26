const iconPaths = {
  grid: 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z',
  list: 'M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01',
  star: 'm12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2-5.6-3-5.6 3 1.1-6.2L3 9.6l6.2-.9Z',
  clock: 'M12 8v4l3 2',
  activity: 'm2 12 5 0 3-8 4 16 3-8h5',
  layers: 'm12 3 10 6-10 6L2 9Zm-9 12 9 6 9-6M3 12l9 6 9-6',
  terminal: 'm5 7 5 5-5 5M13 17h6',
  download: 'M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5',
  menu: 'M4 6h16M4 12h16M4 18h16',
  moon: 'M21 13A9 9 0 0 1 11 3a9 9 0 1 0 10 10Z',
  sun: 'M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1 1M18 18l1 1M5 19l1-1M18 6l1-1',
  refresh: 'M20 7v5h-5M4 17v-5h5M6 7a7 7 0 0 1 12-2l2 2M18 17a7 7 0 0 1-12 2l-2-2',
  plus: 'M12 5v14M5 12h14',
  folder: 'M3 7V5h6l2 2h10v13H3Z',
  search: 'm16 16 5 5M10 17a7 7 0 1 0 0-14 7 7 0 0 0 0 14',
  close: 'm6 6 12 12M6 18 18 6',
  arrow: 'M5 12h14m-5-5 5 5-5 5',
  external: 'M14 3h7v7m0-7L10 14M10 3H3v18h18v-7',
  play: 'm8 4 13 8-13 8Z',
  stop: 'M6 6h12v12H6Z',
  dots: 'M9 5h.01M15 5h.01M9 12h.01M15 12h.01M9 19h.01M15 19h.01',
  code: 'm8 6-6 6 6 6m8-12 6 6-6 6m-3-16-2 20',
  check: 'm5 12 4 4L19 6',
  chevron: 'm9 5 7 7-7 7',
  log: 'M4 4h16v16H4zM8 8h8M8 12h8M8 16h5',
  settings:
    'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8M12 2v3m0 14v3M2 12h3m14 0h3M5 5l2 2m10 10 2 2M5 19l2-2M17 7l2-2',
};

export function renderIcon(name) {
  const circleRadius = name === 'clock' ? 9 : name === 'sun' ? 4 : 0;
  const circle = circleRadius ? `<circle cx="12" cy="12" r="${circleRadius}"/>` : '';

  return `<svg
  viewBox="0 0 24 24"
  width="18"
  height="18"
  fill="none"
  stroke="currentColor"
  stroke-width="1.6"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  ${circle}
  <path d="${iconPaths[name] || iconPaths.code}" />
</svg>`;
}

export function paintIcons(root = document) {
  root.querySelectorAll('[data-icon]').forEach((element) => {
    element.innerHTML = renderIcon(element.dataset.icon);
  });
}

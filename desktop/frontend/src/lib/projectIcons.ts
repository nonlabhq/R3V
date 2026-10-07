// The icons a project can have (drawn on a 24 grid, in the current colour).
// The names are what a team stores: add icons, never rename or remove one.
// A name this build doesn't know shows as the project's initial. The
// programs' icons are in appIcons.ts.

import { appIcons } from "./appIcons";

const drawn: Record<string, string> = {
  note: `<path d="M9 18V5l11-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="17" cy="16" r="3"/>`,
  quaver: `<path d="M12 17V3l6 3"/><circle cx="9" cy="17" r="3"/>`,
  mic: `<rect x="9" y="2" width="6" height="12" rx="3"/><path d="M5 10a7 7 0 0 0 14 0M12 17v5M8 22h8"/>`,
  headphones: `<path d="M3 18v-6a9 9 0 0 1 18 0v6"/><rect x="3" y="14" width="4" height="7" rx="1.5"/><rect x="17" y="14" width="4" height="7" rx="1.5"/>`,
  drum: `<ellipse cx="12" cy="9" rx="8" ry="3"/><path d="M4 9v8c0 1.7 3.6 3 8 3s8-1.3 8-3V9M16 2l-4 7M8 2l4 7"/>`,
  guitar: `<path d="M14 10l6-6M18 2l4 4"/><path d="M11 8a4 4 0 0 0-5.6 1.6L4 12a4.5 4.5 0 0 0 8 8l2.4-1.4A4 4 0 0 0 16 13z"/><circle cx="10" cy="14" r="1.5"/>`,
  keys: `<rect x="2" y="5" width="20" height="14" rx="2"/><path d="M7.5 13v6M12 13v6M16.5 13v6"/><path d="M6.5 5h2v8h-2zM11 5h2v8h-2zM15.5 5h2v8h-2z" fill="currentColor"/>`,
  speaker: `<rect x="5" y="2" width="14" height="20" rx="2"/><circle cx="12" cy="14" r="4"/><circle cx="12" cy="6.5" r="1"/>`,
  waveform: `<path d="M2 12h2M6 8v8M10 4v16M14 7v10M18 10v4M20 12h2"/>`,
  sine: `<path d="M2 12c2-5 4-5 6 0s4 5 6 0 4-5 6 0"/>`,
  sliders: `<path d="M6 3v18M12 3v18M18 3v18"/><rect x="4" y="13" width="4" height="3" rx="1" fill="currentColor"/><rect x="10" y="6" width="4" height="3" rx="1" fill="currentColor"/><rect x="16" y="15" width="4" height="3" rx="1" fill="currentColor"/>`,
  knob: `<circle cx="12" cy="13" r="7"/><path d="M12 13l3-4M12 2v2M3.5 6.5l1.5 1.5M20.5 6.5L19 8"/>`,
  vinyl: `<circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/><path d="M12 5a7 7 0 0 1 7 7"/>`,
  cassette: `<rect x="2" y="5" width="20" height="14" rx="2"/><circle cx="8" cy="11" r="2"/><circle cx="16" cy="11" r="2"/><path d="M6 19l2-4h8l2 4"/>`,
  radio: `<rect x="2" y="8" width="20" height="13" rx="2"/><path d="M6 8l12-5"/><circle cx="8" cy="14.5" r="3"/><path d="M15 13h4M15 16h4"/>`,
  metronome: `<path d="M9 3h6l4 18H5z"/><path d="M12 17l5-10"/>`,
  loop: `<path d="M17 2l4 4-4 4M3 11V9a3 3 0 0 1 3-3h15M7 22l-4-4 4-4M21 13v2a3 3 0 0 1-3 3H3"/>`,
  layers: `<path d="M12 2l10 5-10 5L2 7z"/><path d="M2 12l10 5 10-5M2 17l10 5 10-5"/>`,
  bolt: `<path d="M13 2L4 14h7l-1 8 9-12h-7z"/>`,
  star: `<path d="M12 2l3 6.5 7 .8-5.2 4.8 1.4 7L12 17.6 5.8 21l1.4-7L2 9.3l7-.8z"/>`,
  spark: `<path d="M12 2v6M12 16v6M2 12h6M16 12h6M5 5l3.5 3.5M15.5 15.5L19 19M5 19l3.5-3.5M15.5 8.5L19 5"/>`,
  heart: `<path d="M12 20s-8-4.5-8-11a4.5 4.5 0 0 1 8-2.8A4.5 4.5 0 0 1 20 9c0 6.5-8 11-8 11z"/>`,
  moon: `<path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z"/>`,
  sun: `<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>`,
  cloud: `<path d="M7 18a5 5 0 0 1-.6-10A6 6 0 0 1 18 9a4.5 4.5 0 0 1-.5 9z"/>`,
  flame: `<path d="M12 22a7 7 0 0 0 7-7c0-4-3-6-4-10-2 2-3 4-3 6-1-1-2-2-2-4-2 2-5 5-5 8a7 7 0 0 0 7 7z"/>`,
  leaf: `<path d="M5 19C5 9 11 4 20 4c0 9-5 15-15 15zM5 19l8-8"/>`,
  mountain: `<path d="M2 20l7-12 4 6 3-4 6 10z"/>`,
  waves: `<path d="M2 9c2.5 0 2.5-2 5-2s2.5 2 5 2 2.5-2 5-2 2.5 2 5 2M2 15c2.5 0 2.5-2 5-2s2.5 2 5 2 2.5-2 5-2 2.5 2 5 2"/>`,
  planet: `<circle cx="12" cy="12" r="6"/><path d="M4.5 15.5c-3 3-2.5 4.5-1 5 2 .7 7-1.7 11.4-6.1s6.3-9.2 5.6-10.9c-.5-1.5-2-2-5 1"/>`,
  rocket: `<path d="M12 2c4 3 5 7 4 12H8C7 9 8 5 12 2z"/><path d="M8 14l-3 4h4M16 14l3 4h-4M10 18l2 4 2-4"/><circle cx="12" cy="9" r="1.5"/>`,
  cube: `<path d="M12 2l9 5v10l-9 5-9-5V7z"/><path d="M3 7l9 5 9-5M12 12v10"/>`,
  gamepad: `<rect x="2" y="7" width="20" height="11" rx="5"/><path d="M7 10.5v4M5 12.5h4"/><circle cx="15.5" cy="11.5" r="1"/><circle cx="18" cy="14" r="1"/>`,
  film: `<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M7 3v18M17 3v18M3 8h4M3 16h4M17 8h4M17 16h4M3 12h18"/>`,
  camera: `<path d="M3 8h4l2-3h6l2 3h4v12H3z"/><circle cx="12" cy="13.5" r="3.5"/>`,
  brush: `<path d="M19 3l2 2-9 9-2-2zM10 12c-3 0-4 2-4 4s-1 3-3 4c4 1 9 0 9-4z"/>`,
  pen: `<path d="M16 3l5 5L9 20H4v-5zM13 6l5 5"/>`,
  code: `<path d="M8 6l-6 6 6 6M16 6l6 6-6 6M14 4l-4 16"/>`,
  globe: `<circle cx="12" cy="12" r="10"/><path d="M2 12h20M12 2c3 3 4 6.5 4 10s-1 7-4 10c-3-3-4-6.5-4-10s1-7 4-10z"/>`,
  flag: `<path d="M5 22V3M5 4h13l-3 4.5L18 13H5"/>`,
  diamond: `<path d="M6 3h12l4 6-10 12L2 9zM2 9h20M12 21L8 9l2-6M12 21l4-12-2-6"/>`,
  crown: `<path d="M3 7l4 4 5-7 5 7 4-4-2 12H5z"/>`,
  eye: `<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>`,
  book: `<path d="M4 4.5A2.5 2.5 0 0 1 6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5zM4 19.5A2.5 2.5 0 0 1 6.5 17H20"/>`,
  bell: `<path d="M6 16v-5a6 6 0 0 1 12 0v5l2 2H4zM10 21h4"/>`,
  coffee: `<path d="M4 8h13v6a5 5 0 0 1-5 5H9a5 5 0 0 1-5-5zM17 10h1.5a2.5 2.5 0 0 1 0 5H17M8 2v3M12 2v3"/>`,
  ghost: `<path d="M5 21V10a7 7 0 0 1 14 0v11l-2.3-2-2.4 2-2.3-2-2.3 2-2.4-2z"/><circle cx="9.5" cy="10" r="1"/><circle cx="14.5" cy="10" r="1"/>`,
  house: `<path d="M3 11l9-8 9 8v10H3zM10 21v-6h4v6"/>`,
  target: `<circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/>`,
  triangle: `<path d="M12 3l10 18H2z"/>`,
};

// The drawn ones, in the picker after the programs'.
export const projectIconNames = Object.keys(drawn);

export const projectIcons: Record<string, string> = {
  ...drawn,
  ...Object.fromEntries(Object.entries(appIcons).map(([name, a]) => [name, a.svg])),
};

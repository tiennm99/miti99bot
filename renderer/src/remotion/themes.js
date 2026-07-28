export const themes = {
  classic: {
    background: '#f8fafc',
    wheelShadow: '0 18px 52px rgba(15, 23, 42, 0.22)',
    center: '#111827',
    centerRing: '#f9fafb',
    pointer: '#ef4444',
    pointerStroke: '#991b1b',
    text: '#111827',
    textHalo: 'rgba(255, 255, 255, 0.88)',
    winnerGlow: 'rgba(250, 204, 21, 0.45)',
    slices: ['#f97316', '#14b8a6', '#facc15', '#60a5fa', '#f472b6', '#a78bfa', '#34d399', '#fb7185'],
  },
  festival: {
    background: '#fff7ed',
    wheelShadow: '0 20px 56px rgba(124, 45, 18, 0.24)',
    center: '#7f1d1d',
    centerRing: '#fffbeb',
    pointer: '#dc2626',
    pointerStroke: '#7f1d1d',
    text: '#1f2937',
    textHalo: 'rgba(255, 251, 235, 0.92)',
    winnerGlow: 'rgba(251, 146, 60, 0.5)',
    slices: ['#fb7185', '#f59e0b', '#84cc16', '#06b6d4', '#a78bfa', '#f472b6', '#22c55e', '#fb923c'],
  },
  mono: {
    background: '#f4f4f5',
    wheelShadow: '0 18px 52px rgba(24, 24, 27, 0.18)',
    center: '#18181b',
    centerRing: '#fafafa',
    pointer: '#27272a',
    pointerStroke: '#09090b',
    text: '#18181b',
    textHalo: 'rgba(250, 250, 250, 0.9)',
    winnerGlow: 'rgba(161, 161, 170, 0.45)',
    slices: ['#e4e4e7', '#a1a1aa', '#d4d4d8', '#b8b8bf', '#f4f4f5', '#c4c4ca'],
  },
};

/**
 * @param {'classic' | 'festival' | 'mono'} name
 */
export const getTheme = (name) => themes[name] ?? themes.classic;

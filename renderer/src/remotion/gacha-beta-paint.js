/**
 * Bakes the beta wish's cloud layers once per roll: puffs are painted with
 * radial gradients on a canvas, then torn into painterly edges by a seeded
 * noise displacement. Each frame then only moves the baked image, which keeps
 * the clouds' look without paying for a live SVG filter on every frame.
 */

import {createRandom} from './gacha-timeline.js';

/**
 * @typedef {object} Puff
 * @property {number} dx  Offset in cluster units.
 * @property {number} dy
 * @property {number} r   Radius in cluster units.
 */

/**
 * @typedef {object} CloudCluster
 * @property {number} x     Centre in canvas pixels.
 * @property {number} y
 * @property {number} size  Cluster size in pixels.
 * @property {Puff[]} puffs
 */

/**
 * Smooth value noise in [0, 1] over a seeded lattice, summed over octaves.
 *
 * @param {number} seed
 * @param {number} frequency  Lattice cells per pixel at the first octave.
 * @param {number} octaves
 * @returns {(x: number, y: number) => number}
 */
export const createValueNoise = (seed, frequency, octaves) => {
  const random = createRandom(seed);
  const size = 256;
  const lattice = Float32Array.from({length: size * size}, () => random());
  /** @param {number} t */
  const fade = (t) => t * t * (3 - 2 * t);
  /**
   * @param {number} x
   * @param {number} y
   */
  const sample = (x, y) => {
    const x0 = Math.floor(x);
    const y0 = Math.floor(y);
    const tx = fade(x - x0);
    const ty = fade(y - y0);
    const at = (/** @type {number} */ ix, /** @type {number} */ iy) =>
      lattice[(((iy % size) + size) % size) * size + (((ix % size) + size) % size)] ?? 0;
    const top = at(x0, y0) + (at(x0 + 1, y0) - at(x0, y0)) * tx;
    const bottom = at(x0, y0 + 1) + (at(x0 + 1, y0 + 1) - at(x0, y0 + 1)) * tx;
    return top + (bottom - top) * ty;
  };
  return (x, y) => {
    let total = 0;
    let weight = 0;
    let amplitude = 1;
    let scale = frequency;
    for (let octave = 0; octave < octaves; octave += 1) {
      total += sample(x * scale, y * scale) * amplitude;
      weight += amplitude;
      amplitude /= 2;
      scale *= 2;
    }
    return total / weight;
  };
};

/**
 * Draws one cluster: a blue-shadowed base, then sunlit puffs on top.
 *
 * @param {CanvasRenderingContext2D} context
 * @param {CloudCluster} cluster
 */
const paintCluster = (context, {x, y, size, puffs}) => {
  const base = context.createRadialGradient(x, y + size * 0.1, 0, x, y + size * 0.3, size * 1.05);
  base.addColorStop(0, 'rgba(170,198,240,0.95)');
  base.addColorStop(0.55, 'rgba(120,156,220,0.6)');
  base.addColorStop(1, 'rgba(120,156,220,0)');
  context.fillStyle = base;
  context.beginPath();
  context.ellipse(x, y + size * 0.3, size * 1.05, size * 0.35, 0, 0, Math.PI * 2);
  context.fill();
  for (const puff of puffs) {
    const cx = x + puff.dx * size;
    const cy = y + puff.dy * size;
    const r = puff.r * size;
    const shade = context.createRadialGradient(cx - r * 0.08, cy - r * 0.32, 0, cx - r * 0.08, cy - r * 0.32, r * 1.3);
    shade.addColorStop(0, '#ffffff');
    shade.addColorStop(0.3, '#f5f9ff');
    shade.addColorStop(0.52, '#d6e5fa');
    shade.addColorStop(0.68, 'rgba(150,182,232,0.9)');
    shade.addColorStop(0.8, 'rgba(150,182,232,0)');
    context.fillStyle = shade;
    context.beginPath();
    context.arc(cx, cy, r, 0, Math.PI * 2);
    context.fill();
  }
};

/**
 * Paints clusters onto a canvas of the given size, displaces the result with
 * two noise fields for torn, painterly edges, and returns it as a PNG data
 * URL. Returns an empty string outside a browser.
 *
 * @param {{width: number, height: number, clusters: CloudCluster[], seed: number, frequency: number, strength: number}} options
 *   `frequency` is noise cells per pixel; `strength` is the displacement in pixels.
 * @returns {string}
 */
export const bakeCloudLayer = ({width, height, clusters, seed, frequency, strength}) => {
  if (typeof document === 'undefined') {
    return '';
  }
  const w = Math.ceil(width);
  const h = Math.ceil(height);
  const canvas = document.createElement('canvas');
  canvas.width = w;
  canvas.height = h;
  const context = canvas.getContext('2d');
  if (!context) {
    return '';
  }
  for (const cluster of clusters) {
    paintCluster(context, cluster);
  }
  const source = context.getImageData(0, 0, w, h);
  const output = context.createImageData(w, h);
  const noiseX = createValueNoise(seed, frequency, 2);
  const noiseY = createValueNoise(seed + 1, frequency, 2);
  for (let y = 0; y < h; y += 1) {
    for (let x = 0; x < w; x += 1) {
      const sx = Math.min(w - 1, Math.max(0, Math.round(x + (noiseX(x, y) - 0.5) * 2 * strength)));
      const sy = Math.min(h - 1, Math.max(0, Math.round(y + (noiseY(x, y) - 0.5) * 2 * strength)));
      const from = (sy * w + sx) * 4;
      const to = (y * w + x) * 4;
      output.data[to] = source.data[from] ?? 0;
      output.data[to + 1] = source.data[from + 1] ?? 0;
      output.data[to + 2] = source.data[from + 2] ?? 0;
      output.data[to + 3] = source.data[from + 3] ?? 0;
    }
  }
  context.putImageData(output, 0, 0);
  return canvas.toDataURL('image/png');
};

import {describe, expect, test} from 'vitest';
import {
  CONFETTI_REFERENCE_SIZE,
  createConfettiParticles,
  getConfettiParticleState,
} from '../src/remotion/confetti-layout.js';

// Cyclic pseudo-random source: stable, spans (0, 1), and keys map to values.
const makeRandom = () => {
  const seen = new Map();
  let next = 0;
  return (/** @type {string} */ seed) => {
    if (!seen.has(seed)) {
      seen.set(seed, (next++ % 10) / 10 + 0.05);
    }
    return seen.get(seed);
  };
};

describe('confetti layout', () => {
  test('creates the requested particle count', () => {
    const particles = createConfettiParticles({
      colors: ['#f97316', '#14b8a6'],
      count: 24,
      random: makeRandom(),
      size: 512,
    });

    expect(particles).toHaveLength(24);
  });

  test('is deterministic for the same seed source', () => {
    const params = {colors: ['#f97316', '#14b8a6'], count: 8, size: 512};
    const first = createConfettiParticles({...params, random: makeRandom()});
    const second = createConfettiParticles({...params, random: makeRandom()});

    expect(second).toEqual(first);
  });

  test('draws colors only from the provided palette', () => {
    const palette = ['#f97316', '#14b8a6', '#facc15'];
    const particles = createConfettiParticles({
      colors: palette,
      count: 40,
      random: makeRandom(),
      size: 512,
    });

    for (const particle of particles) {
      expect(palette).toContain(particle.color);
    }
  });

  test('falls back to a default color when palette is empty', () => {
    const particles = createConfettiParticles({
      colors: [],
      count: 4,
      random: makeRandom(),
      size: 512,
    });

    for (const particle of particles) {
      expect(typeof particle.color).toBe('string');
      expect(particle.color.length).toBeGreaterThan(0);
    }
  });

  test('scales physics with canvas size', () => {
    const [small] = createConfettiParticles({colors: ['#000'], count: 1, random: makeRandom(), size: 256});
    const [large] = createConfettiParticles({colors: ['#000'], count: 1, random: makeRandom(), size: 1024});

    if (!small || !large) {
      throw new Error('expected one particle per call');
    }

    expect(large.gravity).toBeCloseTo(small.gravity * 4);
    expect(large.width).toBeCloseTo(small.width * 4);
  });

  test('reference size keeps gravity at the tuned constant', () => {
    const [particle] = createConfettiParticles({
      colors: ['#000'],
      count: 1,
      random: makeRandom(),
      size: CONFETTI_REFERENCE_SIZE,
    });

    expect(particle?.gravity).toBe(1500);
  });

  test('particle starts at its origin and accelerates downward', () => {
    const particle = {
      color: '#000',
      gravity: 1500,
      height: 8,
      originX: 100,
      originY: 200,
      rotationSpeed: 90,
      startRotation: 10,
      vx: 50,
      vy: -300,
      width: 8,
    };

    expect(getConfettiParticleState(particle, 0)).toEqual({
      rotation: 10,
      x: 100,
      y: 200,
    });

    const later = getConfettiParticleState(particle, 1);
    expect(later.x).toBe(150);
    expect(later.y).toBe(650); // 200 - 300 + 0.5 * 1500
    expect(later.rotation).toBe(100);
  });
});

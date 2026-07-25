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

  test('keeps particles thick enough to survive GIF quantization', () => {
    for (const size of [384, 480, 512]) {
      const particles = createConfettiParticles({
        colors: ['#000'],
        count: 70,
        random: makeRandom(),
        size,
      });

      for (const particle of particles) {
        expect(particle.height).toBeGreaterThanOrEqual(3);
        expect(particle.width).toBeGreaterThanOrEqual(3);
      }
    }
  });

  test('launches from two cannons framing the wheel, not from the hub', () => {
    const size = CONFETTI_REFERENCE_SIZE;
    const particles = createConfettiParticles({
      colors: ['#000'],
      count: 20,
      random: makeRandom(),
      size,
    });
    const hubRadius = size * 0.09;

    for (const particle of particles) {
      const distanceFromHub = Math.hypot(particle.originX - size / 2, particle.originY - size / 2);

      expect(distanceFromHub).toBeGreaterThan(hubRadius);
      // Upward launch: negative vy points up in screen coordinates.
      expect(particle.vy).toBeLessThan(0);
      // Inward launch: left cannon throws right, right cannon throws left.
      expect(particle.originX < size / 2 ? particle.vx > 0 : particle.vx < 0).toBe(true);
    }
  });

  test('drag gives a terminal fall speed instead of unbounded acceleration', () => {
    const [particle] = createConfettiParticles({
      colors: ['#000'],
      count: 1,
      random: makeRandom(),
      size: CONFETTI_REFERENCE_SIZE,
    });

    if (!particle) {
      throw new Error('expected one particle');
    }

    expect(particle.drag).toBeGreaterThan(0);

    const terminalSpeed = particle.gravity / particle.drag;
    const late = getConfettiParticleState(particle, 6);
    const later = getConfettiParticleState(particle, 7);

    expect(later.y - late.y).toBeCloseTo(terminalSpeed, 0);
  });

  test('particle sits at its muzzle until its launch delay elapses', () => {
    const [particle] = createConfettiParticles({
      colors: ['#000'],
      count: 4,
      random: makeRandom(),
      size: CONFETTI_REFERENCE_SIZE,
    });

    if (!particle) {
      throw new Error('expected one particle');
    }

    const atLaunch = getConfettiParticleState(particle, particle.delay);

    expect(atLaunch.x).toBeCloseTo(particle.originX);
    expect(atLaunch.y).toBeCloseTo(particle.originY);

    // Orientation at launch is pulled toward the direction of travel, because
    // that is where the streak is strongest. It is a blend, not a snap, so
    // assert it sits closer to travel than the particle's own tumble does.
    const travelDegrees = (Math.atan2(particle.vy, particle.vx) * 180) / Math.PI;
    /** @param {number} degrees */
    const offFromTravel = (degrees) =>
      Math.abs((((degrees - travelDegrees) % 360) + 540) % 360 - 180);

    expect(offFromTravel(atLaunch.rotation)).toBeLessThanOrEqual(
      offFromTravel(particle.startRotation),
    );
  });

  test('streaks while fast and relaxes to its own tumble once slow', () => {
    const particles = createConfettiParticles({
      colors: ['#000'],
      count: 12,
      random: makeRandom(),
      size: CONFETTI_REFERENCE_SIZE,
    });

    for (const particle of particles) {
      const launch = getConfettiParticleState(particle, particle.delay);
      const settled = getConfettiParticleState(particle, particle.delay + 3);

      expect(launch.stretch).toBeGreaterThan(1);
      expect(launch.stretch).toBeLessThanOrEqual(1.8);
      expect(settled.stretch).toBeLessThan(launch.stretch);
      expect(settled.stretch).toBeGreaterThanOrEqual(1);
    }
  });

  test('keeps the streaked long axis short enough to read as paper', () => {
    for (const size of [384, 480, 512]) {
      const particles = createConfettiParticles({
        colors: ['#000'],
        count: 70,
        random: makeRandom(),
        size,
      });

      for (const particle of particles) {
        const {stretch} = getConfettiParticleState(particle, particle.delay);

        expect(particle.width * stretch).toBeLessThanOrEqual(40);
        // Stretching runs along the long axis only, so the thin dimension still
        // survives GIF quantization.
        expect(particle.height).toBeGreaterThanOrEqual(3);
      }
    }
  });

  test('holds a second volley back so a long celebration does not thin out', () => {
    const longWindow = createConfettiParticles({
      colors: ['#000'],
      count: 70,
      random: makeRandom(),
      size: CONFETTI_REFERENCE_SIZE,
      windowSeconds: 2.47,
    });
    const late = longWindow.filter((particle) => particle.delay > 0.8);

    expect(late.length).toBeGreaterThan(8);

    // Every particle must still launch inside the window it was built for.
    for (const particle of longWindow) {
      expect(particle.delay).toBeLessThan(2.47 * 0.6);
    }
  });

  test('collapses the second volley into the first on a short celebration', () => {
    const shortWindow = createConfettiParticles({
      colors: ['#000'],
      count: 70,
      random: makeRandom(),
      size: CONFETTI_REFERENCE_SIZE,
      windowSeconds: 0.47,
    });

    for (const particle of shortWindow) {
      expect(particle.delay).toBeLessThan(0.47);
    }
  });

  test('rises before it falls', () => {
    const particles = createConfettiParticles({
      colors: ['#000'],
      count: 12,
      random: makeRandom(),
      size: CONFETTI_REFERENCE_SIZE,
    });

    for (const particle of particles) {
      const launch = getConfettiParticleState(particle, particle.delay);
      const soonAfter = getConfettiParticleState(particle, particle.delay + 0.15);

      expect(soonAfter.y).toBeLessThan(launch.y);
    }
  });
});

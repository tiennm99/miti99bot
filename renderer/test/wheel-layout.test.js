import {describe, expect, test} from 'vitest';
import {
  cssDegreesToPieRadians,
  getFinalWheelRotationDegrees,
  getPointerDeflectionDegrees,
  getSliceCenterDegrees,
  getSliceDegrees,
  getSpinProgress,
  getSpinRecoilDegrees,
  getSpinTurns,
  getWheelRotationDegrees,
} from '../src/remotion/wheel-layout.js';

describe('wheel layout', () => {
  test('calculates equal slice sizes', () => {
    expect(getSliceDegrees(8)).toBe(45);
  });

  test('calculates center angle from 3h clockwise', () => {
    expect(getSliceCenterDegrees(4, 0)).toBe(45);
    expect(getSliceCenterDegrees(4, 1)).toBe(135);
  });

  test('final rotation places winner center at 3h', () => {
    const optionCount = 8;
    const winnerIndex = 3;
    const rotation = getFinalWheelRotationDegrees(optionCount, winnerIndex, 7);
    const winnerCenter = getSliceCenterDegrees(optionCount, winnerIndex);
    const screenAngle = ((winnerCenter + rotation) % 360 + 360) % 360;

    expect(screenAngle).toBe(0);
  });

  test('converts CSS slice starts to Remotion Pie rotation radians', () => {
    /**
     * @param {number} cssDegrees
     */
    const getRenderedStartDegrees = (cssDegrees) => {
      const rotationDegrees = (cssDegreesToPieRadians(cssDegrees) * 180) / Math.PI;
      return (270 + rotationDegrees + 360) % 360;
    };

    expect(getRenderedStartDegrees(0)).toBe(0);
    expect(getRenderedStartDegrees(45)).toBe(45);
    expect(getRenderedStartDegrees(90)).toBe(90);
    expect(getRenderedStartDegrees(180)).toBe(180);
    expect(getRenderedStartDegrees(270)).toBe(270);
  });

  test('starts from rest, rises monotonically, and ends exactly on target', () => {
    expect(getSpinProgress(0, 100)).toBe(0);
    expect(getSpinProgress(100, 100)).toBe(1);
    expect(getSpinProgress(125, 100)).toBe(1);

    let previous = -1;
    for (let frame = 0; frame <= 100; frame += 1) {
      const progress = getSpinProgress(frame, 100);
      expect(progress).toBeGreaterThanOrEqual(previous);
      previous = progress;
    }
  });

  /**
   * Per-frame rotation change over a whole spin, including the settle recoil.
   *
   * @param {number} optionCount
   * @param {number} spinFrames
   */
  const getPerFrameRotation = (optionCount, spinFrames) => {
    const sliceDegrees = getSliceDegrees(optionCount);
    const finalRotationDegrees = getFinalWheelRotationDegrees(
      optionCount,
      0,
      getSpinTurns(spinFrames, optionCount),
    );
    /** @param {number} at */
    const rotationAt = (at) =>
      getWheelRotationDegrees({
        finalRotationDegrees,
        frame: at,
        spinFrames,
        startRotationDegrees: -24,
      }) + getSpinRecoilDegrees(at, spinFrames, sliceDegrees);
    /** @type {number[]} */
    const deltas = [];

    for (let frame = 1; frame <= spinFrames; frame += 1) {
      deltas.push(Math.abs(rotationAt(frame) - rotationAt(frame - 1)));
    }

    return deltas;
  };

  test('picks an integer turn count within bounds for every configuration', () => {
    for (const spinFrames of [1, 36, 78, 98, 200]) {
      for (let optionCount = 2; optionCount <= 32; optionCount += 1) {
        const turns = getSpinTurns(spinFrames, optionCount);

        expect(Number.isInteger(turns)).toBe(true);
        expect(turns).toBeGreaterThanOrEqual(1);
        expect(turns).toBeLessThanOrEqual(7);
      }
    }
  });

  test('never advances a full slice in one frame when the frame budget allows it', () => {
    for (const spinFrames of [78, 98, 200]) {
      for (let optionCount = 2; optionCount <= 32; optionCount += 1) {
        const deltas = getPerFrameRotation(optionCount, spinFrames);

        expect(Math.max(...deltas)).toBeLessThan(getSliceDegrees(optionCount));
      }
    }
  });

  test('documents the dense short-duration wheels that cannot avoid aliasing', () => {
    // 3000ms at 12fps leaves 36 spin frames. Even a single turn outruns a thin
    // slice there, so this is a boundary of the format, not a tuning miss.
    // Asserted so the boundary cannot move without someone noticing.
    const spinFrames = 36;
    /** @param {number} optionCount */
    const aliases = (optionCount) =>
      Math.max(...getPerFrameRotation(optionCount, spinFrames)) >= getSliceDegrees(optionCount);

    // The cutoff sits at 17 options: one turn over 36 frames peaks at ~22.2deg
    // per frame, which is wider than a 17-slice wedge.
    for (let optionCount = 2; optionCount <= 16; optionCount += 1) {
      expect(aliases(optionCount)).toBe(false);
    }

    for (let optionCount = 17; optionCount <= 32; optionCount += 1) {
      expect(aliases(optionCount)).toBe(true);
    }
  });

  test('blends the launch into the deceleration without a speed jump', () => {
    const deltas = getPerFrameRotation(8, 98);

    // A slope mismatch at the handoff shows up as a single large step in
    // per-frame speed, which reads as the wheel getting a second push. The
    // launch ramp itself gains only a few degrees per frame.
    for (let index = 1; index < deltas.length; index += 1) {
      const previous = deltas[index - 1] ?? 0;
      const current = deltas[index] ?? 0;

      expect(current - previous).toBeLessThan(6);
    }
  });

  test('leaves only a few sub-perceptible frames before the wheel settles', () => {
    const deltas = getPerFrameRotation(8, 98);

    expect(deltas.filter((delta) => delta < 1.5).length).toBeLessThanOrEqual(8);
  });

  test('lands the winner exactly under the pointer at every option count', () => {
    for (const spinFrames of [36, 78, 98, 200]) {
      for (let optionCount = 2; optionCount <= 32; optionCount += 1) {
        const sliceDegrees = getSliceDegrees(optionCount);

        for (const winnerIndex of [0, 1, optionCount - 1]) {
          const finalRotationDegrees = getFinalWheelRotationDegrees(
            optionCount,
            winnerIndex,
            getSpinTurns(spinFrames, optionCount),
          );
          const restRotation =
            getWheelRotationDegrees({
              finalRotationDegrees,
              frame: spinFrames,
              spinFrames,
              startRotationDegrees: -24,
            }) + getSpinRecoilDegrees(spinFrames, spinFrames, sliceDegrees);
          const winnerCenter = getSliceCenterDegrees(optionCount, winnerIndex);
          const screenAngle = (((winnerCenter + restRotation) % 360) + 360) % 360;

          expect(screenAngle).toBeCloseTo(0);
        }
      }
    }
  });

  test('rocks into the detent and resolves to exactly zero', () => {
    const spinFrames = 98;
    const sliceDegrees = 45;

    expect(getSpinRecoilDegrees(spinFrames, spinFrames, sliceDegrees)).toBe(0);
    expect(getSpinRecoilDegrees(spinFrames + 5, spinFrames, sliceDegrees)).toBe(0);
    expect(getSpinRecoilDegrees(0, spinFrames, sliceDegrees)).toBe(0);

    /** @type {number[]} */
    const offsets = [];
    for (let frame = 0; frame <= spinFrames; frame += 1) {
      offsets.push(getSpinRecoilDegrees(frame, spinFrames, sliceDegrees));
    }

    // One full swing: past the detent, then back through it.
    expect(Math.max(...offsets)).toBeGreaterThan(0);
    expect(Math.min(...offsets)).toBeLessThan(0);
    // Never far enough to bring a neighbouring slice near the pointer.
    expect(Math.max(...offsets.map(Math.abs))).toBeLessThanOrEqual(sliceDegrees * 0.25);
  });

  test('scales the rock down so a sparse wheel does not swing', () => {
    // A 2-option wheel has 180deg slices; a quarter of that would be a lurch.
    const wide = getSpinRecoilDegrees(92, 98, 180);
    const narrow = getSpinRecoilDegrees(92, 98, 11.25);

    expect(Math.abs(wide)).toBeLessThanOrEqual(2.2);
    expect(Math.abs(narrow)).toBeLessThanOrEqual(11.25 * 0.25);
  });

  test('parks the pointer flapper neutral at rest for every winner', () => {
    for (const optionCount of [2, 3, 5, 6, 7, 8, 9, 12, 16, 32]) {
      for (let winnerIndex = 0; winnerIndex < optionCount; winnerIndex += 1) {
        const restRotation = getFinalWheelRotationDegrees(optionCount, winnerIndex);

        expect(getPointerDeflectionDegrees(restRotation, getSliceDegrees(optionCount))).toBe(0);
      }
    }
  });

  test('deflects the flapper as a slice edge passes and springs back', () => {
    const sliceDegrees = 45;

    expect(getPointerDeflectionDegrees(0, sliceDegrees)).toBe(-9);
    expect(getPointerDeflectionDegrees(sliceDegrees / 3, sliceDegrees)).toBe(0);
    expect(getPointerDeflectionDegrees(sliceDegrees * 0.75, sliceDegrees)).toBe(0);
    expect(getPointerDeflectionDegrees(-sliceDegrees, sliceDegrees)).toBe(-9);
  });
});

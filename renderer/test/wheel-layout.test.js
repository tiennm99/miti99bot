import {describe, expect, test} from 'vitest';
import {
  getFinalWheelRotationDegrees,
  getSliceCenterDegrees,
  getSliceDegrees,
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
});

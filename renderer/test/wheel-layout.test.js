import {describe, expect, test} from 'vitest';
import {
  cssDegreesToPieRadians,
  getFinalWheelRotationDegrees,
  getSliceCenterDegrees,
  getSliceDegrees,
  getSpinProgress,
  getWheelRotationDegrees,
  rightPointerClipPath,
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

  test('eases spin progress once across the spin duration', () => {
    expect(getSpinProgress(0, 100)).toBe(0);
    expect(getSpinProgress(25, 100)).toBeCloseTo(0.578125);
    expect(getSpinProgress(50, 100)).toBeCloseTo(0.875);
    expect(getSpinProgress(100, 100)).toBe(1);
    expect(getSpinProgress(125, 100)).toBe(1);
  });

  test('maps eased spin progress to wheel rotation', () => {
    const rotation = getWheelRotationDegrees({
      finalRotationDegrees: 696,
      frame: 25,
      spinFrames: 100,
      startRotationDegrees: -24,
    });

    expect(rotation).toBeCloseTo(392.25);
  });

  test('defines a right-side pointer that points into the wheel', () => {
    expect(rightPointerClipPath).toBe('polygon(0 50%, 100% 0, 100% 100%)');
  });
});

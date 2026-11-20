import { describe, expect, it } from 'vitest';
import {
  clamp,
  clampOverlayCenter,
  computeBgAlpha,
  computeContainRect,
  computeSnapPosition,
  numericOr,
  ratioToVirtual,
  virtualToRatio
} from './geometryMath';

describe('geometryMath', () => {
  it('computes exact letterbox contain rect for 16:9 video in wider box', () => {
    const rect = computeContainRect(1000, 400, 1920, 1080);
    // 400 / 1080 < 1000 / 1920 => height-constrained
    expect(rect.height).toBeCloseTo(400, 4);
    expect(rect.width).toBeCloseTo((1920 * 400) / 1080, 4);
    expect(rect.top).toBeCloseTo(0, 4);
    expect(rect.left).toBeCloseTo((1000 - rect.width) / 2, 4);
    expect(rect.scale).toBeCloseTo(400 / 1080, 6);
  });

  it('computes exact pillarbox contain rect for 9:16 portrait video in 16:9 container', () => {
    const rect = computeContainRect(800, 450, 1080, 1920);
    expect(rect.height).toBeCloseTo(450, 4);
    expect(rect.width).toBeCloseTo(253.125, 4);
    expect(rect.left).toBeCloseTo(273.4375, 4);
    expect(rect.top).toBeCloseTo(0, 4);
    expect(rect.scale).toBeCloseTo(450 / 1920, 6);
  });

  it('computes contain rect for 1:1 square and 21:9 ultrawide videos', () => {
    const square = computeContainRect(800, 500, 1080, 1080);
    expect(square.width).toBeCloseTo(500, 4);
    expect(square.height).toBeCloseTo(500, 4);
    expect(square.left).toBeCloseTo(150, 4);
    expect(square.top).toBeCloseTo(0, 4);

    const ultra = computeContainRect(800, 600, 2560, 1080);
    expect(ultra.width).toBeCloseTo(800, 4);
    expect(ultra.height).toBeCloseTo(337.5, 4);
    expect(ultra.left).toBeCloseTo(0, 4);
    expect(ultra.top).toBeCloseTo(131.25, 4);
  });

  it('handles zero or invalid dimensions safely', () => {
    expect(computeContainRect(0, 400, 1920, 1080)).toEqual({
      left: 0,
      top: 0,
      width: 0,
      height: 0,
      scale: 1
    });
    expect(computeContainRect(800, 500, 0, 1080).scale).toBe(1);
  });

  it('preserves numeric zero without || fallback bugs', () => {
    expect(numericOr(0, 100)).toBe(0);
    expect(numericOr('0', 100)).toBe(0);
    expect(numericOr(0, 6)).toBe(0);
    expect(numericOr('', 6)).toBe(6);
    expect(numericOr(NaN, 100)).toBe(100);
    expect(numericOr(undefined, 32)).toBe(32);
  });

  it('roundtrips ratios and clamps overlay center cleanly', () => {
    const pos = ratioToVirtual(0.25, 0.75, 1080, 1920);
    expect(pos).toEqual({ x: 270, y: 1440 });

    const back = virtualToRatio(270, 1440, 1080, 1920);
    expect(back.xRatio).toBeCloseTo(0.25, 6);
    expect(back.yRatio).toBeCloseTo(0.75, 6);

    const clamped = clampOverlayCenter(-50, 2000, 100, 50, 1080, 1920);
    expect(clamped).toEqual({ x: 100, y: 1870 });
  });

  it('snaps magnetically to center guides when within threshold', () => {
    const snapped = computeSnapPosition(545, 955, 80, 30, 1080, 1920, 15);
    expect(snapped.x).toBe(540);
    expect(snapped.y).toBe(960);
    expect(snapped.guideX).toBe(540);
    expect(snapped.guideY).toBe(960);
  });

  it('computes combined bgAlpha accurately', () => {
    expect(computeBgAlpha(0, 60)).toBe(0);
    expect(computeBgAlpha(50, 60)).toBeCloseTo(0.3, 6);
    expect(computeBgAlpha(100, 0)).toBe(0);
    expect(clamp(5, 0, 1)).toBe(1);
  });
});

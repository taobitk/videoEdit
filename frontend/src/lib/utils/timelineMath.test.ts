import { describe, expect, it } from 'vitest';
import {
  clampTrimSegment,
  computeKeepSegments,
  computeOutputDuration,
  findNextKeepTime,
  formatTime,
  isOverlayVisibleAtPct,
  mapSourceTimeToOutputPct,
  parseTime
} from './timelineMath';
import {
  buildSplitSegments,
  computeUniformSplitPoints,
  normalizeSplitMarkers
} from './splitMath';

describe('timelineMath', () => {
  it('formats floating seconds accurately without IEEE-754 off-by-one bug', () => {
    expect(formatTime(2.3)).toBe('00:02.30');
    expect(formatTime(0)).toBe('00:00.00');
    expect(formatTime(59.999)).toBe('01:00.00');
    expect(formatTime(3661.5)).toBe('01:01:01.50');
  });

  it('parses formatted strings and raw seconds cleanly', () => {
    expect(parseTime('00:02.30')).toBeCloseTo(2.3, 4);
    expect(parseTime('01:02:03.45')).toBeCloseTo(3723.45, 4);
    expect(parseTime('14.25')).toBeCloseTo(14.25, 4);
    expect(parseTime('-10')).toBe(0);
    expect(parseTime('')).toBe(0);
  });

  it('clamps end time cleanly after start enforcement', () => {
    expect(clampTrimSegment(10, 9, 10, 'end')).toEqual({ start: 9.9, end: 10 });
    expect(clampTrimSegment(4, 2, 10, 'end')).toEqual({ start: 4, end: 4.1 });
    expect(clampTrimSegment(6, 4, 10, 'start')).toEqual({ start: 3.9, end: 4 });
    expect(clampTrimSegment(-2, 15, 10, 'start')).toEqual({ start: 0, end: 10 });
  });

  it('merges overlapping excludes and inverts to valid keep segments', () => {
    const res = computeKeepSegments(10, true, [
      { start: 3, end: 5 },
      { start: 1, end: 3.0005 },
      { start: 8, end: 9.97 }
    ]);
    expect(res.mergedExclude).toEqual([
      { start: 1, end: 5 },
      { start: 8, end: 9.97 }
    ]);
    // [9.97, 10] is 0.03s < 0.05s so dropped!
    expect(res.keepSegments).toEqual([
      { start: 0, end: 1 },
      { start: 5, end: 8 }
    ]);
    expect(res.keptDuration).toBeCloseTo(4, 4);
    expect(res.isValid).toBe(true);
    expect(computeOutputDuration(res.keptDuration, 2)).toBeCloseTo(2, 4);
  });

  it('marks fully excluded video as invalid', () => {
    const res = computeKeepSegments(5, true, [{ start: 0, end: 4.98 }]);
    expect(res.isValid).toBe(false);
    expect(res.keepSegments).toEqual([]);
  });

  it('maps source time to output pct across keep spans and skips excluded spans', () => {
    const keeps = [
      { start: 0, end: 2 },
      { start: 5, end: 10 }
    ];
    expect(mapSourceTimeToOutputPct(1, keeps, 7)).toBeCloseTo((1 / 7) * 100, 4);
    expect(mapSourceTimeToOutputPct(7, keeps, 7)).toBeCloseTo((4 / 7) * 100, 4);
    expect(findNextKeepTime(3.2, keeps)).toBe(5);
    expect(findNextKeepTime(1.5, keeps)).toBeNull();
  });

  it('checks overlay visibility including zero endPct', () => {
    expect(isOverlayVisibleAtPct(0, 0, 0)).toBe(true);
    expect(isOverlayVisibleAtPct(5, 0, 0)).toBe(false);
    expect(isOverlayVisibleAtPct(100, 50, 100)).toBe(true);
    expect(isOverlayVisibleAtPct(40, 50, 100)).toBe(false);
  });

  it('sorts and deduplicates split markers cleanly', () => {
    const clean = normalizeSplitMarkers([15, 5, 5.05, 30, -1], 25);
    expect(clean).toEqual([5, 15]);
    const segs = buildSplitSegments(clean, 25);
    expect(segs).toHaveLength(3);
    expect(segs[0]).toEqual({ index: 1, start: 0, end: 5, duration: 5 });
    expect(segs[2]).toEqual({ index: 3, start: 15, end: 25, duration: 10 });
  });

  it('computes uniform split points by equal parts or fixed duration', () => {
    const byParts = computeUniformSplitPoints(30, 'parts', 3);
    expect(byParts.segmentDuration).toBeCloseTo(10, 4);
    expect(byParts.splitPoints).toEqual([10, 20]);

    const byDur = computeUniformSplitPoints(25, 'duration', 10);
    expect(byDur.splitPoints).toEqual([10, 20]);
  });
});

import { clamp } from './geometryMath';

export { formatTime, formatTimeShort, parseTime } from './timeFormat';

export interface TimeInterval {
  start: number;
  end: number;
}

export interface KeepSegmentsResult {
  keepSegments: TimeInterval[];
  mergedExclude: TimeInterval[];
  keptDuration: number;
  isValid: boolean;
}

const roundMs = (v: number): number => Math.round(v * 1000) / 1000;

/** Enforce 0 <= start < end <= duration cleanly after editing either endpoint. */
export function clampTrimSegment(
  start: number,
  end: number,
  duration: number,
  editedField: 'start' | 'end'
): TimeInterval {
  if (!Number.isFinite(duration) || duration <= 0) {
    return { start: 0, end: 0 };
  }
  const minGap = Math.min(0.1, duration);
  let s = clamp(start, 0, duration);
  let e = clamp(end, 0, duration);

  if (editedField === 'start') {
    if (s >= e) {
      s = Math.max(0, e - minGap);
      if (e <= s) e = Math.min(duration, s + minGap);
    }
  } else if (e <= s) {
    e = Math.min(duration, s + minGap);
    if (e <= s) s = Math.max(0, e - minGap);
  }

  return {
    start: clamp(roundMs(s), 0, duration),
    end: clamp(roundMs(e), 0, duration)
  };
}

/** Exclude-to-Keep inversion matching Go backend internal/mathutil/timeline.go. */
export function computeKeepSegments(
  duration: number,
  trimEnabled: boolean,
  excludeSegments: TimeInterval[]
): KeepSegmentsResult {
  if (!Number.isFinite(duration) || duration <= 0) {
    return { keepSegments: [], mergedExclude: [], keptDuration: 0, isValid: false };
  }
  if (!trimEnabled || excludeSegments.length === 0) {
    return {
      keepSegments: [{ start: 0, end: duration }],
      mergedExclude: [],
      keptDuration: duration,
      isValid: duration >= 0.05
    };
  }

  const normalized = excludeSegments
    .map((seg) => ({
      start: clamp(seg.start, 0, duration),
      end: clamp(seg.end, 0, duration)
    }))
    .filter((seg) => seg.end > seg.start)
    .sort((a, b) => a.start - b.start || a.end - b.end);

  const mergedExclude: TimeInterval[] = [];
  for (const seg of normalized) {
    const prev = mergedExclude[mergedExclude.length - 1];
    if (!prev) {
      mergedExclude.push({ ...seg });
    } else if (seg.start <= prev.end + 0.001) {
      prev.end = Math.max(prev.end, seg.end);
    } else {
      mergedExclude.push({ ...seg });
    }
  }

  const rawKeep: TimeInterval[] = [];
  let cursor = 0;
  for (const ex of mergedExclude) {
    if (ex.start > cursor) rawKeep.push({ start: cursor, end: ex.start });
    cursor = Math.max(cursor, ex.end);
  }
  if (cursor < duration) rawKeep.push({ start: cursor, end: duration });

  const keepSegments = rawKeep.filter((seg) => seg.end - seg.start >= 0.05);
  const keptDuration = keepSegments.reduce((acc, s) => acc + (s.end - s.start), 0);

  return {
    keepSegments,
    mergedExclude,
    keptDuration,
    isValid: keepSegments.length > 0 && keptDuration >= 0.05
  };
}

export function computeOutputDuration(keptDuration: number, speed: number): number {
  if (!Number.isFinite(keptDuration) || keptDuration <= 0) return 0;
  if (!Number.isFinite(speed) || speed <= 0) return keptDuration;
  return keptDuration / speed;
}

/** Map current source player timestamp to [0..100]% on the kept output timeline. */
export function mapSourceTimeToOutputPct(
  sourceTime: number,
  keepSegments: TimeInterval[],
  keptDuration: number
): number {
  if (keptDuration <= 0 || keepSegments.length === 0) return 0;
  let elapsedKept = 0;
  for (const seg of keepSegments) {
    if (sourceTime <= seg.start) break;
    if (sourceTime >= seg.end) {
      elapsedKept += seg.end - seg.start;
    } else {
      elapsedKept += sourceTime - seg.start;
      break;
    }
  }
  return clamp((elapsedKept / keptDuration) * 100, 0, 100);
}

/** If sourceTime falls in an excluded span, return the start of the next kept span (or null). */
export function findNextKeepTime(
  sourceTime: number,
  keepSegments: TimeInterval[]
): number | null {
  for (const seg of keepSegments) {
    if (sourceTime < seg.start - 0.02) return seg.start;
    if (sourceTime >= seg.start - 0.02 && sourceTime <= seg.end + 0.02) return null;
  }
  return keepSegments.length > 0 ? keepSegments[0].start : null;
}

export function isOverlayVisibleAtPct(
  currentPct: number,
  startPct: number,
  endPct: number
): boolean {
  if (endPct >= 99.95) return currentPct >= startPct - 0.01;
  return currentPct >= startPct - 0.01 && currentPct <= endPct + 0.01;
}

/** Compute exclude trim bounds starting at playhead (min 1.0s or 5% of duration). */
export function computeCutSegmentBounds(playhead: number, duration: number): TimeInterval {
  const dur = Number.isFinite(duration) && duration > 0 ? duration : 10;
  const minGap = Math.min(0.1, dur);
  const span = Math.max(1.0, dur * 0.05);
  let start = clamp(playhead, 0, Math.max(0, dur - minGap));
  let end = Math.min(dur, start + span);
  if (end - start < minGap) start = Math.max(0, end - minGap);
  return { start: Math.round(start * 100) / 100, end: Math.round(end * 100) / 100 };
}

/** Compute shifted bounds when duplicating an exclude trim segment. */
export function computeDuplicateTrimBounds(seg: TimeInterval, duration: number): TimeInterval {
  const dur = Number.isFinite(duration) && duration > 0 ? duration : Math.max(seg.end + 2, 10);
  const span = Math.max(0.1, seg.end - seg.start);
  let start = clamp(seg.end + 0.2, 0, Math.max(0, dur - 0.1));
  let end = Math.min(dur, start + span);
  if (end - start < 0.1) {
    start = Math.max(0, seg.start - span - 0.2);
    end = Math.min(dur, start + span);
  }
  return { start: Math.round(start * 100) / 100, end: Math.round(end * 100) / 100 };
}

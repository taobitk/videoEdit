import { clamp } from './geometryMath';

export interface SplitSegmentInfo {
  index: number;
  start: number;
  end: number;
  duration: number;
}

export function normalizeSplitMarkers(
  markers: number[],
  duration: number,
  minGap = 0.2
): number[] {
  if (!Number.isFinite(duration) || duration <= minGap * 2) return [];
  const sorted = markers
    .filter((m) => Number.isFinite(m) && m > 0 && m < duration)
    .map((m) => Math.round(clamp(m, minGap, duration - minGap) * 10) / 10)
    .sort((a, b) => a - b);

  const deduped: number[] = [];
  for (const m of sorted) {
    const prev = deduped[deduped.length - 1];
    if (prev === undefined || m - prev >= minGap) {
      deduped.push(m);
    }
  }
  return deduped;
}

export function buildSplitSegments(markers: number[], duration: number): SplitSegmentInfo[] {
  if (!Number.isFinite(duration) || duration <= 0) return [];
  const clean = normalizeSplitMarkers(markers, duration);
  const points = [0, ...clean, duration];
  const segments: SplitSegmentInfo[] = [];
  for (let i = 0; i < points.length - 1; i++) {
    segments.push({
      index: i + 1,
      start: points[i],
      end: points[i + 1],
      duration: points[i + 1] - points[i]
    });
  }
  return segments;
}

export function computeUniformSplitPoints(
  duration: number,
  mode: 'parts' | 'duration',
  value: number
): { splitPoints: number[]; segmentDuration: number } {
  if (!Number.isFinite(duration) || duration <= 0.4 || !Number.isFinite(value) || value <= 0) {
    return { splitPoints: [], segmentDuration: Math.max(0, duration) };
  }
  if (mode === 'parts') {
    const parts = Math.max(2, Math.floor(value));
    const segDur = duration / parts;
    const pts: number[] = [];
    for (let i = 1; i < parts; i++) {
      pts.push(Math.round(i * segDur * 100) / 100);
    }
    return { splitPoints: normalizeSplitMarkers(pts, duration), segmentDuration: segDur };
  }
  const segDur = Math.max(0.5, value);
  const pts: number[] = [];
  for (let t = segDur; t < duration - 0.2; t += segDur) {
    pts.push(Math.round(t * 100) / 100);
  }
  return { splitPoints: normalizeSplitMarkers(pts, duration), segmentDuration: segDur };
}

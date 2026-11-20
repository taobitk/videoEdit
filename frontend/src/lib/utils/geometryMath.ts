export interface ContainRect {
  left: number;
  top: number;
  width: number;
  height: number;
  scale: number;
}

export interface SnapResult {
  x: number;
  y: number;
  guideX: number | null;
  guideY: number | null;
}

/** Clamp a numeric value safely between min and max. */
export function clamp(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) return min;
  if (min > max) return min;
  return Math.min(Math.max(value, min), max);
}

/**
 * Parse a numeric value without treating finite 0 as falsy.
 * Preserves 0 for opacity=0%, bgPadding=0px, and endPct=0%.
 */
export function numericOr(value: unknown, fallback: number): number {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : fallback;
  }
  if (typeof value === 'string') {
    const trimmed = value.trim();
    if (trimmed === '') return fallback;
    const parsed = Number(trimmed);
    return Number.isFinite(parsed) ? parsed : fallback;
  }
  return fallback;
}

/**
 * Compute exact object-contain rectangle inside a container of size (boxW, boxH)
 * for display video dimensions (videoW, videoH).
 */
export function computeContainRect(
  boxW: number,
  boxH: number,
  videoW: number,
  videoH: number
): ContainRect {
  if (
    !Number.isFinite(boxW) ||
    !Number.isFinite(boxH) ||
    !Number.isFinite(videoW) ||
    !Number.isFinite(videoH) ||
    boxW <= 0 ||
    boxH <= 0 ||
    videoW <= 0 ||
    videoH <= 0
  ) {
    return { left: 0, top: 0, width: 0, height: 0, scale: 1 };
  }

  const scale = Math.min(boxW / videoW, boxH / videoH);
  const width = videoW * scale;
  const height = videoH * scale;
  const left = (boxW - width) / 2;
  const top = (boxH - height) / 2;

  return { left, top, width, height, scale };
}

/** Convert normalized center ratio [0..1] into virtual video pixel coordinates. */
export function ratioToVirtual(
  xRatio: number,
  yRatio: number,
  videoW: number,
  videoH: number
): { x: number; y: number } {
  return {
    x: clamp(xRatio, 0, 1) * Math.max(1, videoW),
    y: clamp(yRatio, 0, 1) * Math.max(1, videoH)
  };
}

/** Convert virtual video pixel center coordinates into clamped [0..1] ratios. */
export function virtualToRatio(
  x: number,
  y: number,
  videoW: number,
  videoH: number
): { xRatio: number; yRatio: number } {
  if (videoW <= 0 || videoH <= 0) {
    return { xRatio: 0.5, yRatio: 0.5 };
  }
  return {
    xRatio: clamp(x / videoW, 0, 1),
    yRatio: clamp(y / videoH, 0, 1)
  };
}

/** Keep overlay center inside video bounds cleanly. */
export function clampOverlayCenter(
  cx: number,
  cy: number,
  halfW: number,
  halfH: number,
  videoW: number,
  videoH: number
): { x: number; y: number } {
  const minX = halfW * 2 <= videoW ? halfW : 0;
  const maxX = halfW * 2 <= videoW ? videoW - halfW : videoW;
  const minY = halfH * 2 <= videoH ? halfH : 0;
  const maxY = halfH * 2 <= videoH ? videoH - halfH : videoH;

  return {
    x: clamp(cx, minX, maxX),
    y: clamp(cy, minY, maxY)
  };
}

/**
 * Snap center coordinates to canvas centerlines or frame edges when within thresholdPx.
 */
export function computeSnapPosition(
  cx: number,
  cy: number,
  halfW: number,
  halfH: number,
  videoW: number,
  videoH: number,
  thresholdPx = 12
): SnapResult {
  const clamped = clampOverlayCenter(cx, cy, halfW, halfH, videoW, videoH);
  let x = clamped.x;
  let y = clamped.y;
  let guideX: number | null = null;
  let guideY: number | null = null;

  const xCandidates = [
    { target: videoW / 2, guide: videoW / 2 },
    { target: halfW, guide: 0 },
    { target: videoW - halfW, guide: videoW }
  ];
  for (const cand of xCandidates) {
    if (Math.abs(x - cand.target) <= thresholdPx) {
      x = cand.target;
      guideX = cand.guide;
      break;
    }
  }

  const yCandidates = [
    { target: videoH / 2, guide: videoH / 2 },
    { target: halfH, guide: 0 },
    { target: videoH - halfH, guide: videoH }
  ];
  for (const cand of yCandidates) {
    if (Math.abs(y - cand.target) <= thresholdPx) {
      y = cand.target;
      guideY = cand.guide;
      break;
    }
  }

  const finalClamped = clampOverlayCenter(x, y, halfW, halfH, videoW, videoH);
  return { x: finalClamped.x, y: finalClamped.y, guideX, guideY };
}

/** Combine overall overlay opacity and box background opacity in [0..1]. */
export function computeBgAlpha(opacityPct: number, bgOpacityPct: number): number {
  const main = clamp(numericOr(opacityPct, 100), 0, 100) / 100;
  const bg = clamp(numericOr(bgOpacityPct, 60), 0, 100) / 100;
  return main * bg;
}

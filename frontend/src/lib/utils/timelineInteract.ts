import interact from 'interactjs';
import { clamp } from './geometryMath';

export interface TimelineClipInteractParams {
  disabled?: boolean;
  start: number;
  end: number;
  minBound: number;
  maxBound: number;
  minSpan: number;
  playheadValue: number;
  trackWidthPx: number;
  snapThresholdPx?: number;
  onStart?: () => void;
  onChange: (start: number, end: number, edge: 'move' | 'left' | 'right') => void;
  onCommit?: (start: number, end: number, edge: 'move' | 'left' | 'right') => void;
}

const RULER_STEPS = [0.1, 0.25, 0.5, 1, 2, 5, 10, 15, 30, 60, 120, 300, 600, 1800];
const round2 = (v: number): number => Math.round(v * 100) / 100;

/** Compute adaptive major tick positions for a zoomed timeline ruler. */
export function computeRulerTicks(
  duration: number,
  zoom: number
): { step: number; ticks: number[] } {
  if (!Number.isFinite(duration) || duration <= 0) return { step: 1, ticks: [0] };
  const visibleDur = duration / Math.max(1, zoom);
  let step = RULER_STEPS[RULER_STEPS.length - 1];
  for (const cand of RULER_STEPS) {
    if (visibleDur / cand <= 9) {
      step = cand;
      break;
    }
  }
  const ticks: number[] = [];
  const count = Math.floor(duration / step);
  for (let i = 0; i <= count; i++) {
    ticks.push(Math.round(i * step * 1000) / 1000);
  }
  if (ticks.length === 0 || duration - ticks[ticks.length - 1] > step * 0.25) {
    ticks.push(Math.round(duration * 1000) / 1000);
  }
  return { step, ticks };
}

/** Shift a [start, end] clip along x-axis with magnetic playhead snap. */
export function computeDraggedRange(
  initStart: number,
  initEnd: number,
  deltaPx: number,
  trackWidthPx: number,
  minBound: number,
  maxBound: number,
  playheadVal: number,
  snapPx = 6
): { start: number; end: number } {
  const span = Math.max(maxBound - minBound, 0.0001);
  const pxPerUnit = trackWidthPx / span;
  const clipLen = Math.max(0, initEnd - initStart);
  const deltaUnit = pxPerUnit > 0 ? deltaPx / pxPerUnit : 0;

  let nextStart = clamp(initStart + deltaUnit, minBound, Math.max(minBound, maxBound - clipLen));
  let nextEnd = nextStart + clipLen;

  if (pxPerUnit > 0 && Math.abs(nextStart - playheadVal) * pxPerUnit <= snapPx) {
    nextStart = clamp(playheadVal, minBound, Math.max(minBound, maxBound - clipLen));
    nextEnd = nextStart + clipLen;
  } else if (pxPerUnit > 0 && Math.abs(nextEnd - playheadVal) * pxPerUnit <= snapPx) {
    nextEnd = clamp(playheadVal, minBound + clipLen, maxBound);
    nextStart = nextEnd - clipLen;
  }

  return { start: round2(nextStart), end: round2(nextEnd) };
}

/** Resize left or right edge of a clip with magnetic playhead snap. */
export function computeResizedRange(
  edge: 'left' | 'right',
  initStart: number,
  initEnd: number,
  deltaPx: number,
  trackWidthPx: number,
  minBound: number,
  maxBound: number,
  minSpan: number,
  playheadVal: number,
  snapPx = 6
): { start: number; end: number } {
  const span = Math.max(maxBound - minBound, 0.0001);
  const pxPerUnit = trackWidthPx / span;
  const deltaUnit = pxPerUnit > 0 ? deltaPx / pxPerUnit : 0;

  if (edge === 'left') {
    const maxS = Math.max(minBound, initEnd - minSpan);
    let nextStart = clamp(initStart + deltaUnit, minBound, maxS);
    if (pxPerUnit > 0 && Math.abs(nextStart - playheadVal) * pxPerUnit <= snapPx) {
      nextStart = clamp(playheadVal, minBound, maxS);
    }
    return { start: round2(nextStart), end: round2(initEnd) };
  }

  const minE = Math.min(maxBound, initStart + minSpan);
  let nextEnd = clamp(initEnd + deltaUnit, minE, maxBound);
  if (pxPerUnit > 0 && Math.abs(nextEnd - playheadVal) * pxPerUnit <= snapPx) {
    nextEnd = clamp(playheadVal, minE, maxBound);
  }
  return { start: round2(initStart), end: round2(nextEnd) };
}

/** Svelte action wrapping interactjs draggable + edge-resizable clip behavior. */
export function timelineClipInteract(node: HTMLElement, initialParams: TimelineClipInteractParams) {
  let cfg = initialParams;
  let snap = { start: 0, end: 0, accumDx: 0, lastS: 0, lastE: 0, edge: 'move' as 'move' | 'left' | 'right' };

  const instance = interact(node)
    .draggable({
      enabled: !cfg.disabled,
      startAxis: 'x',
      lockAxis: 'x',
      listeners: {
        start() {
          snap = { start: cfg.start, end: cfg.end, accumDx: 0, lastS: cfg.start, lastE: cfg.end, edge: 'move' };
          cfg.onStart?.();
        },
        move(event) {
          snap.accumDx += event.dx;
          const next = computeDraggedRange(
            snap.start, snap.end, snap.accumDx, cfg.trackWidthPx,
            cfg.minBound, cfg.maxBound, cfg.playheadValue, cfg.snapThresholdPx ?? 6
          );
          snap.lastS = next.start;
          snap.lastE = next.end;
          cfg.onChange(next.start, next.end, 'move');
        },
        end() {
          cfg.onCommit?.(snap.lastS, snap.lastE, 'move');
        }
      }
    })
    .resizable({
      enabled: !cfg.disabled,
      edges: { left: '[data-edge="left"]', right: '[data-edge="right"]', top: false, bottom: false },
      axis: 'x',
      listeners: {
        start(event) {
          const edge: 'left' | 'right' = event.edges?.left ? 'left' : 'right';
          snap = { start: cfg.start, end: cfg.end, accumDx: 0, lastS: cfg.start, lastE: cfg.end, edge };
          cfg.onStart?.();
        },
        move(event) {
          snap.accumDx += event.dx;
          const edge = snap.edge === 'left' ? 'left' : 'right';
          const next = computeResizedRange(
            edge, snap.start, snap.end, snap.accumDx, cfg.trackWidthPx,
            cfg.minBound, cfg.maxBound, cfg.minSpan, cfg.playheadValue, cfg.snapThresholdPx ?? 6
          );
          snap.lastS = next.start;
          snap.lastE = next.end;
          cfg.onChange(next.start, next.end, edge);
        },
        end() {
          cfg.onCommit?.(snap.lastS, snap.lastE, snap.edge);
        }
      }
    });

  return {
    update(nextParams: TimelineClipInteractParams) {
      cfg = nextParams;
      instance.draggable({ enabled: !cfg.disabled });
      instance.resizable({ enabled: !cfg.disabled });
    },
    destroy() {
      instance.unset();
    }
  };
}

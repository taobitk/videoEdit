import { describe, expect, it, vi } from 'vitest';
import { handleMonitorKeydown, type MonitorKeyHandlers } from './monitorShortcuts';
import {
  computeDraggedRange,
  computeResizedRange,
  computeRulerTicks
} from './timelineInteract';
import {
  computeCutSegmentBounds,
  computeDuplicateTrimBounds
} from './timelineMath';

function makeKeyEvent(key: string, extra: Partial<KeyboardEvent> = {}) {
  const preventDefault = vi.fn();
  const stopPropagation = vi.fn();
  const evt = {
    key,
    code: extra.code ?? (key === ' ' ? 'Space' : ''),
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    altKey: false,
    isComposing: false,
    target: { tagName: 'DIV', isContentEditable: false },
    preventDefault,
    stopPropagation,
    ...extra
  } as unknown as KeyboardEvent;
  return { evt, preventDefault, stopPropagation };
}

function makeHandlers(overrides: Partial<MonitorKeyHandlers> = {}): MonitorKeyHandlers {
  return {
    activeTab: 'edit',
    hasVideo: true,
    isInlineTextEditing: false,
    selectedLayerId: null,
    selectedTrimId: null,
    displayW: 1920,
    displayH: 1080,
    onTogglePlay: vi.fn(),
    onSeekBy: vi.fn(),
    onBladeCut: vi.fn(),
    onSetIn: vi.fn(),
    onSetOut: vi.fn(),
    onAddText: vi.fn(),
    onPickImage: vi.fn(),
    onCopyLayer: vi.fn(),
    onPasteLayer: vi.fn(),
    onDuplicate: vi.fn(),
    onUndo: vi.fn(),
    onRedo: vi.fn(),
    onDeleteSelected: vi.fn(),
    onNudge: vi.fn(),
    onZoomIn: vi.fn(),
    onZoomOut: vi.fn(),
    onZoomReset: vi.fn(),
    ...overrides
  };
}

describe('NLE timeline math & interact range', () => {
  it('computes cut and duplicate trim bounds safely within video duration', () => {
    expect(computeCutSegmentBounds(2, 10)).toEqual({ start: 2, end: 3 });
    expect(computeCutSegmentBounds(9.95, 10)).toEqual({ start: 9.9, end: 10 });
    expect(computeDuplicateTrimBounds({ start: 1, end: 2.5 }, 10)).toEqual({
      start: 2.7,
      end: 4.2
    });
    expect(computeDuplicateTrimBounds({ start: 8.5, end: 9.9 }, 10)).toEqual({
      start: 6.9,
      end: 8.3
    });
  });

  it('computes adaptive ruler ticks as timeline zoom increases', () => {
    const z1 = computeRulerTicks(60, 1);
    const z20 = computeRulerTicks(60, 20);
    expect(z1.step).toBeGreaterThanOrEqual(5);
    expect(z20.step).toBeLessThan(z1.step);
    expect(z20.ticks[0]).toBe(0);
    expect(z20.ticks[z20.ticks.length - 1]).toBe(60);
  });

  it('drags and resizes clips with 6px magnetic snap to playhead', () => {
    // 100s over 1000px => 10 px/s. Playhead at 25s.
    // Drag clip [10, 15] by +146px => delta 14.6s => start 24.6s (diff 0.4s = 4px <= 6px -> snaps to 25s!)
    const snappedDrag = computeDraggedRange(10, 15, 146, 1000, 0, 100, 25, 6);
    expect(snappedDrag).toEqual({ start: 25, end: 30 });

    // Beyond 6px snap threshold (+160px => start 26s, diff 10px > 6px => no snap)
    const freeDrag = computeDraggedRange(10, 15, 160, 1000, 0, 100, 25, 6);
    expect(freeDrag).toEqual({ start: 26, end: 31 });

    // Resize right edge of [10, 20] by +45px => 24.5s (diff 5px <= 6px -> snaps to 25s!)
    const snappedResize = computeResizedRange('right', 10, 20, 45, 1000, 0, 100, 0.5, 25, 6);
    expect(snappedResize).toEqual({ start: 10, end: 25 });

    // Resize left edge clamped to minSpan
    const clampedLeft = computeResizedRange('left', 10, 12, 50, 1000, 0, 100, 0.5, 99, 6);
    expect(clampedLeft).toEqual({ start: 11.5, end: 12 });
  });
});

describe('NLE keyboard shortcuts', () => {
  it('ignores shortcuts inside form inputs or inline text editing', () => {
    const h = makeHandlers({ selectedLayerId: 'l1' });
    const inputEvt = makeKeyEvent(' ', {
      target: { tagName: 'INPUT', isContentEditable: false } as unknown as EventTarget
    });
    expect(handleMonitorKeydown(inputEvt.evt, h)).toBe(false);
    expect(h.onTogglePlay).not.toHaveBeenCalled();

    const editableEvt = makeKeyEvent('b', {
      ctrlKey: true,
      target: { tagName: 'DIV', isContentEditable: true } as unknown as EventTarget
    });
    expect(handleMonitorKeydown(editableEvt.evt, h)).toBe(false);
    expect(h.onBladeCut).not.toHaveBeenCalled();

    const inlineEvt = makeKeyEvent('Delete');
    expect(
      handleMonitorKeydown(inlineEvt.evt, makeHandlers({ isInlineTextEditing: true, selectedLayerId: 'l1' }))
    ).toBe(false);
  });

  it('prevents browser defaults and triggers transport, blade, in/out, copy/paste, zoom, and nudge', () => {
    const h = makeHandlers({ selectedLayerId: 'l1' });

    const space = makeKeyEvent(' ');
    expect(handleMonitorKeydown(space.evt, h)).toBe(true);
    expect(space.preventDefault).toHaveBeenCalled();
    expect(space.stopPropagation).toHaveBeenCalled();
    expect(h.onTogglePlay).toHaveBeenCalledTimes(1);

    const ctrlB = makeKeyEvent('b', { ctrlKey: true });
    expect(handleMonitorKeydown(ctrlB.evt, h)).toBe(true);
    expect(ctrlB.preventDefault).toHaveBeenCalled();
    expect(h.onBladeCut).toHaveBeenCalledTimes(1);

    expect(handleMonitorKeydown(makeKeyEvent('s').evt, h)).toBe(true);
    expect(h.onBladeCut).toHaveBeenCalledTimes(2);

    expect(handleMonitorKeydown(makeKeyEvent('[').evt, h)).toBe(true);
    expect(h.onSetIn).toHaveBeenCalledTimes(1);
    expect(handleMonitorKeydown(makeKeyEvent('o').evt, h)).toBe(true);
    expect(h.onSetOut).toHaveBeenCalledTimes(1);

    expect(handleMonitorKeydown(makeKeyEvent('t').evt, h)).toBe(true);
    expect(h.onAddText).toHaveBeenCalledTimes(1);
    expect(handleMonitorKeydown(makeKeyEvent('p').evt, h)).toBe(true);
    expect(h.onPickImage).toHaveBeenCalledTimes(1);

    expect(handleMonitorKeydown(makeKeyEvent('c', { ctrlKey: true }).evt, h)).toBe(true);
    expect(h.onCopyLayer).toHaveBeenCalledTimes(1);
    expect(handleMonitorKeydown(makeKeyEvent('v', { ctrlKey: true }).evt, h)).toBe(true);
    expect(h.onPasteLayer).toHaveBeenCalledTimes(1);
    expect(handleMonitorKeydown(makeKeyEvent('d', { ctrlKey: true }).evt, h)).toBe(true);
    expect(h.onDuplicate).toHaveBeenCalledTimes(1);

    expect(handleMonitorKeydown(makeKeyEvent('z', { ctrlKey: true }).evt, h)).toBe(true);
    expect(h.onUndo).toHaveBeenCalledTimes(1);
    expect(handleMonitorKeydown(makeKeyEvent('z', { ctrlKey: true, shiftKey: true }).evt, h)).toBe(true);
    expect(h.onRedo).toHaveBeenCalledTimes(1);

    expect(handleMonitorKeydown(makeKeyEvent('=', { ctrlKey: true }).evt, h)).toBe(true);
    expect(h.onZoomIn).toHaveBeenCalledTimes(1);
    expect(handleMonitorKeydown(makeKeyEvent('-', { ctrlKey: true }).evt, h)).toBe(true);
    expect(h.onZoomOut).toHaveBeenCalledTimes(1);
    expect(handleMonitorKeydown(makeKeyEvent('0', { ctrlKey: true }).evt, h)).toBe(true);
    expect(h.onZoomReset).toHaveBeenCalledTimes(1);

    expect(handleMonitorKeydown(makeKeyEvent('ArrowRight', { shiftKey: true }).evt, h)).toBe(true);
    expect(h.onNudge).toHaveBeenCalledWith(10, 0, 1920, 1080);

    // Without selected layer, ArrowLeft seeks playhead
    const hNoLayer = makeHandlers({ selectedLayerId: null });
    expect(handleMonitorKeydown(makeKeyEvent('ArrowLeft', { shiftKey: true }).evt, hNoLayer)).toBe(true);
    expect(hNoLayer.onSeekBy).toHaveBeenCalledWith(-5);
  });
});

import type { WorkspaceTab } from '$lib/types/studioTypes';

export interface MonitorKeyHandlers {
  activeTab: WorkspaceTab;
  hasVideo: boolean;
  isInlineTextEditing: boolean;
  selectedLayerId: string | null;
  selectedTrimId: string | null;
  displayW: number;
  displayH: number;
  onTogglePlay: () => void;
  onSeekBy: (deltaSec: number) => void;
  onBladeCut: () => void;
  onSetIn: () => void;
  onSetOut: () => void;
  onAddText: () => void;
  onPickImage: () => void;
  onCopyLayer: () => void;
  onPasteLayer: () => void;
  onDuplicate: () => void;
  onUndo: () => void;
  onRedo: () => void;
  onDeleteSelected: () => void;
  onNudge: (dx: number, dy: number, w: number, h: number) => void;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onZoomReset: () => void;
}

function isEditableTarget(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el) return false;
  const tag = el.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable;
}

function blockEvent(e: KeyboardEvent) {
  e.preventDefault();
  e.stopPropagation();
}

function hasWindowSelection(): boolean {
  if (typeof window === 'undefined' || !window.getSelection) return false;
  return Boolean(window.getSelection()?.toString());
}

export function handleMonitorKeydown(e: KeyboardEvent, opts: MonitorKeyHandlers): boolean {
  if (e.isComposing || opts.isInlineTextEditing || isEditableTarget(e.target)) return false;
  if (opts.activeTab === 'merge') return false;

  const mod = e.ctrlKey || e.metaKey;
  const key = e.key.toLowerCase();
  const isEdit = opts.activeTab === 'edit';

  if (mod && !e.altKey) {
    if (key === 'b') {
      blockEvent(e);
      if (opts.hasVideo) opts.onBladeCut();
      return true;
    }
    if (key === 'z' && isEdit) {
      blockEvent(e);
      if (e.shiftKey) opts.onRedo();
      else opts.onUndo();
      return true;
    }
    if (key === 'y' && isEdit) {
      blockEvent(e);
      opts.onRedo();
      return true;
    }
    if (key === 'c' && isEdit && opts.selectedLayerId && !hasWindowSelection()) {
      blockEvent(e);
      opts.onCopyLayer();
      return true;
    }
    if (key === 'v' && isEdit) {
      blockEvent(e);
      opts.onPasteLayer();
      return true;
    }
    if (key === 'd' && isEdit) {
      blockEvent(e);
      opts.onDuplicate();
      return true;
    }
    if (key === '=' || key === '+' || e.code === 'Equal' || e.code === 'NumpadAdd') {
      blockEvent(e);
      if (opts.hasVideo) opts.onZoomIn();
      return true;
    }
    if (key === '-' || e.code === 'Minus' || e.code === 'NumpadSubtract') {
      blockEvent(e);
      if (opts.hasVideo) opts.onZoomOut();
      return true;
    }
    if (key === '0' || e.code === 'Digit0' || e.code === 'Numpad0') {
      blockEvent(e);
      if (opts.hasVideo) opts.onZoomReset();
      return true;
    }
    return false;
  }

  if (e.altKey) return false;

  if (e.code === 'Space' || e.key === ' ') {
    if (!opts.hasVideo) return false;
    blockEvent(e);
    opts.onTogglePlay();
    return true;
  }

  if (key === 's' && !e.shiftKey && opts.hasVideo) {
    blockEvent(e);
    opts.onBladeCut();
    return true;
  }

  if ((e.key === '[' || (key === 'i' && !e.shiftKey)) && opts.hasVideo && isEdit) {
    blockEvent(e);
    opts.onSetIn();
    return true;
  }

  if ((e.key === ']' || (key === 'o' && !e.shiftKey)) && opts.hasVideo && isEdit) {
    blockEvent(e);
    opts.onSetOut();
    return true;
  }

  if (key === 't' && !e.shiftKey && isEdit) {
    blockEvent(e);
    opts.onAddText();
    return true;
  }

  if (key === 'p' && !e.shiftKey && isEdit) {
    blockEvent(e);
    opts.onPickImage();
    return true;
  }

  if ((e.key === 'Delete' || e.key === 'Backspace') && isEdit) {
    if (!opts.selectedLayerId && !opts.selectedTrimId) return false;
    blockEvent(e);
    opts.onDeleteSelected();
    return true;
  }

  if (isEdit && opts.selectedLayerId) {
    const step = e.shiftKey ? 10 : 1;
    if (e.key === 'ArrowLeft') { blockEvent(e); opts.onNudge(-step, 0, opts.displayW, opts.displayH); return true; }
    if (e.key === 'ArrowRight') { blockEvent(e); opts.onNudge(step, 0, opts.displayW, opts.displayH); return true; }
    if (e.key === 'ArrowUp') { blockEvent(e); opts.onNudge(0, -step, opts.displayW, opts.displayH); return true; }
    if (e.key === 'ArrowDown') { blockEvent(e); opts.onNudge(0, step, opts.displayW, opts.displayH); return true; }
  }

  if (opts.hasVideo && (e.key === 'ArrowLeft' || e.key === 'ArrowRight')) {
    blockEvent(e);
    const jump = e.shiftKey ? 5 : 1;
    opts.onSeekBy(e.key === 'ArrowLeft' ? -jump : jump);
    return true;
  }

  return false;
}

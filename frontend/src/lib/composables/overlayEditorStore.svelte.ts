import { selectFile } from '$lib/api/studioClient';
import type { ImageLayer, OverlayLayer, TextLayer } from '$lib/types/studioTypes';
import { clamp } from '$lib/utils/geometryMath';
import {
  buildImageLayer,
  buildTextLayer,
  toProcessLayerPayloads
} from '$lib/utils/overlayPayload';
import { computeClampedSnapRatios } from '$lib/utils/textRasterizer';

function cloneLayers(list: OverlayLayer[]): OverlayLayer[] {
  return list.map((item) => ({ ...item }));
}

export function createOverlayEditorStore() {
  let layers = $state<OverlayLayer[]>([]);
  let selectedLayerId = $state<string | null>(null);
  let copiedLayer = $state<OverlayLayer | null>(null);
  let editingTextLayerId = $state<string | null>(null);
  let undoStack = $state<OverlayLayer[][]>([]);
  let redoStack = $state<OverlayLayer[][]>([]);
  let errorMsg = $state('');

  const selectedLayer = $derived(
    layers.find((layer) => layer.id === selectedLayerId) ?? null
  );
  const canUndo = $derived(undoStack.length > 0);
  const canRedo = $derived(redoStack.length > 0);

  function commitHistory() {
    undoStack = [...undoStack.slice(-39), cloneLayers(layers)];
    redoStack = [];
  }

  function undo() {
    if (undoStack.length === 0) return;
    const prev = undoStack[undoStack.length - 1];
    redoStack = [...redoStack, cloneLayers(layers)];
    undoStack = undoStack.slice(0, -1);
    layers = cloneLayers(prev);
    if (selectedLayerId && !layers.some((l) => l.id === selectedLayerId)) {
      selectedLayerId = layers.length > 0 ? layers[layers.length - 1].id : null;
    }
  }

  function redo() {
    if (redoStack.length === 0) return;
    const next = redoStack[redoStack.length - 1];
    undoStack = [...undoStack, cloneLayers(layers)];
    redoStack = redoStack.slice(0, -1);
    layers = cloneLayers(next);
    if (selectedLayerId && !layers.some((l) => l.id === selectedLayerId)) {
      selectedLayerId = layers.length > 0 ? layers[layers.length - 1].id : null;
    }
  }

  function addTextLayer(videoHeight = 1080, startPct = 0) {
    commitHistory();
    const count = layers.filter((l) => l.type === 'text').length + 1;
    const next = buildTextLayer(videoHeight, count, startPct);
    layers = [...layers, next];
    selectedLayerId = next.id;
  }

  function addImageLayer(filename: string) {
    commitHistory();
    const next = buildImageLayer(filename);
    layers = [...layers, next];
    selectedLayerId = next.id;
  }

  async function pickAndAddImageLayer() {
    errorMsg = '';
    try {
      const res = await selectFile('image');
      if (res.success && res.path) addImageLayer(res.path);
    } catch (err) {
      errorMsg = err instanceof Error ? err.message : 'Không thể chọn ảnh phủ';
    }
  }

  function selectLayer(id: string | null) {
    selectedLayerId = id;
  }

  function updateLayer(
    id: string,
    patch: Partial<TextLayer> | Partial<ImageLayer>,
    recordHistory = false
  ) {
    if (recordHistory) commitHistory();
    layers = layers.map((layer) =>
      layer.id === id ? ({ ...layer, ...patch } as OverlayLayer) : layer
    );
  }

  function removeLayer(id: string) {
    commitHistory();
    layers = layers.filter((layer) => layer.id !== id);
    if (selectedLayerId === id) {
      selectedLayerId = layers.length > 0 ? layers[layers.length - 1].id : null;
    }
  }

  function moveLayerZ(id: string, direction: 'up' | 'down') {
    const idx = layers.findIndex((l) => l.id === id);
    if (idx === -1) return;
    const target = direction === 'up' ? idx + 1 : idx - 1;
    if (target < 0 || target >= layers.length) return;
    commitHistory();
    const next = [...layers];
    [next[idx], next[target]] = [next[target], next[idx]];
    layers = next;
  }

  function reorderLayer(fromId: string, toId: string) {
    if (fromId === toId) return;
    const fromIdx = layers.findIndex((l) => l.id === fromId);
    const toIdx = layers.findIndex((l) => l.id === toId);
    if (fromIdx === -1 || toIdx === -1) return;
    commitHistory();
    const next = [...layers];
    const [moved] = next.splice(fromIdx, 1);
    next.splice(toIdx, 0, moved);
    layers = next;
  }

  function nudgeSelected(dxPx: number, dyPx: number, videoW: number, videoH: number) {
    if (!selectedLayerId || videoW <= 0 || videoH <= 0) return;
    const layer = layers.find((l) => l.id === selectedLayerId);
    if (!layer) return;
    commitHistory();
    updateLayer(selectedLayerId, {
      xRatio: clamp(layer.xRatio + dxPx / videoW, 0, 1),
      yRatio: clamp(layer.yRatio + dyPx / videoH, 0, 1)
    });
  }

  function copySelectedLayer() {
    if (selectedLayer) copiedLayer = { ...selectedLayer };
  }

  function pasteLayer(preferSelected = false) {
    const src = preferSelected ? (selectedLayer ?? copiedLayer) : (copiedLayer ?? selectedLayer);
    if (!src) return;
    commitHistory();
    const next: OverlayLayer = {
      ...src,
      id: `${src.type}-${Date.now()}-${Math.random().toString(36).slice(2, 5)}`,
      xRatio: clamp(Math.round((src.xRatio + 0.03) * 1000) / 1000, 0, 1),
      yRatio: clamp(Math.round((src.yRatio + 0.03) * 1000) / 1000, 0, 1)
    };
    layers = [...layers, next];
    selectedLayerId = next.id;
    if (!preferSelected) copiedLayer = { ...next };
  }

  function setLayerInOutAtPct(edge: 'start' | 'end', pct: number, layerId?: string) {
    const target = (layerId ? layers.find((l) => l.id === layerId) : null) ?? selectedLayer;
    if (!target) return;
    const p = Math.round(clamp(pct, 0, 100) * 10) / 10;
    const patch = edge === 'start'
      ? { startPct: p, endPct: p > target.endPct ? p : target.endPct }
      : { endPct: p, startPct: p < target.startPct ? p : target.startPct };
    updateLayer(target.id, patch, true);
  }

  function snapSelectedLayerTo(xRatio: number, yRatio: number, videoW = 1920, videoH = 1080) {
    if (!selectedLayer) return;
    const clamped = computeClampedSnapRatios(selectedLayer, xRatio, yRatio, videoW, videoH);
    updateLayer(selectedLayer.id, clamped, true);
  }

  function startInlineTextEdit(id: string) {
    if (layers.some((l) => l.id === id && l.type === 'text')) {
      selectedLayerId = id;
      editingTextLayerId = id;
    }
  }

  function stopInlineTextEdit() { editingTextLayerId = null; }

  return {
    get layers() { return layers; },
    get selectedLayerId() { return selectedLayerId; },
    get selectedLayer() { return selectedLayer; },
    get copiedLayer() { return copiedLayer; },
    get editingTextLayerId() { return editingTextLayerId; },
    get canUndo() { return canUndo; },
    get canRedo() { return canRedo; },
    get errorMsg() { return errorMsg; },
    commitHistory, undo, redo, addTextLayer, addImageLayer, pickAndAddImageLayer,
    selectLayer, updateLayer, removeLayer, moveLayerZ, reorderLayer, nudgeSelected,
    copySelectedLayer, pasteLayer, setLayerInOutAtPct, snapSelectedLayerTo,
    startInlineTextEdit, stopInlineTextEdit,
    toProcessPayloads: (videoW: number) => toProcessLayerPayloads(layers, videoW)
  };
}

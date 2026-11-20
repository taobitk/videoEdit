<script lang="ts">
  import { tick, untrack } from 'svelte';
  import Konva from 'konva';
  import type { ImageLayer, OverlayLayer, TextLayer } from '$lib/types/studioTypes';
  import {
    clamp,
    computeSnapPosition,
    type ContainRect,
    virtualToRatio
  } from '$lib/utils/geometryMath';
  import { buildLayerNode, createOverlayTransformer, drawSnapGuides } from '$lib/utils/konvaNodeSync';
  import { isOverlayVisibleAtPct } from '$lib/utils/timelineMath';

  interface Props {
    containRect: ContainRect;
    videoW: number;
    videoH: number;
    layers: OverlayLayer[];
    selectedLayerId: string | null;
    currentOutputPct: number;
    interactive?: boolean;
    onSelectLayer: (id: string | null) => void;
    onCommitHistory: () => void;
    onUpdateLayer: (id: string, patch: Partial<TextLayer> | Partial<ImageLayer>) => void;
    onStartInlineEdit?: (id: string) => void;
    onStopInlineEdit?: () => void;
  }

  let {
    containRect, videoW, videoH, layers, selectedLayerId, currentOutputPct,
    interactive = true, onSelectLayer, onCommitHistory, onUpdateLayer,
    onStartInlineEdit, onStopInlineEdit
  }: Props = $props();

  let containerEl = $state<HTMLDivElement | null>(null);
  let inlineEl = $state<HTMLTextAreaElement | null>(null);
  let stage: Konva.Stage | null = null;
  let mainLayer: Konva.Layer | null = null;
  let guideLayer: Konva.Layer | null = null;
  let transformer: Konva.Transformer | null = null;
  let isInteracting = false;
  let imageLoadTick = $state(0);
  let inlineEdit = $state<{
    id: string; orig: string; draft: string;
    left: number; top: number; width: number; height: number; fontSize: number; color: string;
  } | null>(null);
  const imageCache = new Map<string, HTMLImageElement>();

  async function openInlineEditor(layer: TextLayer, node: Konva.Group) {
    const s = Math.max(0.01, containRect.scale);
    const w = clamp(node.width() * s + 28, 120, Math.max(120, containRect.width - 8));
    const h = clamp(node.height() * s + 16, 44, Math.max(44, containRect.height - 8));
    const left = clamp(node.x() * s - w / 2, 4, Math.max(4, containRect.width - w - 4));
    const top = clamp(node.y() * s - h / 2, 4, Math.max(4, containRect.height - h - 4));
    inlineEdit = {
      id: layer.id, orig: layer.text, draft: layer.text,
      left, top, width: w, height: h, fontSize: Math.max(12, Math.round(layer.fontSize * s)),
      color: layer.fontColor || '#ffffff'
    };
    onStartInlineEdit?.(layer.id);
    await tick();
    inlineEl?.focus();
    inlineEl?.select();
  }

  function commitInlineEdit() {
    if (!inlineEdit) return;
    const { id, orig, draft } = inlineEdit;
    inlineEdit = null;
    onStopInlineEdit?.();
    if (draft !== orig) {
      onCommitHistory();
      onUpdateLayer(id, { text: draft });
    }
  }

  function cancelInlineEdit() {
    if (!inlineEdit) return;
    inlineEdit = null;
    onStopInlineEdit?.();
  }

  function attachNodeHandlers(node: Konva.Group, layer: OverlayLayer) {
    node.on('mousedown touchstart', (e) => { e.cancelBubble = true; onSelectLayer(layer.id); });
    node.on('dblclick dbltap', (e) => {
      e.cancelBubble = true;
      if (layer.type === 'text' && interactive) void openInlineEditor(layer, node);
    });

    node.on('dragstart', () => {
      isInteracting = true;
      onSelectLayer(layer.id);
      onCommitHistory();
    });

    node.on('dragmove', () => {
      if (!guideLayer) return;
      const halfW = (node.width() * node.scaleX()) / 2;
      const halfH = (node.height() * node.scaleY()) / 2;
      const thresh = 10 / Math.max(containRect.scale, 0.1);
      const snapped = computeSnapPosition(
        node.x(),
        node.y(),
        halfW,
        halfH,
        videoW,
        videoH,
        thresh
      );
      node.position({ x: snapped.x, y: snapped.y });
      drawSnapGuides(guideLayer, snapped.guideX, snapped.guideY, videoW, videoH, containRect.scale);
    });

    node.on('dragend', () => {
      isInteracting = false;
      guideLayer?.destroyChildren();
      guideLayer?.batchDraw();
      const { xRatio, yRatio } = virtualToRatio(node.x(), node.y(), videoW, videoH);
      onUpdateLayer(layer.id, { xRatio, yRatio });
    });

    node.on('transformstart', () => {
      isInteracting = true;
      onCommitHistory();
    });

    node.on('transformend', () => {
      isInteracting = false;
      const sx = node.scaleX();
      const sy = node.scaleY();
      node.scaleX(1);
      node.scaleY(1);
      const { xRatio, yRatio } = virtualToRatio(node.x(), node.y(), videoW, videoH);
      if (layer.type === 'image') {
        const widthRatio = clamp((node.width() * sx) / videoW, 0.02, 1);
        onUpdateLayer(layer.id, { xRatio, yRatio, widthRatio });
      } else {
        const fontSize = clamp(Math.round(layer.fontSize * Math.max(sx, sy)), 10, 400);
        onUpdateLayer(layer.id, { xRatio, yRatio, fontSize });
      }
    });
  }

  $effect(() => {
    if (!containerEl) return;
    Konva.pixelRatio = typeof window !== 'undefined' ? window.devicePixelRatio || 1 : 1;
    stage = new Konva.Stage({
      container: containerEl,
      width: Math.max(1, containRect.width),
      height: Math.max(1, containRect.height)
    });
    mainLayer = new Konva.Layer();
    guideLayer = new Konva.Layer({ listening: false });
    transformer = createOverlayTransformer();

    mainLayer.add(transformer);
    stage.add(mainLayer);
    stage.add(guideLayer);
    stage.on('mousedown touchstart', (e) => {
      if (e.target === stage) onSelectLayer(null);
    });

    return () => {
      stage?.destroy();
      stage = null;
      mainLayer = null;
      guideLayer = null;
      transformer = null;
    };
  });

  $effect(() => {
    if (!stage || !mainLayer || !transformer) return;
    void imageLoadTick;
    const w = Math.max(1, containRect.width);
    const h = Math.max(1, containRect.height);
    const s = Math.max(0.01, containRect.scale);
    const nativeRatio = clamp(1 / s, 1, 6);
    mainLayer.getCanvas().setPixelRatio(nativeRatio);
    guideLayer?.getCanvas().setPixelRatio(nativeRatio);

    stage.width(w);
    stage.height(h);
    stage.scale({ x: s, y: s });

    if (isInteracting) return;

    mainLayer.getChildren((n) => n !== transformer).forEach((n) => n.destroy());
    const activeSelectedId = untrack(() => selectedLayerId);
    const activeInteractive = untrack(() => interactive);
    let selectedNode: Konva.Group | null = null;

    for (const layer of layers) {
      if (!layer.visible || layer.id === inlineEdit?.id) continue;
      if (!isOverlayVisibleAtPct(currentOutputPct, layer.startPct, layer.endPct)) continue;

      const group = buildLayerNode(layer, videoW, videoH, imageCache, () => {
        imageLoadTick += 1;
      });
      group.draggable(activeInteractive);
      attachNodeHandlers(group, layer);
      mainLayer.add(group);
      if (layer.id === activeSelectedId && activeInteractive) {
        selectedNode = group;
      }
    }

    transformer.nodes(selectedNode ? [selectedNode] : []);
    transformer.moveToTop();
    mainLayer.batchDraw();
  });

  $effect(() => {
    if (!mainLayer || !transformer) return;
    const id = selectedLayerId;
    const enabled = interactive;
    for (const child of mainLayer.getChildren((n) => n !== transformer)) {
      child.draggable(enabled);
    }
    const found =
      id && enabled
        ? (mainLayer.getChildren((n) => n !== transformer && n.id() === id)[0] as Konva.Group | undefined)
        : undefined;
    transformer.nodes(found ? [found] : []);
    transformer.moveToTop();
    mainLayer.batchDraw();
  });
</script>

<div
  class="absolute z-20"
  style="left: {containRect.left}px; top: {containRect.top}px; width: {containRect.width}px; height: {containRect.height}px;"
>
  <div bind:this={containerEl} data-testid="konva-stage-container" class="h-full w-full"></div>
  {#if inlineEdit}
    <textarea
      bind:this={inlineEl}
      data-testid="inline-text-editor"
      bind:value={inlineEdit.draft}
      onblur={commitInlineEdit}
      onkeydown={(e) => {
        if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); commitInlineEdit(); }
        else if (e.key === 'Escape') { e.preventDefault(); cancelInlineEdit(); }
      }}
      class="absolute z-30 resize-none rounded border border-purple-400 bg-black/80 px-2 py-1 text-center font-bold outline-none shadow-xl"
      style="left: {inlineEdit.left}px; top: {inlineEdit.top}px; width: {inlineEdit.width}px; height: {inlineEdit.height}px; font-size: {inlineEdit.fontSize}px; color: {inlineEdit.color};"
    ></textarea>
  {/if}
</div>

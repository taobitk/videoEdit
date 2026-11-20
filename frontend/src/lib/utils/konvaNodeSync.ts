import Konva from 'konva';
import { getPreviewUrl } from '$lib/api/studioClient';
import type { ImageLayer, OverlayLayer, TextLayer } from '$lib/types/studioTypes';
import { clamp, numericOr } from '$lib/utils/geometryMath';
import { buildTextGroupLayout, populateTextGroup } from '$lib/utils/textRasterizer';

export function createTextNodeGroup(layer: TextLayer, videoW: number, videoH: number): Konva.Group {
  const layout = buildTextGroupLayout(layer);
  const group = new Konva.Group({
    id: layer.id,
    name: 'overlay-node',
    x: clamp(layer.xRatio, 0, 1) * videoW,
    y: clamp(layer.yRatio, 0, 1) * videoH,
    width: layout.boxW,
    height: layout.boxH,
    offsetX: layout.boxW / 2,
    offsetY: layout.boxH / 2,
    draggable: true
  });
  group.setAttr('layerType', 'text');
  populateTextGroup(group, layout, true);
  return group;
}

export function createImageNodeGroup(
  layer: ImageLayer,
  videoW: number,
  videoH: number,
  imageCache: Map<string, HTMLImageElement>,
  onLoaded: () => void
): Konva.Group {
  const targetW = clamp(numericOr(layer.widthRatio, 0.2), 0.02, 1) * videoW;
  let img = imageCache.get(layer.filename);
  if (!img && layer.filename) {
    img = new window.Image();
    img.onload = () => onLoaded();
    img.src = getPreviewUrl(layer.filename);
    imageCache.set(layer.filename, img);
  }

  const loaded = Boolean(img && img.complete && img.naturalWidth > 0);
  const ratio = loaded && img ? img.naturalHeight / img.naturalWidth : 1;
  const targetH = Math.max(16, targetW * ratio);
  const opacity01 = clamp(numericOr(layer.opacity, 100), 0, 100) / 100;

  const group = new Konva.Group({
    id: layer.id,
    name: 'overlay-node',
    x: clamp(layer.xRatio, 0, 1) * videoW,
    y: clamp(layer.yRatio, 0, 1) * videoH,
    width: targetW,
    height: targetH,
    offsetX: targetW / 2,
    offsetY: targetH / 2,
    draggable: true
  });
  group.setAttr('layerType', 'image');

  if (loaded && img) {
    group.add(
      new Konva.Image({
        image: img,
        width: targetW,
        height: targetH,
        opacity: opacity01
      })
    );
  } else {
    group.add(
      new Konva.Rect({
        width: targetW,
        height: targetH,
        fill: 'rgba(139, 92, 246, 0.18)',
        stroke: '#a78bfa',
        strokeWidth: 2,
        dash: [6, 4],
        opacity: Math.max(0.25, opacity01)
      })
    );
  }
  return group;
}

export function drawSnapGuides(
  guideLayer: Konva.Layer,
  guideX: number | null,
  guideY: number | null,
  videoW: number,
  videoH: number,
  scale: number
) {
  guideLayer.destroyChildren();
  const strokeWidth = 1.5 / Math.max(scale, 0.1);
  if (guideX !== null) {
    guideLayer.add(
      new Konva.Line({
        points: [guideX, 0, guideX, videoH],
        stroke: '#22d3ee',
        strokeWidth,
        dash: [6 / Math.max(scale, 0.1), 4 / Math.max(scale, 0.1)]
      })
    );
  }
  if (guideY !== null) {
    guideLayer.add(
      new Konva.Line({
        points: [0, guideY, videoW, guideY],
        stroke: '#ec4899',
        strokeWidth,
        dash: [6 / Math.max(scale, 0.1), 4 / Math.max(scale, 0.1)]
      })
    );
  }
  guideLayer.batchDraw();
}

export function buildLayerNode(
  layer: OverlayLayer,
  videoW: number,
  videoH: number,
  imageCache: Map<string, HTMLImageElement>,
  onLoaded: () => void
): Konva.Group {
  if (layer.type === 'text') {
    return createTextNodeGroup(layer, videoW, videoH);
  }
  return createImageNodeGroup(layer, videoW, videoH, imageCache, onLoaded);
}

export function createOverlayTransformer(): Konva.Transformer {
  return new Konva.Transformer({
    keepRatio: true,
    rotateEnabled: false,
    flipEnabled: false,
    borderStroke: '#a78bfa',
    borderStrokeWidth: 1.5,
    anchorFill: '#ffffff',
    anchorStroke: '#8b5cf6',
    anchorSize: 8,
    anchorCornerRadius: 2,
    enabledAnchors: [
      'top-left', 'top-center', 'top-right', 'middle-right',
      'bottom-right', 'bottom-center', 'bottom-left', 'middle-left'
    ],
    boundBoxFunc: (oldBox, newBox) => (newBox.width < 12 || newBox.height < 12 ? oldBox : newBox)
  });
}

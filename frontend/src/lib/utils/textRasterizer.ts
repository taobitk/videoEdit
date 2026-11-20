import Konva from 'konva';
import type { ImageLayer, OverlayLayer, TextLayer } from '$lib/types/studioTypes';
import { clamp, clampOverlayCenter, computeBgAlpha, numericOr } from '$lib/utils/geometryMath';

export interface TextGroupLayout {
  textNode: Konva.Text;
  boxW: number;
  boxH: number;
  offsetX: number;
  offsetY: number;
  bgEnabled: boolean;
  bgColor: string;
  bgAlpha: number;
}

export interface RasterizedText {
  dataUrl: string;
  widthRatio: number;
  width: number;
  height: number;
}

const evenUp = (n: number, minVal = 10): number => {
  const v = Math.max(minVal, Math.ceil(n));
  return v % 2 === 0 ? v : v + 1;
};

export function hexToRgba(hex: string, alpha: number): string {
  const clean = /^#[0-9a-fA-F]{6}$/.test(hex) ? hex : '#000000';
  const r = parseInt(clean.slice(1, 3), 16);
  const g = parseInt(clean.slice(3, 5), 16);
  const b = parseInt(clean.slice(5, 7), 16);
  return `rgba(${r}, ${g}, ${b}, ${clamp(alpha, 0, 1)})`;
}

export function buildTextGroupLayout(layer: TextLayer): TextGroupLayout {
  const opacity01 = clamp(numericOr(layer.opacity, 100), 0, 100) / 100;
  const borderW = clamp(numericOr(layer.borderWidth, 2), 0, 30);
  const pad = layer.bgEnabled
    ? clamp(numericOr(layer.bgPadding, 6), 0, 100)
    : borderW > 0
      ? Math.ceil(borderW)
      : 0;

  const textNode = new Konva.Text({
    text: layer.text || ' ',
    fontSize: clamp(numericOr(layer.fontSize, 32), 10, 400),
    fontFamily: 'Arial, Helvetica, sans-serif',
    fontStyle: layer.fontWeight === 'bold' ? 'bold' : 'normal',
    align: 'center',
    fill: layer.fontColor,
    stroke: borderW > 0 ? layer.borderColor : undefined,
    strokeWidth: borderW * 2,
    lineJoin: 'round',
    fillAfterStrokeEnabled: true,
    opacity: opacity01
  });

  const textW = Math.max(20, textNode.width());
  const textH = Math.max(16, textNode.height());
  const boxW = evenUp(textW + pad * 2, 10);
  const boxH = evenUp(textH + pad * 2, 10);

  return {
    textNode,
    boxW,
    boxH,
    offsetX: (boxW - textW) / 2,
    offsetY: (boxH - textH) / 2,
    bgEnabled: layer.bgEnabled,
    bgColor: layer.bgColor,
    bgAlpha: computeBgAlpha(layer.opacity, layer.bgOpacity)
  };
}

export function populateTextGroup(
  group: Konva.Group,
  layout: TextGroupLayout,
  withHitArea: boolean
) {
  if (layout.bgEnabled) {
    group.add(
      new Konva.Rect({
        x: 0,
        y: 0,
        width: layout.boxW,
        height: layout.boxH,
        cornerRadius: 4,
        fill: hexToRgba(layout.bgColor, layout.bgAlpha)
      })
    );
  } else if (withHitArea) {
    group.add(
      new Konva.Rect({
        x: 0,
        y: 0,
        width: layout.boxW,
        height: layout.boxH,
        fill: 'rgba(0, 0, 0, 0.001)'
      })
    );
  }
  layout.textNode.position({ x: layout.offsetX, y: layout.offsetY });
  group.add(layout.textNode);
}

export function rasterizeTextLayer(layer: TextLayer, videoW: number): RasterizedText | null {
  if (typeof document === 'undefined' || !layer.text || layer.text.trim() === '') {
    return null;
  }
  const layout = buildTextGroupLayout(layer);
  const container = document.createElement('div');
  const offStage = new Konva.Stage({
    container,
    width: layout.boxW,
    height: layout.boxH
  });
  try {
    const offLayer = new Konva.Layer();
    const group = new Konva.Group({ x: 0, y: 0, width: layout.boxW, height: layout.boxH });
    populateTextGroup(group, layout, false);
    offLayer.add(group);
    offStage.add(offLayer);
    offLayer.draw();
    const dataUrl = offLayer.toDataURL({
      pixelRatio: 1,
      x: 0,
      y: 0,
      width: layout.boxW,
      height: layout.boxH
    });
    return {
      dataUrl,
      widthRatio: clamp(layout.boxW / Math.max(1, videoW), 0.01, 2),
      width: layout.boxW,
      height: layout.boxH
    };
  } finally {
    offStage.destroy();
  }
}

export function computeClampedSnapRatios(
  layer: OverlayLayer,
  targetXRatio: number,
  targetYRatio: number,
  videoW: number,
  videoH: number
): { xRatio: number; yRatio: number } {
  const safeW = Math.max(1, videoW);
  const safeH = Math.max(1, videoH);
  let halfW = 0;
  let halfH = 0;

  if (layer.type === 'text') {
    if (typeof document !== 'undefined') {
      const layout = buildTextGroupLayout(layer);
      halfW = layout.boxW / 2;
      halfH = layout.boxH / 2;
    }
  } else {
    const imgW = clamp(numericOr((layer as ImageLayer).widthRatio, 0.2), 0.02, 1) * safeW;
    halfW = imgW / 2;
    halfH = Math.min(safeH * 0.25, imgW / 2);
  }

  const clamped = clampOverlayCenter(
    clamp(targetXRatio, 0, 1) * safeW,
    clamp(targetYRatio, 0, 1) * safeH,
    halfW,
    halfH,
    safeW,
    safeH
  );
  return {
    xRatio: clamp(clamped.x / safeW, 0, 1),
    yRatio: clamp(clamped.y / safeH, 0, 1)
  };
}

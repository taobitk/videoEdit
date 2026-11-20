import type {
  ImageLayer,
  OverlayLayer,
  ProcessLayerPayload,
  TextLayer
} from '$lib/types/studioTypes';
import { clamp, numericOr } from './geometryMath';
import { rasterizeTextLayer } from './textRasterizer';

export function buildTextLayer(videoHeight: number, index: number, startPct = 0): TextLayer {
  const baseSize = clamp(Math.round(videoHeight * 0.04), 18, 96);
  const cleanStart = clamp(Math.round(startPct * 10) / 10, 0, 95);
  return {
    id: `text-${Date.now()}-${Math.random().toString(36).slice(2, 5)}`,
    type: 'text',
    visible: true,
    text: `Chữ mới ${index}`,
    fontSize: baseSize,
    fontWeight: 'bold',
    fontColor: '#ffffff',
    borderColor: '#000000',
    borderWidth: 2,
    opacity: 100,
    bgEnabled: false,
    bgColor: '#000000',
    bgOpacity: 60,
    bgPadding: 6,
    startPct: cleanStart,
    endPct: 100,
    xRatio: 0.5,
    yRatio: 0.5
  };
}

export function buildImageLayer(filename: string): ImageLayer {
  const displayName = filename.split(/[\\/]/).pop() || 'watermark.png';
  return {
    id: `img-${Date.now()}-${Math.random().toString(36).slice(2, 5)}`,
    type: 'image',
    visible: true,
    filename,
    displayName,
    widthRatio: 0.2,
    opacity: 100,
    startPct: 0,
    endPct: 100,
    xRatio: 0.5,
    yRatio: 0.5
  };
}

export function toProcessLayerPayloads(
  layers: OverlayLayer[],
  videoW: number
): ProcessLayerPayload[] {
  const out: ProcessLayerPayload[] = [];
  for (const layer of layers) {
    if (!layer.visible) continue;
    if (layer.type === 'text') {
      const raster = rasterizeTextLayer(layer, videoW);
      if (!raster) continue;
      out.push({
        type: 'image',
        raster_data_url: raster.dataUrl,
        width_ratio: raster.widthRatio,
        opacity: 100,
        start_pct: clamp(numericOr(layer.startPct, 0), 0, 100),
        end_pct: clamp(numericOr(layer.endPct, 100), 0, 100),
        x_ratio: clamp(numericOr(layer.xRatio, 0.5), 0, 1),
        y_ratio: clamp(numericOr(layer.yRatio, 0.5), 0, 1)
      });
    } else {
      out.push({
        type: 'image',
        filename: layer.filename,
        width_ratio: clamp(numericOr(layer.widthRatio, 0.2), 0.02, 1),
        opacity: clamp(numericOr(layer.opacity, 100), 0, 100),
        start_pct: clamp(numericOr(layer.startPct, 0), 0, 100),
        end_pct: clamp(numericOr(layer.endPct, 100), 0, 100),
        x_ratio: clamp(numericOr(layer.xRatio, 0.5), 0, 1),
        y_ratio: clamp(numericOr(layer.yRatio, 0.5), 0, 1)
      });
    }
  }
  return out;
}

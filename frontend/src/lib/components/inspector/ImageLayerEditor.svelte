<script lang="ts">
  import type { ImageLayer } from '$lib/types/studioTypes';
  import { clamp, numericOr } from '$lib/utils/geometryMath';
  import LayerTransformBar from './LayerTransformBar.svelte';

  interface Props {
    layer: ImageLayer;
    onUpdate: (patch: Partial<ImageLayer>) => void;
    onSnap: (xRatio: number, yRatio: number) => void;
    onSetIn: () => void;
    onSetOut: () => void;
    onFullSpan: () => void;
  }

  let { layer, onUpdate, onSnap, onSetIn, onSetOut, onFullSpan }: Props = $props();
</script>

<div class="flex flex-col gap-2.5 rounded-xl border border-cyan-500/25 bg-black/30 p-3 text-xs">
  <div class="flex items-center justify-between">
    <span class="font-bold text-cyan-300">Thuộc tính Ảnh</span>
    <span class="font-mono text-[10px] text-gray-400">
      x:{(layer.xRatio * 100).toFixed(0)}% y:{(layer.yRatio * 100).toFixed(0)}%
    </span>
  </div>

  <div class="truncate font-mono text-[10px] text-gray-400" title={layer.filename}>
    {layer.filename}
  </div>

  <div class="grid grid-cols-2 gap-2">
    <label class="flex flex-col gap-1">
      <span class="flex justify-between text-[10px] text-gray-400">
        <span>Chiều rộng</span>
        <span class="font-mono text-cyan-300">{Math.round(layer.widthRatio * 100)}%</span>
      </span>
      <input
        type="range"
        data-testid="input-image-width"
        min="5" max="100" step="1"
        value={Math.round(layer.widthRatio * 100)}
        oninput={(e) =>
          onUpdate({
            widthRatio: clamp(numericOr((e.currentTarget as HTMLInputElement).value, 20), 5, 100) / 100
          })}
        class="w-full accent-cyan-500 cursor-pointer"
      />
    </label>

    <label class="flex flex-col gap-1">
      <span class="flex justify-between text-[10px] text-gray-400">
        <span>Độ mờ</span>
        <span class="font-mono text-cyan-300">{layer.opacity}%</span>
      </span>
      <input
        type="range"
        data-testid="input-image-opacity"
        min="0" max="100" step="5"
        value={layer.opacity}
        oninput={(e) =>
          onUpdate({
            opacity: clamp(numericOr((e.currentTarget as HTMLInputElement).value, 100), 0, 100)
          })}
        class="w-full accent-cyan-500 cursor-pointer"
      />
    </label>
  </div>

  <LayerTransformBar
    xRatio={layer.xRatio}
    yRatio={layer.yRatio}
    startPct={layer.startPct}
    endPct={layer.endPct}
    accent="cyan"
    {onSnap} {onSetIn} {onSetOut} {onFullSpan}
  />
</div>

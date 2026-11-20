<script lang="ts">
  import { Bold, Box } from 'lucide-svelte';
  import type { TextLayer } from '$lib/types/studioTypes';
  import { clamp, numericOr } from '$lib/utils/geometryMath';
  import LayerTransformBar from './LayerTransformBar.svelte';

  interface Props {
    layer: TextLayer;
    onUpdate: (patch: Partial<TextLayer>) => void;
    onSnap: (xRatio: number, yRatio: number) => void;
    onSetIn: () => void;
    onSetOut: () => void;
    onFullSpan: () => void;
  }

  let { layer, onUpdate, onSnap, onSetIn, onSetOut, onFullSpan }: Props = $props();

  const SIZE_PRESETS = [
    { label: 'S', size: 24 },
    { label: 'M', size: 36 },
    { label: 'L', size: 52 },
    { label: 'XL', size: 72 }
  ];
</script>

<div class="flex flex-col gap-2.5 rounded-xl border border-purple-500/25 bg-black/30 p-3 text-xs">
  <div class="flex items-center justify-between">
    <span class="font-bold text-purple-300">Thuộc tính Chữ</span>
    <span data-testid="text-coords-label" class="font-mono text-[10px] text-gray-400">
      x:{(layer.xRatio * 100).toFixed(0)}% y:{(layer.yRatio * 100).toFixed(0)}%
    </span>
  </div>

  <label class="flex flex-col gap-1">
    <span class="text-[10px] font-semibold text-gray-400">Nội dung (nhấp đúp trên video để sửa)</span>
    <textarea
      data-testid="input-text-content"
      rows="2"
      value={layer.text}
      oninput={(e) => onUpdate({ text: (e.currentTarget as HTMLTextAreaElement).value })}
      class="w-full rounded-lg border border-white/10 bg-black/40 px-2.5 py-1.5 text-xs text-gray-100 outline-none focus:border-purple-500"
    ></textarea>
  </label>

  <div class="flex flex-col gap-1.5">
    <div class="flex items-center justify-between text-[10px] text-gray-400">
      <span>Cỡ chữ &amp; Kiểu</span>
      <span data-testid="text-size-display" class="font-mono text-purple-300">{layer.fontSize}px</span>
    </div>
    <div class="flex flex-wrap items-center gap-1">
      {#each SIZE_PRESETS as p (p.label)}
        <button
          type="button"
          data-testid={`btn-font-${p.label}`}
          onclick={() => onUpdate({ fontSize: p.size })}
          class="rounded border px-2 py-1 font-mono text-[10px] font-bold {layer.fontSize === p.size
            ? 'border-purple-400 bg-purple-500/25 text-purple-100'
            : 'border-white/10 bg-white/5 text-gray-300 hover:bg-white/10'}"
        >
          {p.label}
        </button>
      {/each}
      <button
        type="button"
        data-testid="btn-font-dec"
        onclick={() => onUpdate({ fontSize: clamp(layer.fontSize - 4, 10, 400) })}
        class="rounded border border-white/10 bg-white/5 px-1.5 py-1 font-mono text-[10px] text-gray-200 hover:bg-white/15"
      >
        A-
      </button>
      <button
        type="button"
        data-testid="btn-font-inc"
        onclick={() => onUpdate({ fontSize: clamp(layer.fontSize + 4, 10, 400) })}
        class="rounded border border-white/10 bg-white/5 px-1.5 py-1 font-mono text-[10px] text-gray-200 hover:bg-white/15"
      >
        A+
      </button>

      <button
        type="button"
        role="checkbox"
        aria-checked={layer.fontWeight === 'bold'}
        data-testid="toggle-text-bold"
        onclick={() => onUpdate({ fontWeight: layer.fontWeight === 'bold' ? 'normal' : 'bold' })}
        title="Chữ đậm (Bold)"
        class="ml-auto flex items-center gap-1 rounded border px-2 py-1 text-[10px] font-bold {layer.fontWeight === 'bold'
          ? 'border-purple-400 bg-purple-500/30 text-white'
          : 'border-white/10 bg-white/5 text-gray-400'}"
      >
        <Bold class="h-3 w-3" />
      </button>
      <button
        type="button"
        role="checkbox"
        aria-checked={layer.bgEnabled}
        data-testid="toggle-text-bg"
        onclick={() => onUpdate({ bgEnabled: !layer.bgEnabled })}
        title="Bật / Tắt hộp nền"
        class="flex items-center gap-1 rounded border px-2 py-1 text-[10px] font-semibold {layer.bgEnabled
          ? 'border-cyan-400 bg-cyan-500/25 text-cyan-100'
          : 'border-white/10 bg-white/5 text-gray-400'}"
      >
        <Box class="h-3 w-3" /> Nền
      </button>
    </div>
  </div>

  <div class="grid grid-cols-4 gap-2 border-t border-white/10 pt-2">
    <label class="flex flex-col gap-1">
      <span class="text-[10px] text-gray-400">Màu chữ</span>
      <input
        type="color"
        value={layer.fontColor}
        oninput={(e) => onUpdate({ fontColor: (e.currentTarget as HTMLInputElement).value })}
        class="h-6 w-full cursor-pointer rounded border border-white/15 bg-transparent"
      />
    </label>
    <label class="flex flex-col gap-1">
      <span class="text-[10px] text-gray-400">Viền</span>
      <input
        type="color"
        value={layer.borderColor}
        oninput={(e) => onUpdate({ borderColor: (e.currentTarget as HTMLInputElement).value })}
        class="h-6 w-full cursor-pointer rounded border border-white/15 bg-transparent"
      />
    </label>
    <label class="flex flex-col gap-1">
      <span class="flex justify-between text-[10px] text-gray-400">
        <span>Độ dày</span><span class="font-mono">{layer.borderWidth}</span>
      </span>
      <input
        type="range"
        min="0" max="15" step="1"
        value={layer.borderWidth}
        oninput={(e) => onUpdate({ borderWidth: clamp(numericOr((e.currentTarget as HTMLInputElement).value, 0), 0, 15) })}
        class="mt-1 w-full accent-purple-500 cursor-pointer"
      />
    </label>
    <label class="flex flex-col gap-1">
      <span class="flex justify-between text-[10px] text-gray-400">
        <span>Mờ</span><span class="font-mono text-purple-300">{layer.opacity}%</span>
      </span>
      <input
        type="range"
        data-testid="input-text-opacity"
        min="0" max="100" step="5"
        value={layer.opacity}
        oninput={(e) => onUpdate({ opacity: clamp(numericOr((e.currentTarget as HTMLInputElement).value, 100), 0, 100) })}
        class="mt-1 w-full accent-purple-500 cursor-pointer"
      />
    </label>
  </div>

  {#if layer.bgEnabled}
    <div class="grid grid-cols-3 gap-2 rounded-lg bg-black/30 p-2">
      <label class="flex flex-col gap-1">
        <span class="text-[10px] text-gray-400">Màu nền</span>
        <input
          type="color"
          value={layer.bgColor}
          oninput={(e) => onUpdate({ bgColor: (e.currentTarget as HTMLInputElement).value })}
          class="h-6 w-full cursor-pointer rounded bg-transparent"
        />
      </label>
      <label class="flex flex-col gap-1">
        <span class="text-[10px] text-gray-400">Mờ nền ({layer.bgOpacity}%)</span>
        <input
          type="range"
          min="0" max="100" step="5"
          value={layer.bgOpacity}
          oninput={(e) => onUpdate({ bgOpacity: clamp(numericOr((e.currentTarget as HTMLInputElement).value, 60), 0, 100) })}
          class="mt-1.5 w-full accent-purple-500"
        />
      </label>
      <label class="flex flex-col gap-1">
        <span class="text-[10px] text-gray-400">Đệm ({layer.bgPadding}px)</span>
        <input
          type="range"
          data-testid="input-text-bg-padding"
          min="0" max="30" step="1"
          value={layer.bgPadding}
          oninput={(e) => onUpdate({ bgPadding: clamp(numericOr((e.currentTarget as HTMLInputElement).value, 6), 0, 30) })}
          class="mt-1.5 w-full accent-purple-500"
        />
      </label>
    </div>
  {/if}

  <LayerTransformBar
    xRatio={layer.xRatio}
    yRatio={layer.yRatio}
    startPct={layer.startPct}
    endPct={layer.endPct}
    accent="purple"
    {onSnap} {onSetIn} {onSetOut} {onFullSpan}
  />
</div>

<script lang="ts">
  import { Sliders, Zap } from 'lucide-svelte';
  import type {
    CodecId,
    OverlayLayer,
    PresetId,
    QualityMode,
    SystemCapabilities,
    VideoFilterId
  } from '$lib/types/studioTypes';
  import { VIDEO_FILTER_OPTIONS } from '$lib/utils/colorFilters';
  import { numericOr } from '$lib/utils/geometryMath';
  import GpuEncoderSettings from './GpuEncoderSettings.svelte';
  import ImageLayerEditor from './ImageLayerEditor.svelte';
  import LayerListPanel from './LayerListPanel.svelte';
  import TextLayerEditor from './TextLayerEditor.svelte';

  interface Props {
    canStart: boolean;
    speed: number;
    videoFilter: VideoFilterId;
    useGpu: boolean;
    codec: CodecId;
    preset: PresetId;
    qualityMode: QualityMode;
    cq: number;
    capabilities: SystemCapabilities | null;
    layers: OverlayLayer[];
    selectedLayer: OverlayLayer | null;
    canUndo: boolean;
    canRedo: boolean;
    layerError: string;
    onSpeedChange: (v: number) => void;
    onFilterChange: (f: VideoFilterId) => void;
    onChangeGpu: (v: boolean) => void;
    onChangeCodec: (c: CodecId) => void;
    onChangePreset: (p: PresetId) => void;
    onChangeQualityMode: (q: QualityMode) => void;
    onChangeCq: (q: number) => void;
    onAddText: () => void;
    onPickImage: () => void;
    onAddImagePath: (p: string) => void;
    onSelectLayer: (id: string) => void;
    onMoveZ: (id: string, dir: 'up' | 'down') => void;
    onReorderLayer?: (fromId: string, toId: string) => void;
    onToggleVisible: (id: string, v: boolean) => void;
    onRemoveLayer: (id: string) => void;
    onUpdateLayer: (id: string, patch: Record<string, unknown>) => void;
    onSnapLayer: (xRatio: number, yRatio: number) => void;
    onSetLayerIn: () => void;
    onSetLayerOut: () => void;
    onLayerFullSpan: () => void;
    onUndo: () => void;
    onRedo: () => void;
    onStartRender: () => void;
  }

  let {
    canStart, speed, videoFilter, useGpu, codec, preset, qualityMode, cq, capabilities,
    layers, selectedLayer, canUndo, canRedo, layerError,
    onSpeedChange, onFilterChange, onChangeGpu, onChangeCodec, onChangePreset, onChangeQualityMode, onChangeCq,
    onAddText, onPickImage, onAddImagePath, onSelectLayer, onMoveZ, onReorderLayer, onToggleVisible,
    onRemoveLayer, onUpdateLayer, onSnapLayer, onSetLayerIn, onSetLayerOut, onLayerFullSpan,
    onUndo, onRedo, onStartRender
  }: Props = $props();

  const SPEED_PRESETS = [0.5, 0.8, 1.0, 1.2, 1.5, 2.0, 4.0];
</script>

<div class="flex flex-col gap-3.5">
  <div>
    <h2 class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-purple-300">
      <Sliders class="h-4 w-4" /> Studio Biên Tập
    </h2>
    <p class="mt-0.5 text-[11px] text-gray-400">Tốc độ, bộ lọc màu &amp; lớp phủ Konva</p>
  </div>

  <!-- Speed -->
  <div class="flex flex-col gap-2.5 rounded-xl border border-white/10 bg-white/5 p-3">
    <div class="flex items-center justify-between">
      <span class="text-xs font-semibold text-gray-300">Tốc độ phát</span>
      <span
        data-testid="speed-val-display"
        class="rounded-full bg-gradient-to-r from-purple-500 to-pink-500 px-2.5 py-0.5 font-mono text-xs font-bold text-white"
      >
        {speed.toFixed(2)}x
      </span>
    </div>

    <div class="grid grid-cols-7 gap-1">
      {#each SPEED_PRESETS as p (p)}
        <button
          type="button"
          onclick={() => onSpeedChange(p)}
          class="rounded-md border py-1 font-mono text-[11px] font-semibold transition-colors {Math.abs(speed - p) < 0.01
            ? 'border-pink-500/50 bg-pink-500/20 text-pink-200'
            : 'border-white/10 bg-black/25 text-gray-400 hover:text-white'}"
        >
          {p}x
        </button>
      {/each}
    </div>

    <input
      type="range"
      data-testid="speed-slider"
      min="0.25"
      max="8"
      step="0.05"
      value={speed}
      oninput={(e) => onSpeedChange(numericOr((e.currentTarget as HTMLInputElement).value, 1))}
      class="w-full accent-pink-500 cursor-pointer"
    />
    <div class="flex justify-between text-[10px] text-gray-500">
      <span>0.25x</span>
      <span>8.00x</span>
    </div>
  </div>

  <!-- Color Filter -->
  <label class="flex flex-col gap-1">
    <span class="text-xs font-semibold text-gray-300">Bộ lọc màu video</span>
    <select
      data-testid="select-video-filter"
      value={videoFilter}
      onchange={(e) => onFilterChange((e.currentTarget as HTMLSelectElement).value as VideoFilterId)}
      class="rounded-lg border border-white/10 bg-black/40 px-3 py-2 text-xs text-gray-100 outline-none focus:border-purple-500"
    >
      {#each VIDEO_FILTER_OPTIONS as opt (opt.id)}
        <option value={opt.id}>{opt.label}</option>
      {/each}
    </select>
  </label>

  <!-- Unified Z-order Layers -->
  <LayerListPanel
    {layers}
    selectedLayerId={selectedLayer?.id ?? null}
    {canUndo}
    {canRedo}
    errorMsg={layerError}
    {onAddText}
    {onPickImage}
    {onAddImagePath}
    onSelect={onSelectLayer}
    {onMoveZ}
    onReorder={onReorderLayer}
    {onToggleVisible}
    onRemove={onRemoveLayer}
    {onUndo}
    {onRedo}
  />

  {#if selectedLayer?.type === 'text'}
    <TextLayerEditor
      layer={selectedLayer}
      onUpdate={(patch) => onUpdateLayer(selectedLayer.id, patch)}
      onSnap={onSnapLayer}
      onSetIn={onSetLayerIn}
      onSetOut={onSetLayerOut}
      onFullSpan={onLayerFullSpan}
    />
  {:else if selectedLayer?.type === 'image'}
    <ImageLayerEditor
      layer={selectedLayer}
      onUpdate={(patch) => onUpdateLayer(selectedLayer.id, patch)}
      onSnap={onSnapLayer}
      onSetIn={onSetLayerIn}
      onSetOut={onSetLayerOut}
      onFullSpan={onLayerFullSpan}
    />
  {/if}

  <GpuEncoderSettings
    {useGpu} {codec} {preset} {qualityMode} {cq} {capabilities}
    {onChangeGpu} {onChangeCodec} {onChangePreset} {onChangeQualityMode} {onChangeCq}
  />

  <button
    type="button"
    data-testid="btn-start-process"
    disabled={!canStart}
    onclick={onStartRender}
    class="btn-neon mt-1 flex w-full items-center justify-center gap-2 rounded-xl py-3 text-xs font-bold tracking-wide text-white"
  >
    <Zap class="h-4 w-4" />
    ACCELERATE RENDER
  </button>
</div>

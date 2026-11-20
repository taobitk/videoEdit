<script lang="ts">
  import { Scissors } from 'lucide-svelte';
  import type { CodecId, PresetId, QualityMode, SystemCapabilities } from '$lib/types/studioTypes';
  import GpuEncoderSettings from './GpuEncoderSettings.svelte';

  interface Props {
    canStart: boolean;
    segmentCount: number;
    accurate: boolean;
    useGpu: boolean;
    codec: CodecId;
    preset: PresetId;
    qualityMode: QualityMode;
    capabilities: SystemCapabilities | null;
    onChangeAccurate: (v: boolean) => void;
    onChangeGpu: (v: boolean) => void;
    onChangeCodec: (c: CodecId) => void;
    onChangePreset: (p: PresetId) => void;
    onChangeQualityMode: (q: QualityMode) => void;
    onStartSplit: () => void;
  }

  let {
    canStart, segmentCount, accurate, useGpu, codec, preset, qualityMode, capabilities,
    onChangeAccurate, onChangeGpu, onChangeCodec, onChangePreset, onChangeQualityMode, onStartSplit
  }: Props = $props();
</script>

<div class="flex flex-col gap-4">
  <div>
    <h2 class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-purple-300">
      <Scissors class="h-4 w-4" /> Cấu hình Chia Video
    </h2>
    <p class="mt-0.5 text-[11px] text-gray-400">Chọn chế độ cắt nhanh hoặc chính xác từng frame</p>
  </div>

  <div class="rounded-xl border border-white/10 bg-white/5 p-3 text-xs">
    <div class="flex justify-between text-gray-300">
      <span>Số đoạn đầu ra dự kiến:</span>
      <strong class="font-mono text-purple-300">{segmentCount} đoạn</strong>
    </div>
  </div>

  <div class="flex flex-col gap-2">
    <button
      type="button"
      onclick={() => onChangeAccurate(false)}
      class="rounded-xl border p-3 text-left transition-colors {!accurate
        ? 'border-cyan-500/45 bg-cyan-500/15'
        : 'border-white/10 bg-white/5 hover:bg-white/10'}"
    >
      <div class="text-xs font-bold text-cyan-200">Siêu tốc (-c copy)</div>
      <div class="mt-0.5 text-[11px] text-gray-400">
        Không mã hóa lại, cắt theo keyframe gần nhất trong tích tắc.
      </div>
    </button>

    <button
      type="button"
      onclick={() => onChangeAccurate(true)}
      class="rounded-xl border p-3 text-left transition-colors {accurate
        ? 'border-purple-500/45 bg-purple-500/15'
        : 'border-white/10 bg-white/5 hover:bg-white/10'}"
    >
      <div class="text-xs font-bold text-purple-200">Chính xác từng khung hình (Re-encode)</div>
      <div class="mt-0.5 text-[11px] text-gray-400">
        Cắt chính xác tuyệt đối đến mili-giây bằng GPU/CPU encoder.
      </div>
    </button>
  </div>

  {#if accurate}
    <GpuEncoderSettings
      {useGpu} {codec} {preset} {qualityMode} {capabilities}
      {onChangeGpu} {onChangeCodec} {onChangePreset} {onChangeQualityMode}
    />
  {/if}

  <button
    type="button"
    data-testid="btn-start-split"
    disabled={!canStart}
    onclick={onStartSplit}
    class="btn-neon mt-2 flex w-full items-center justify-center gap-2 rounded-xl py-3 text-xs font-bold text-white"
  >
    <Scissors class="h-4 w-4" />
    BẮT ĐẦU CHIA VIDEO
  </button>
</div>

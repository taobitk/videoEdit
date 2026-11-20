<script lang="ts">
  import { Link2 } from 'lucide-svelte';
  import type { CodecId, PresetId, QualityMode, SystemCapabilities } from '$lib/types/studioTypes';
  import GpuEncoderSettings from './GpuEncoderSettings.svelte';

  interface Props {
    clipCount: number;
    useGpu: boolean;
    codec: CodecId;
    preset: PresetId;
    qualityMode: QualityMode;
    capabilities: SystemCapabilities | null;
    onChangeGpu: (v: boolean) => void;
    onChangeCodec: (c: CodecId) => void;
    onChangePreset: (p: PresetId) => void;
    onChangeQualityMode: (q: QualityMode) => void;
    onStartMerge: () => void;
  }

  let {
    clipCount, useGpu, codec, preset, qualityMode, capabilities,
    onChangeGpu, onChangeCodec, onChangePreset, onChangeQualityMode, onStartMerge
  }: Props = $props();
</script>

<div class="flex flex-col gap-4">
  <div>
    <h2 class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-cyan-300">
      <Link2 class="h-4 w-4" /> Cấu hình Ghép Video
    </h2>
    <p class="mt-0.5 text-[11px] text-gray-400">Nối tuần tự danh sách video thành một file MP4</p>
  </div>

  <div class="rounded-xl border border-white/10 bg-white/5 p-3 text-xs">
    <div class="flex justify-between text-gray-300">
      <span>Tổng số clip đã chọn:</span>
      <strong class="font-mono text-cyan-300">{clipCount} clip</strong>
    </div>
    <p class="mt-1.5 text-[11px] text-gray-400">
      Nếu cùng thông số codec/độ phân giải, hệ thống nối nhanh không giảm chất lượng; nếu khác, sẽ tự chuẩn hóa về clip đầu tiên.
    </p>
  </div>

  <GpuEncoderSettings
    {useGpu} {codec} {preset} {qualityMode} {capabilities}
    {onChangeGpu} {onChangeCodec} {onChangePreset} {onChangeQualityMode}
  />

  <button
    type="button"
    data-testid="btn-start-merge"
    disabled={clipCount < 2}
    onclick={onStartMerge}
    class="btn-neon mt-2 flex w-full items-center justify-center gap-2 rounded-xl py-3 text-xs font-bold text-white"
  >
    <Link2 class="h-4 w-4" />
    GHÉP TẤT CẢ VIDEO
  </button>
</div>

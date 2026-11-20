<script lang="ts">
  import { Minimize2, ShieldCheck, Sparkles } from 'lucide-svelte';
  import type { CodecId, PresetId, QualityMode, SystemCapabilities } from '$lib/types/studioTypes';

  interface Props {
    useGpu: boolean;
    codec: CodecId;
    preset: PresetId;
    qualityMode: QualityMode;
    capabilities: SystemCapabilities | null;
    onChangeGpu: (v: boolean) => void;
    onChangeCodec: (v: CodecId) => void;
    onChangePreset: (v: PresetId) => void;
    onChangeQualityMode: (q: QualityMode) => void;
  }

  let {
    useGpu, codec, preset, qualityMode, capabilities,
    onChangeGpu, onChangeCodec, onChangePreset, onChangeQualityMode
  }: Props = $props();

  const hasNvenc = $derived(
    Boolean(capabilities?.has_nvidia_gpu && (capabilities.nvenc_h264 || capabilities.nvenc_hevc))
  );

  const QUALITY_MODES: Array<{ id: QualityMode; label: string; hint: string }> = [
    { id: 'auto_cap', label: 'Chuẩn gốc', hint: 'Giữ nét & giới hạn dung lượng không phình to hơn file gốc' },
    { id: 'compact', label: 'Siêu nhẹ', hint: 'Nén gọn nhẹ tối đa (~60% bitrate gốc)' },
    { id: 'high', label: 'Nét cao', hint: 'Chất lượng tối đa, không giới hạn bitrate' }
  ];
</script>

<div class="flex flex-col gap-2.5 border-t border-white/10 pt-3">
  <div class="flex items-center justify-between">
    <div>
      <div class="text-xs font-semibold text-cyan-300">Tăng tốc GPU (NVENC)</div>
      <div class="text-[10px] text-gray-500">
        {hasNvenc ? 'NVIDIA NVENC khả dụng' : 'Không thấy GPU NVENC (tự fallback CPU)'}
      </div>
    </div>
    <input
      type="checkbox"
      data-testid="toggle-gpu"
      checked={useGpu}
      onchange={(e) => onChangeGpu((e.currentTarget as HTMLInputElement).checked)}
      class="h-4 w-4 accent-cyan-500 cursor-pointer"
    />
  </div>

  <div class="grid grid-cols-2 gap-2">
    <label class="flex flex-col gap-1">
      <span class="text-[10px] font-semibold text-gray-400">Codec</span>
      <select
        data-testid="select-codec"
        value={codec}
        onchange={(e) => onChangeCodec((e.currentTarget as HTMLSelectElement).value as CodecId)}
        class="rounded-lg border border-white/10 bg-black/40 px-2.5 py-1.5 text-xs text-gray-100 outline-none focus:border-purple-500"
      >
        <option value="h264">H.264 (Phổ biến)</option>
        <option value="hevc">HEVC (H.265 Nét cao)</option>
      </select>
    </label>

    <label class="flex flex-col gap-1">
      <span class="text-[10px] font-semibold text-gray-400">Preset</span>
      <select
        data-testid="select-preset"
        value={preset}
        onchange={(e) => onChangePreset((e.currentTarget as HTMLSelectElement).value as PresetId)}
        class="rounded-lg border border-white/10 bg-black/40 px-2.5 py-1.5 text-xs text-gray-100 outline-none focus:border-purple-500"
      >
        <option value="p1">p1 (Nhanh nhất)</option>
        <option value="p4">p4 (Cân bằng)</option>
        <option value="p7">p7 (Chất lượng cao)</option>
      </select>
    </label>
  </div>
</div>

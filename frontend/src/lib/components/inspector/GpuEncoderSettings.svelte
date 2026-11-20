<script lang="ts">
  import type { CodecId, PresetId, QualityMode, SystemCapabilities } from '$lib/types/studioTypes';

  interface Props {
    useGpu: boolean;
    codec: CodecId;
    preset: PresetId;
    qualityMode: QualityMode;
    cq: number;
    capabilities: SystemCapabilities | null;
    onChangeGpu: (v: boolean) => void;
    onChangeCodec: (v: CodecId) => void;
    onChangePreset: (v: PresetId) => void;
    onChangeQualityMode: (q: QualityMode) => void;
    onChangeCq: (q: number) => void;
  }

  let {
    useGpu, codec, preset, qualityMode, cq, capabilities,
    onChangeGpu, onChangeCodec, onChangePreset, onChangeQualityMode, onChangeCq
  }: Props = $props();

  const hasNvenc = $derived(
    Boolean(capabilities?.has_nvidia_gpu && (capabilities.nvenc_h264 || capabilities.nvenc_hevc))
  );
  const isAv1 = $derived(codec === 'av1');
  const qMin = $derived(isAv1 ? 24 : 20);
  const qMax = $derived(isAv1 ? 38 : 28);
  const qHint = $derived(
    isAv1
      ? cq <= 27 ? 'Siêu nét' : cq <= 32 ? 'Chuẩn YouTube' : 'Siêu nhẹ'
      : cq <= 21 ? 'Siêu nét' : cq <= 25 ? 'Nét & gọn' : 'Siêu nhẹ'
  );

  function handleCodecChange(next: CodecId) {
    const wasAv1 = codec === 'av1';
    onChangeCodec(next);
    if (next === 'av1' && !wasAv1) {
      const toSvt: Record<string, PresetId> = { p1: '10', p4: '8', p7: '6' };
      onChangePreset(toSvt[preset] ?? '8');
      onChangeCq(30);
    } else if (next !== 'av1' && wasAv1) {
      const toNvenc: Record<string, PresetId> = { '10': 'p1', '8': 'p4', '6': 'p7' };
      onChangePreset(toNvenc[preset] ?? 'p4');
      onChangeCq(20);
    }
    void qualityMode;
    onChangeQualityMode(next === 'av1' ? 'compact' : 'high');
  }
</script>

<div class="flex flex-col gap-2.5 border-t border-white/10 pt-3">
  <div class="flex items-center justify-between">
    <div>
      <div class="text-xs font-semibold text-cyan-300">
        {isAv1 ? 'Tăng tốc GPU + CPU SVT-AV1' : 'Tăng tốc GPU (NVENC)'}
      </div>
      <div class="text-[10px] text-gray-500">
        {#if isAv1}
          {hasNvenc ? 'RTX 3060 giải mã/lọc màu • CPU nén AV1' : 'CPU toàn phần (SVT-AV1)'}
        {:else}
          {hasNvenc ? 'NVIDIA NVENC khả dụng' : 'Không thấy GPU NVENC (tự fallback CPU)'}
        {/if}
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
        onchange={(e) => handleCodecChange((e.currentTarget as HTMLSelectElement).value as CodecId)}
        class="rounded-lg border border-white/10 bg-black/40 px-2.5 py-1.5 text-xs text-gray-100 outline-none focus:border-purple-500"
      >
        <option value="h264">H.264 (Phổ biến)</option>
        <option value="hevc">HEVC (H.265 Nét cao)</option>
        <option value="av1">AV1 (SVT-AV1 Siêu nhẹ)</option>
      </select>
    </label>

    <label class="flex flex-col gap-1">
      <span class="text-[10px] font-semibold text-gray-400">
        {isAv1 ? 'Preset SVT-AV1' : 'Preset NVENC'}
      </span>
      <select
        data-testid="select-preset"
        value={preset}
        onchange={(e) => onChangePreset((e.currentTarget as HTMLSelectElement).value as PresetId)}
        class="rounded-lg border border-white/10 bg-black/40 px-2.5 py-1.5 text-xs text-gray-100 outline-none focus:border-purple-500"
      >
        {#if isAv1}
          <option value="10">10 (Nhanh ~180 fps)</option>
          <option value="8">8 (Cân bằng ~110 fps)</option>
          <option value="6">6 (Nét & nhỏ ~67 fps)</option>
        {:else}
          <option value="p1">p1 (Nhanh nhất)</option>
          <option value="p4">p4 (Cân bằng)</option>
          <option value="p7">p7 (Chất lượng cao)</option>
        {/if}
      </select>
    </label>
  </div>

  <label class="flex flex-col gap-1 rounded-lg border border-white/10 bg-black/30 px-2.5 py-2">
    <div class="flex items-center justify-between text-[10px]">
      <span class="font-semibold text-gray-400">{isAv1 ? 'Chất lượng (CRF)' : 'Chất lượng (CQ)'}</span>
      <span data-testid="cq-val-display" class="font-mono font-semibold text-cyan-300">
        {isAv1 ? 'CRF' : 'CQ'} {cq} ({qHint})
      </span>
    </div>
    <input
      type="range"
      data-testid="input-cq"
      min={qMin}
      max={qMax}
      step="1"
      value={cq}
      oninput={(e) => onChangeCq(Number((e.currentTarget as HTMLInputElement).value))}
      class="w-full accent-cyan-500 cursor-pointer"
    />
    <div class="flex justify-between text-[9px] text-gray-500">
      {#if isAv1}
        <span>24 (Rất nét)</span>
        <span>30 (Chuẩn YT)</span>
        <span>38 (Nhẹ nhất)</span>
      {:else}
        <span>20 (Nét nhất)</span>
        <span>24 (Cân bằng)</span>
        <span>28 (Nhẹ nhất)</span>
      {/if}
    </div>
  </label>
</div>

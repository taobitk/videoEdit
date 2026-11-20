<script lang="ts">
  import { FolderOpen, Minus, Plus, Scissors, Trash2 } from 'lucide-svelte';
  import type { SplitMode, UniformSplitMode } from '$lib/types/studioTypes';
  import { clamp } from '$lib/utils/geometryMath';
  import type { SplitSegmentInfo } from '$lib/utils/splitMath';
  import { formatTime } from '$lib/utils/timelineMath';

  interface Props {
    path: string;
    duration: number;
    loading: boolean;
    error: string;
    mode: SplitMode;
    uniformType: UniformSplitMode;
    uniformParts: number;
    uniformDuration: number;
    segments: SplitSegmentInfo[];
    onBrowse: () => void;
    onLoadPath: (p: string) => void;
    onModeChange: (m: SplitMode) => void;
    onUniformTypeChange: (u: UniformSplitMode) => void;
    onUniformPartsChange: (n: number) => void;
    onUniformDurationChange: (s: number) => void;
    onAddMarker: () => void;
    onClearMarkers: () => void;
  }

  let {
    path, duration, loading, error, mode, uniformType, uniformParts, uniformDuration, segments,
    onBrowse, onLoadPath, onModeChange, onUniformTypeChange,
    onUniformPartsChange, onUniformDurationChange, onAddMarker, onClearMarkers
  }: Props = $props();

  let draft = $state('');
  $effect(() => { draft = path; });

  const PARTS_PRESETS = [2, 3, 4, 5, 10];
  const DUR_PRESETS = [
    { val: 15, label: '15s' },
    { val: 30, label: '30s' },
    { val: 60, label: '60s' },
    { val: 300, label: '5m' },
    { val: 600, label: '10m' }
  ];
  const maxUniformDur = $derived(Math.max(1, Math.floor(duration)));
  const durStep = $derived(uniformDuration <= 10 ? 1 : 5);
</script>

<div class="flex flex-col gap-3">
  <div class="rounded-lg border border-purple-500/20 bg-purple-500/10 px-3 py-2 text-[11px] text-gray-300">
    Chia video dài theo điểm cắt kéo thả trực quan hoặc chia đều tự động.
  </div>

  <button
    type="button"
    data-testid="btn-browse-split"
    onclick={onBrowse}
    class="btn-neon flex w-full items-center justify-center gap-2 rounded-xl px-4 py-2.5 text-xs font-bold text-white"
  >
    <FolderOpen class="h-4 w-4" /> CHỌN VIDEO ĐỂ CHIA
  </button>

  <div class="flex gap-1.5">
    <input
      type="text"
      data-testid="input-split-path"
      bind:value={draft}
      onkeydown={(e) => e.key === 'Enter' && onLoadPath(draft)}
      placeholder="D:\Videos\long_video.mp4"
      class="w-full rounded-lg border border-white/10 bg-black/40 px-2.5 py-2 font-mono text-xs text-gray-100 outline-none focus:border-purple-500"
    />
    <button
      type="button"
      data-testid="btn-probe-split"
      onclick={() => onLoadPath(draft)}
      disabled={loading}
      class="shrink-0 rounded-lg border border-white/10 bg-white/10 px-3 py-2 text-xs font-semibold text-gray-200 hover:bg-white/15"
    >
      {loading ? '...' : 'Nạp'}
    </button>
  </div>

  {#if error}
    <div class="rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-[11px] text-rose-300">
      {error}
    </div>
  {/if}

  {#if duration > 0}
    <div class="grid grid-cols-2 gap-1.5 rounded-lg border border-white/10 bg-black/30 p-1">
      <button
        type="button"
        data-testid="split-mode-markers"
        onclick={() => onModeChange('markers')}
        class="rounded-md py-1.5 text-[11px] font-semibold {mode === 'markers' ? 'bg-purple-500/25 text-purple-200' : 'text-gray-400'}"
      >
        Điểm cắt (Markers)
      </button>
      <button
        type="button"
        data-testid="split-mode-uniform"
        onclick={() => onModeChange('uniform')}
        class="rounded-md py-1.5 text-[11px] font-semibold {mode === 'uniform' ? 'bg-purple-500/25 text-purple-200' : 'text-gray-400'}"
      >
        Chia đều (Uniform)
      </button>
    </div>

    {#if mode === 'markers'}
      <div class="flex gap-1.5">
        <button
          type="button"
          data-testid="btn-add-split-marker"
          onclick={onAddMarker}
          class="flex flex-1 items-center justify-center gap-1.5 rounded-lg border border-purple-500/35 bg-purple-500/15 px-2 py-2 text-xs font-semibold text-purple-100 hover:bg-purple-500/25"
        >
          <Scissors class="h-3.5 w-3.5" /> Tách tại Playhead (Ctrl+B)
        </button>
        <button
          type="button"
          data-testid="btn-clear-split-markers"
          onclick={onClearMarkers}
          title="Xoá hết điểm cắt"
          class="flex items-center justify-center gap-1 rounded-lg border border-rose-500/30 bg-rose-500/10 px-2.5 py-2 text-xs font-semibold text-rose-300 hover:bg-rose-500/20"
        >
          <Trash2 class="h-3.5 w-3.5" /> Xoá
        </button>
      </div>
    {:else}
      <div class="flex flex-col gap-2.5 rounded-xl border border-white/10 bg-black/25 p-2.5 text-xs">
        <div class="grid grid-cols-2 gap-1.5">
          <button
            type="button"
            onclick={() => onUniformTypeChange('parts')}
            class="rounded py-1 text-[11px] {uniformType === 'parts' ? 'bg-cyan-500/20 text-cyan-200 border border-cyan-500/30' : 'bg-white/5 text-gray-400'}"
          >
            Số phần bằng nhau
          </button>
          <button
            type="button"
            onclick={() => onUniformTypeChange('duration')}
            class="rounded py-1 text-[11px] {uniformType === 'duration' ? 'bg-cyan-500/20 text-cyan-200 border border-cyan-500/30' : 'bg-white/5 text-gray-400'}"
          >
            Theo số giây / đoạn
          </button>
        </div>

        {#if uniformType === 'parts'}
          <div class="flex items-center justify-between">
            <span class="text-[11px] text-gray-300">Số phần:</span>
            <div class="flex items-center gap-1.5">
              <button
                type="button"
                data-testid="btn-split-parts-dec"
                onclick={() => onUniformPartsChange(clamp(uniformParts - 1, 2, 50))}
                class="rounded border border-white/15 bg-white/5 p-1 text-gray-200 hover:bg-white/15"
              >
                <Minus class="h-3.5 w-3.5" />
              </button>
              <span data-testid="split-parts-value" class="min-w-[52px] text-center font-mono text-xs font-bold text-cyan-300">
                {uniformParts} phần
              </span>
              <button
                type="button"
                data-testid="btn-split-parts-inc"
                onclick={() => onUniformPartsChange(clamp(uniformParts + 1, 2, 50))}
                class="rounded border border-white/15 bg-white/5 p-1 text-gray-200 hover:bg-white/15"
              >
                <Plus class="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
          <div class="grid grid-cols-5 gap-1">
            {#each PARTS_PRESETS as p (p)}
              <button
                type="button"
                data-testid={`preset-split-parts-${p}`}
                onclick={() => onUniformPartsChange(p)}
                class="rounded border py-1 font-mono text-[10px] font-semibold {uniformParts === p
                  ? 'border-cyan-400 bg-cyan-500/25 text-cyan-100'
                  : 'border-white/10 bg-white/5 text-gray-300 hover:bg-white/10'}"
              >
                {p}
              </button>
            {/each}
          </div>
        {:else}
          <div class="flex items-center justify-between">
            <span class="text-[11px] text-gray-300">Thời lượng / đoạn:</span>
            <div class="flex items-center gap-1.5">
              <button
                type="button"
                data-testid="btn-split-dur-dec"
                onclick={() => onUniformDurationChange(clamp(uniformDuration - durStep, 1, maxUniformDur))}
                class="rounded border border-white/15 bg-white/5 p-1 text-gray-200 hover:bg-white/15"
              >
                <Minus class="h-3.5 w-3.5" />
              </button>
              <span data-testid="split-dur-value" class="min-w-[52px] text-center font-mono text-xs font-bold text-cyan-300">
                {uniformDuration}s
              </span>
              <button
                type="button"
                data-testid="btn-split-dur-inc"
                onclick={() => onUniformDurationChange(clamp(uniformDuration + durStep, 1, maxUniformDur))}
                class="rounded border border-white/15 bg-white/5 p-1 text-gray-200 hover:bg-white/15"
              >
                <Plus class="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
          <div class="grid grid-cols-5 gap-1">
            {#each DUR_PRESETS as p (p.val)}
              <button
                type="button"
                data-testid={`preset-split-dur-${p.val}`}
                onclick={() => onUniformDurationChange(clamp(p.val, 1, maxUniformDur))}
                class="rounded border py-1 font-mono text-[10px] font-semibold {uniformDuration === p.val
                  ? 'border-cyan-400 bg-cyan-500/25 text-cyan-100'
                  : 'border-white/10 bg-white/5 text-gray-300 hover:bg-white/10'}"
              >
                {p.label}
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    <div class="flex max-h-48 flex-col gap-1 overflow-y-auto pr-1 studio-scroll" data-testid="split-segments-list">
      {#each segments as seg (seg.index)}
        <div class="flex items-center justify-between rounded-md border border-white/10 bg-white/5 px-2.5 py-1.5 font-mono text-[10px]">
          <span class="font-bold text-purple-300">Đ{seg.index}</span>
          <span class="text-gray-200">{formatTime(seg.start)} &rarr; {formatTime(seg.end)}</span>
          <span class="text-gray-400">{seg.duration.toFixed(1)}s</span>
        </div>
      {/each}
    </div>
  {/if}
</div>

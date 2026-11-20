<script lang="ts">
  import { ArrowLeftToLine, ArrowRightToLine, FolderOpen, Plus, Scissors, Trash2 } from 'lucide-svelte';
  import type { ProbeResult, TrimSegment } from '$lib/types/studioTypes';
  import { formatTime } from '$lib/utils/timelineMath';

  interface Props {
    videoPath: string;
    videoMeta: ProbeResult | null;
    probeLoading: boolean;
    probeError: string;
    trimEnabled: boolean;
    trimSegments: TrimSegment[];
    selectedTrimId: string | null;
    keptDuration: number;
    trimValid: boolean;
    onBrowse: () => void;
    onLoadPath: (path: string) => void;
    onToggleTrim: (enabled: boolean) => void;
    onAddTrim: () => void;
    onCutAtPlayhead: () => void;
    onSelectTrim: (id: string) => void;
    onSetTrimBoundary: (id: string, edge: 'start' | 'end') => void;
    onRemoveTrim: (id: string) => void;
  }

  let {
    videoPath, videoMeta, probeLoading, probeError, trimEnabled, trimSegments,
    selectedTrimId, keptDuration, trimValid,
    onBrowse, onLoadPath, onToggleTrim, onAddTrim, onCutAtPlayhead,
    onSelectTrim, onSetTrimBoundary, onRemoveTrim
  }: Props = $props();

  let pathDraft = $state('');
  $effect(() => { pathDraft = videoPath; });

  function submitPath() {
    if (pathDraft.trim() !== videoPath || !videoMeta) onLoadPath(pathDraft.trim());
  }
</script>

<div class="flex flex-col gap-3">
  <div class="rounded-lg border border-purple-500/20 bg-purple-500/10 px-3 py-2 text-[11px] text-gray-300">
    Chọn video cục bộ để chỉnh tốc độ, bộ lọc màu, chữ/ảnh phủ &amp; cắt đoạn dư.
  </div>

  <button
    type="button"
    data-testid="btn-browse-edit"
    onclick={onBrowse}
    class="btn-neon flex w-full items-center justify-center gap-2 rounded-xl px-4 py-2.5 text-xs font-bold text-white"
  >
    <FolderOpen class="h-4 w-4" /> CHỌN VIDEO TỪ MÁY
  </button>

  <div class="flex flex-col gap-1.5">
    <label for="edit-path-input" class="text-[11px] font-semibold text-gray-400">Đường dẫn tuyệt đối</label>
    <div class="flex gap-1.5">
      <input
        id="edit-path-input"
        data-testid="input-edit-path"
        type="text"
        bind:value={pathDraft}
        onkeydown={(e) => e.key === 'Enter' && submitPath()}
        placeholder="D:\Videos\clip.mp4"
        class="w-full rounded-lg border border-white/10 bg-black/40 px-2.5 py-2 font-mono text-xs text-gray-100 outline-none focus:border-purple-500"
      />
      <button
        type="button"
        data-testid="btn-probe-edit"
        onclick={submitPath}
        disabled={probeLoading}
        class="shrink-0 rounded-lg border border-white/10 bg-white/10 px-3 py-2 text-xs font-semibold text-gray-200 hover:bg-white/15 disabled:opacity-50"
      >
        {probeLoading ? '...' : 'Nạp'}
      </button>
    </div>
  </div>

  {#if probeError}
    <div class="rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-[11px] text-rose-300">
      {probeError}
    </div>
  {/if}

  {#if videoMeta}
    <div
      data-testid="edit-probe-meta"
      class="grid grid-cols-2 gap-1.5 rounded-xl border border-white/10 bg-white/5 p-2.5 font-mono text-[10px] text-gray-300"
    >
      <div>Hiển thị: <span class="text-cyan-300">{videoMeta.width}x{videoMeta.height}</span></div>
      <div>Gốc: <span class="text-gray-200">{formatTime(videoMeta.duration)}</span></div>
      <div>Codec: <span class="text-purple-300">{videoMeta.video_codec || '?'}</span></div>
      <div>Âm thanh: <span class="text-emerald-300">{videoMeta.has_audio ? videoMeta.audio_codec || 'có' : 'không'}</span></div>
      {#if videoMeta.bitrate_kbps && videoMeta.bitrate_kbps > 0}
        <div class="col-span-2" data-testid="probe-bitrate-badge">
          Bitrate: <span class="text-amber-300">{videoMeta.bitrate_kbps} kbps</span>
        </div>
      {/if}
    </div>
  {/if}

  <div class="my-1 h-px bg-white/10"></div>

  <div class="flex items-center justify-between">
    <div>
      <div class="text-xs font-semibold text-gray-200">Cắt bỏ đoạn video (Exclude)</div>
      <div class="text-[10px] text-gray-500">Kéo trực tiếp trên timeline hoặc nhấn Ctrl+B</div>
    </div>
    <input
      type="checkbox"
      data-testid="toggle-trim"
      checked={trimEnabled}
      onchange={(e) => onToggleTrim((e.currentTarget as HTMLInputElement).checked)}
      class="h-4 w-4 accent-purple-500 cursor-pointer"
    />
  </div>

  {#if trimEnabled}
    <div class="flex flex-col gap-2.5 rounded-xl border border-white/10 bg-black/30 p-3">
      <div class="flex items-center justify-between gap-1 border-b border-white/10 pb-2 text-[11px]">
        <span class="text-gray-400">
          Giữ: <strong data-testid="trim-kept-duration" class="font-mono text-purple-300">{keptDuration.toFixed(2)}s</strong>
        </span>
        <div class="flex items-center gap-1">
          <button
            type="button"
            data-testid="btn-cut-trim-playhead"
            onclick={onCutAtPlayhead}
            title="Cắt tại Playhead (Ctrl+B)"
            class="flex items-center gap-1 rounded-md border border-rose-500/35 bg-rose-500/15 px-2 py-1 text-[11px] font-semibold text-rose-200 hover:bg-rose-500/25"
          >
            <Scissors class="h-3 w-3" /> Cắt
          </button>
          <button
            type="button"
            data-testid="btn-add-trim"
            onclick={onAddTrim}
            class="flex items-center gap-1 rounded-md border border-white/15 bg-white/10 px-2 py-1 text-[11px] font-semibold text-gray-100 hover:bg-white/15"
          >
            <Plus class="h-3 w-3" /> Thêm
          </button>
        </div>
      </div>

      {#if !trimValid && videoMeta}
        <p class="text-[11px] text-rose-400">Toàn bộ video đã bị cắt bỏ. Vui lòng giữ lại ít nhất 0.05s.</p>
      {/if}

      <div class="flex max-h-56 flex-col gap-1.5 overflow-y-auto pr-1 studio-scroll" data-testid="trim-chips-list">
        {#each trimSegments as seg, idx (seg.id)}
          {@const selected = seg.id === selectedTrimId}
          <div
            role="button"
            tabindex="0"
            data-testid={`trim-chip-${seg.id}`}
            onclick={() => onSelectTrim(seg.id)}
            onkeydown={(e) => e.key === 'Enter' && onSelectTrim(seg.id)}
            class="flex items-center justify-between gap-1.5 rounded-lg border px-2 py-1.5 transition-colors cursor-pointer {selected
              ? 'border-rose-400 bg-rose-500/20 text-white'
              : 'border-white/10 bg-white/5 text-gray-200 hover:bg-white/10'}"
          >
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-1.5 font-mono text-[11px]">
                <span class="font-bold text-rose-300">#{idx + 1}</span>
                <span class="truncate">{formatTime(seg.start)} &rarr; {formatTime(seg.end)}</span>
              </div>
              <div class="font-mono text-[9px] text-gray-400">
                Loại bỏ {(seg.end - seg.start).toFixed(2)}s
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-0.5">
              <button
                type="button"
                onclick={(e) => { e.stopPropagation(); onSelectTrim(seg.id); onSetTrimBoundary(seg.id, 'start'); }}
                title="[ Đặt đầu đoạn tại Playhead"
                class="rounded border border-white/10 bg-black/30 p-1 text-cyan-300 hover:bg-white/15"
              >
                <ArrowRightToLine class="h-3 w-3" />
              </button>
              <button
                type="button"
                onclick={(e) => { e.stopPropagation(); onSelectTrim(seg.id); onSetTrimBoundary(seg.id, 'end'); }}
                title="] Đặt cuối đoạn tại Playhead"
                class="rounded border border-white/10 bg-black/30 p-1 text-cyan-300 hover:bg-white/15"
              >
                <ArrowLeftToLine class="h-3 w-3" />
              </button>
              <button
                type="button"
                onclick={(e) => { e.stopPropagation(); onRemoveTrim(seg.id); }}
                class="rounded p-1 text-rose-400 hover:bg-rose-500/20"
                title="Xóa đoạn"
              >
                <Trash2 class="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>

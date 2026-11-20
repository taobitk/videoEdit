<script lang="ts">
  import { ArrowDown, ArrowUp, FolderOpen, Plus, Trash2 } from 'lucide-svelte';
  import type { MergeFileItem } from '$lib/types/studioTypes';
  import { formatTime } from '$lib/utils/timelineMath';

  interface Props {
    files: MergeFileItem[];
    error: string;
    onBrowseAdd: () => void;
    onAddPath: (p: string) => void;
    onMoveFile: (id: string, dir: 'up' | 'down') => void;
    onRemoveFile: (id: string) => void;
    onRequestClearAll: () => void;
  }

  let {
    files,
    error,
    onBrowseAdd,
    onAddPath,
    onMoveFile,
    onRemoveFile,
    onRequestClearAll
  }: Props = $props();

  let pathDraft = $state('');

  function submitAdd() {
    if (!pathDraft.trim()) return;
    onAddPath(pathDraft.trim());
    pathDraft = '';
  }
</script>

<div class="flex flex-col gap-3">
  <div class="rounded-lg border border-cyan-500/25 bg-cyan-500/10 px-3 py-2 text-[11px] text-gray-300">
    Thêm các video cần ghép và điều chỉnh thứ tự. Hỗ trợ ghép khác độ phân giải.
  </div>

  <button
    type="button"
    data-testid="btn-browse-merge"
    onclick={onBrowseAdd}
    class="btn-neon flex w-full items-center justify-center gap-2 rounded-xl px-4 py-2.5 text-xs font-bold text-white"
  >
    <FolderOpen class="h-4 w-4" />
    THÊM VIDEO CẦN GHÉP
  </button>

  <div class="flex gap-1.5">
    <input
      type="text"
      data-testid="input-merge-path"
      bind:value={pathDraft}
      onkeydown={(e) => e.key === 'Enter' && submitAdd()}
      placeholder="Nhập đường dẫn... D:\vid.mp4"
      class="w-full rounded-lg border border-white/10 bg-black/40 px-2.5 py-2 font-mono text-xs text-gray-100 outline-none focus:border-cyan-500"
    />
    <button
      type="button"
      data-testid="btn-add-merge-path"
      onclick={submitAdd}
      class="flex shrink-0 items-center gap-1 rounded-lg border border-white/15 bg-white/10 px-2.5 py-2 text-xs font-semibold text-gray-100 hover:bg-white/15"
    >
      <Plus class="h-3.5 w-3.5" /> Thêm
    </button>
  </div>

  {#if error}
    <div class="rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-[11px] text-rose-300">
      {error}
    </div>
  {/if}

  <div class="flex max-h-64 flex-col gap-1.5 overflow-y-auto pr-1 studio-scroll" data-testid="merge-files-list">
    {#each files as f, idx (f.id)}
      <div class="flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-2.5 py-2">
        <span class="font-mono text-[11px] font-bold text-purple-300">
          {String(idx + 1).padStart(2, '0')}
        </span>
        <div class="min-w-0 flex-1">
          <div class="truncate text-xs font-medium text-gray-200" title={f.path}>{f.displayName}</div>
          {#if f.probe}
            <div class="font-mono text-[10px] text-cyan-300/90">
              {f.probe.width}x{f.probe.height} &bull; {formatTime(f.probe.duration)}
            </div>
          {:else if f.probeError}
            <div class="text-[10px] text-rose-400">{f.probeError}</div>
          {:else}
            <div class="text-[10px] text-gray-500">Đang phân tích...</div>
          {/if}
        </div>
        <div class="flex shrink-0 items-center gap-0.5">
          <button
            type="button"
            onclick={() => onMoveFile(f.id, 'up')}
            disabled={idx === 0}
            class="rounded p-1 text-gray-400 hover:bg-white/10 hover:text-white disabled:opacity-30"
            title="Lên"
          >
            <ArrowUp class="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            onclick={() => onMoveFile(f.id, 'down')}
            disabled={idx === files.length - 1}
            class="rounded p-1 text-gray-400 hover:bg-white/10 hover:text-white disabled:opacity-30"
            title="Xuống"
          >
            <ArrowDown class="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            onclick={() => onRemoveFile(f.id)}
            class="rounded p-1 text-rose-400 hover:bg-rose-500/15"
            title="Xóa"
          >
            <Trash2 class="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
    {/each}
  </div>

  <div class="flex items-center justify-between text-[11px] text-gray-400">
    <span data-testid="merge-count-label">{files.length} video đã thêm</span>
    {#if files.length > 0}
      <button
        type="button"
        data-testid="btn-clear-merge"
        onclick={onRequestClearAll}
        class="text-rose-400 hover:text-rose-300"
      >
        Xóa tất cả
      </button>
    {/if}
  </div>
</div>

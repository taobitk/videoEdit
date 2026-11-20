<script lang="ts">
  import { CheckCircle2, Download, FolderOpen, RotateCcw, Square, XCircle } from 'lucide-svelte';
  import type { EngineStatus } from '$lib/types/studioTypes';

  interface Props {
    status: EngineStatus;
    onCancel: () => void;
    onReset: () => void;
    onReveal: (filename: string) => void;
  }

  let { status, onCancel, onReset, onReveal }: Props = $props();

  let consoleEl = $state<HTMLPreElement | null>(null);
  let selectedPreviewUrl = $state('');

  const multiFiles = $derived(status.output_files.length > 1 ? status.output_files : []);
  const primaryUrl = $derived(
    selectedPreviewUrl || status.output_url || status.output_files[0]?.url || ''
  );
  const primaryFilename = $derived(
    status.output_filename ||
      status.output_files[0]?.filename ||
      primaryUrl.split('/').pop() ||
      ''
  );

  $effect(() => {
    if (status.status !== 'completed') {
      selectedPreviewUrl = '';
    }
  });

  $effect(() => {
    if (consoleEl && status.logs.length > 0) {
      consoleEl.scrollTop = consoleEl.scrollHeight;
    }
  });
</script>

{#if status.status === 'processing'}
  <div class="flex h-full flex-col gap-3" data-testid="render-processing-view">
    <div class="border-b border-white/10 pb-2">
      <div class="flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-rose-400">
        <span class="h-2.5 w-2.5 animate-pulse rounded-full bg-rose-500"></span>
        RENDERING STREAM
      </div>
      <p class="mt-1 text-[11px] text-gray-400">{status.current_step || 'Đang xử lý...'}</p>
    </div>

    {#if status.warnings.length > 0}
      <div
        data-testid="render-warnings-banner"
        class="flex flex-col gap-1 rounded-xl border border-amber-500/30 bg-amber-500/10 p-3 text-[11px] text-amber-300"
      >
        {#each status.warnings as w, i (i)}
          <div>{w}</div>
        {/each}
      </div>
    {/if}

    <div class="rounded-xl border border-white/10 bg-white/5 p-3">
      <div class="mb-1.5 flex items-center justify-between text-xs">
        <span class="text-gray-400">Tiến độ</span>
        <span class="font-mono font-bold text-pink-300">{status.progress.toFixed(1)}%</span>
      </div>
      <div class="h-2.5 overflow-hidden rounded-full bg-white/10">
        <div
          class="h-full rounded-full bg-gradient-to-r from-purple-500 via-pink-500 to-cyan-400 transition-all"
          style="width: {Math.min(100, Math.max(0, status.progress))}%;"
        ></div>
      </div>
    </div>

    <div class="grid grid-cols-2 gap-2 font-mono">
      <div class="rounded-lg border border-white/10 bg-black/30 p-2.5">
        <div class="text-[9px] uppercase text-gray-500">Tốc độ</div>
        <div class="text-xs font-bold text-white">{status.speed || '0.0x'}</div>
      </div>
      <div class="rounded-lg border border-white/10 bg-black/30 p-2.5">
        <div class="text-[9px] uppercase text-gray-500">Khung hình</div>
        <div class="text-xs font-bold text-white">{Math.round(status.fps)} fps</div>
      </div>
      <div class="rounded-lg border border-white/10 bg-black/30 p-2.5">
        <div class="text-[9px] uppercase text-gray-500">Đã xử lý</div>
        <div class="text-xs font-bold text-white">{status.time_processed || '00:00:00'}</div>
      </div>
      <div class="rounded-lg border border-white/10 bg-black/30 p-2.5">
        <div class="text-[9px] uppercase text-gray-500">Còn lại</div>
        <div class="text-xs font-bold text-pink-300">{status.eta || 'Calculating...'}</div>
      </div>
    </div>

    <div class="flex items-center justify-between rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-[11px]">
      <span class="text-gray-400">Encoder:</span>
      <span class="font-mono font-bold text-cyan-300">
        {status.active_encoder}{status.encoder_detail ? ` • ${status.encoder_detail}` : ''}
      </span>
    </div>

    <button
      type="button"
      data-testid="btn-cancel-render"
      onclick={onCancel}
      class="flex items-center justify-center gap-2 rounded-xl border border-rose-500/40 bg-rose-500/15 py-2.5 text-xs font-bold text-rose-300 hover:bg-rose-500/25"
    >
      <Square class="h-3.5 w-3.5" /> HỦY TÁC VỤ
    </button>

    <div class="flex min-h-[140px] flex-1 flex-col overflow-hidden rounded-xl border border-white/10 bg-black/50">
      <div class="border-b border-white/10 bg-white/5 px-3 py-1.5 font-mono text-[10px] text-gray-400">
        FFmpeg Console Stream
      </div>
      <pre
        bind:this={consoleEl}
        class="studio-scroll flex-1 overflow-auto p-2.5 font-mono text-[10px] leading-relaxed text-emerald-200/90 whitespace-pre-wrap break-all"
      >{status.logs.join('\n') || 'Đang chờ nhật ký...'}</pre>
    </div>
  </div>
{:else if status.status === 'completed'}
  <div class="flex flex-col gap-3.5" data-testid="render-completed-view">
    <div class="flex flex-col items-center py-1 text-center">
      <CheckCircle2 class="mb-2 h-10 w-10 text-emerald-400" />
      <h3 class="text-xs font-bold uppercase tracking-wider text-emerald-300">
        {multiFiles.length > 0 ? `Hoàn thành (${multiFiles.length} đoạn)` : 'Xuất Video Thành Công!'}
      </h3>
      <p class="mt-1 text-[11px] text-gray-400">{primaryFilename}</p>
    </div>

    {#if status.warnings.length > 0}
      <div
        data-testid="render-warnings-banner"
        class="flex flex-col gap-1 rounded-xl border border-amber-500/30 bg-amber-500/10 p-3 text-[11px] text-amber-300"
      >
        {#each status.warnings as w, i (i)}
          <div>{w}</div>
        {/each}
      </div>
    {/if}

    {#if primaryUrl}
      <div class="overflow-hidden rounded-xl border border-white/10 bg-black">
        <video src={primaryUrl} controls preload="metadata" class="max-h-44 w-full object-contain">
          <track kind="captions" />
        </video>
      </div>
    {/if}

    {#if multiFiles.length > 0}
      <div class="flex max-h-48 flex-col gap-1.5 overflow-y-auto pr-1 studio-scroll" data-testid="output-files-list">
        {#each multiFiles as file, idx (file.filename)}
          <div class="flex items-center justify-between gap-2 rounded-lg border border-white/10 bg-white/5 px-2.5 py-2 text-xs">
            <button
              type="button"
              onclick={() => (selectedPreviewUrl = file.url)}
              class="min-w-0 flex-1 truncate text-left font-mono text-[11px] text-purple-200 hover:underline"
            >
              {String(idx + 1).padStart(2, '0')}. {file.filename}
            </button>
            <a
              href={`${file.url}?download=1`}
              download={file.filename}
              class="flex shrink-0 items-center gap-1 rounded bg-cyan-500/20 px-2 py-1 text-[10px] font-semibold text-cyan-200 hover:bg-cyan-500/30"
            >
              <Download class="h-3 w-3" /> Tải
            </a>
          </div>
        {/each}
      </div>
    {:else if primaryUrl}
      <a
        data-testid="btn-download-video"
        href={`${primaryUrl}?download=1`}
        download={primaryFilename}
        class="btn-neon flex items-center justify-center gap-2 rounded-xl py-3 text-xs font-bold text-white no-underline"
      >
        <Download class="h-4 w-4" /> TẢI VIDEO XUỐNG
      </a>
    {/if}

    <div class="grid grid-cols-2 gap-2">
      <button
        type="button"
        data-testid="btn-show-explorer"
        onclick={() => onReveal(primaryFilename)}
        class="flex items-center justify-center gap-1.5 rounded-xl border border-white/15 bg-white/10 py-2.5 text-xs font-semibold text-gray-200 hover:bg-white/15"
      >
        <FolderOpen class="h-3.5 w-3.5" /> Explorer
      </button>
      <button
        type="button"
        data-testid="btn-new-render"
        onclick={onReset}
        class="flex items-center justify-center gap-1.5 rounded-xl border border-white/15 bg-white/10 py-2.5 text-xs font-semibold text-gray-200 hover:bg-white/15"
      >
        <RotateCcw class="h-3.5 w-3.5" /> Render mới
      </button>
    </div>
  </div>
{:else if status.status === 'failed'}
  <div class="flex flex-col items-center gap-3 py-6 text-center" data-testid="render-failed-view">
    <XCircle class="h-10 w-10 text-rose-500" />
    <h3 class="text-xs font-bold uppercase tracking-wider text-rose-400">Xử Lý Thất Bại</h3>
    <p class="w-full rounded-xl border border-rose-500/25 bg-black/40 p-3 font-mono text-[11px] text-rose-200 break-words">
      {status.error_message || 'Đã xảy ra lỗi khi render video.'}
    </p>
    <button
      type="button"
      data-testid="btn-error-reset"
      onclick={onReset}
      class="btn-neon mt-2 w-full rounded-xl py-2.5 text-xs font-bold text-white"
    >
      QUAY LẠI STUDIO
    </button>
  </div>
{/if}

<script lang="ts">
  import {
    ArrowLeftToLine, ArrowRightToLine, ChevronLeft, ChevronRight, ClipboardPaste,
    Copy, Image as ImageIcon, Maximize2, Pause, Play, Redo2, Scissors,
    Trash2, Type, Undo2, ZoomIn, ZoomOut
  } from 'lucide-svelte';
  import type { WorkspaceTab } from '$lib/types/studioTypes';
  import { formatTime } from '$lib/utils/timelineMath';

  interface Props {
    mode: WorkspaceTab;
    duration: number;
    playhead: number;
    isPlaying: boolean;
    speed: number;
    outputDuration: number;
    timelineZoom: number;
    splitSegmentCount: number;
    hasLayerSelected: boolean;
    hasAnySelection: boolean;
    canPaste: boolean;
    canUndo: boolean;
    canRedo: boolean;
    onTogglePlay: () => void;
    onStepSeek: (deltaSec: number) => void;
    onBladeCut: () => void;
    onSetIn: () => void;
    onSetOut: () => void;
    onAddText: () => void;
    onPickImage: () => void;
    onCopy: () => void;
    onPaste: () => void;
    onUndo: () => void;
    onRedo: () => void;
    onDelete: () => void;
    onZoomChange: (zoom: number) => void;
  }

  let {
    mode, duration, playhead, isPlaying, speed, outputDuration, timelineZoom,
    splitSegmentCount, hasLayerSelected, hasAnySelection, canPaste, canUndo, canRedo,
    onTogglePlay, onStepSeek, onBladeCut, onSetIn, onSetOut, onAddText, onPickImage,
    onCopy, onPaste, onUndo, onRedo, onDelete, onZoomChange
  }: Props = $props();

  const btnCls = 'flex h-7 items-center gap-1 rounded-md border border-white/10 bg-white/5 px-1.5 text-[11px] text-gray-200 hover:bg-white/15 disabled:opacity-30 disabled:pointer-events-none';
  const kbdCls = 'hidden sm:inline rounded bg-black/40 px-1 font-mono text-[9px] text-gray-400';
</script>

<div class="mb-2 flex flex-wrap items-center justify-between gap-1.5">
  <div class="flex flex-wrap items-center gap-1">
    <button
      type="button"
      data-testid="btn-play-toggle"
      onclick={onTogglePlay}
      title="Phát / Tạm dừng (Space)"
      class="flex h-7 items-center gap-1 rounded-md border border-purple-500/40 bg-purple-500/20 px-2 text-purple-200 hover:bg-purple-500/35"
    >
      {#if isPlaying}<Pause class="h-3.5 w-3.5" />{:else}<Play class="h-3.5 w-3.5" />{/if}
      <span class={kbdCls}>Space</span>
    </button>

    <button type="button" data-testid="btn-step-back" onclick={() => onStepSeek(-1)} title="Lùi 1s (←)" class={btnCls}>
      <ChevronLeft class="h-3.5 w-3.5" />
    </button>
    <span class="px-1 font-mono text-[11px] text-gray-200">
      {formatTime(playhead)} <span class="text-gray-500">/ {formatTime(duration)}</span>
    </span>
    <button type="button" data-testid="btn-step-fwd" onclick={() => onStepSeek(1)} title="Tới 1s (→)" class={btnCls}>
      <ChevronRight class="h-3.5 w-3.5" />
    </button>

    <span class="mx-0.5 h-4 w-px bg-white/10"></span>

    <button
      type="button"
      data-testid="btn-nle-cut"
      onclick={onBladeCut}
      disabled={duration <= 0}
      title="Cắt tại Playhead (Ctrl+B / S)"
      class="flex h-7 items-center gap-1 rounded-md border border-rose-500/35 bg-rose-500/15 px-2 text-[11px] font-semibold text-rose-200 hover:bg-rose-500/25 disabled:opacity-30"
    >
      <Scissors class="h-3.5 w-3.5" />
      <span class={kbdCls}>Ctrl+B</span>
    </button>

    {#if mode === 'edit'}
      <button type="button" data-testid="btn-nle-in" onclick={onSetIn} disabled={!hasAnySelection} title="Đặt điểm đầu [ tại Playhead (I)" class={btnCls}>
        <ArrowRightToLine class="h-3.5 w-3.5 text-cyan-300" /><span class={kbdCls}>[</span>
      </button>
      <button type="button" data-testid="btn-nle-out" onclick={onSetOut} disabled={!hasAnySelection} title="Đặt điểm cuối ] tại Playhead (O)" class={btnCls}>
        <ArrowLeftToLine class="h-3.5 w-3.5 text-cyan-300" /><span class={kbdCls}>]</span>
      </button>

      <span class="mx-0.5 h-4 w-px bg-white/10"></span>

      <button type="button" data-testid="btn-nle-add-text" onclick={onAddText} title="Thêm Chữ (T)" class={btnCls}>
        <Type class="h-3.5 w-3.5 text-purple-300" /><span class={kbdCls}>T</span>
      </button>
      <button type="button" data-testid="btn-nle-add-image" onclick={onPickImage} title="Thêm Ảnh (P)" class={btnCls}>
        <ImageIcon class="h-3.5 w-3.5 text-cyan-300" /><span class={kbdCls}>P</span>
      </button>
      <button type="button" data-testid="btn-nle-copy" onclick={onCopy} disabled={!hasLayerSelected} title="Sao chép lớp (Ctrl+C)" class={btnCls}>
        <Copy class="h-3.5 w-3.5" />
      </button>
      <button type="button" data-testid="btn-nle-paste" onclick={onPaste} disabled={!canPaste} title="Dán lớp (Ctrl+V)" class={btnCls}>
        <ClipboardPaste class="h-3.5 w-3.5" />
      </button>
      <button type="button" data-testid="btn-nle-undo" onclick={onUndo} disabled={!canUndo} title="Hoàn tác (Ctrl+Z)" class={btnCls}>
        <Undo2 class="h-3.5 w-3.5" />
      </button>
      <button type="button" data-testid="btn-nle-redo" onclick={onRedo} disabled={!canRedo} title="Làm lại (Ctrl+Y)" class={btnCls}>
        <Redo2 class="h-3.5 w-3.5" />
      </button>
      <button type="button" data-testid="btn-nle-delete" onclick={onDelete} disabled={!hasAnySelection} title="Xóa mục chọn (Del)" class="flex h-7 items-center rounded-md border border-rose-500/25 bg-rose-500/10 px-1.5 text-rose-300 hover:bg-rose-500/20 disabled:opacity-30 disabled:pointer-events-none">
        <Trash2 class="h-3.5 w-3.5" />
      </button>
    {/if}
  </div>

  <div class="flex items-center gap-1.5">
    <div class="flex items-center gap-0.5 rounded-md border border-white/10 bg-black/35 p-0.5">
      <button type="button" data-testid="btn-zoom-out" onclick={() => onZoomChange(timelineZoom / 1.5)} disabled={timelineZoom <= 1} title="Thu nhỏ (Ctrl+-)" class="rounded p-1 text-gray-300 hover:bg-white/10 disabled:opacity-30">
        <ZoomOut class="h-3 w-3" />
      </button>
      <span data-testid="timeline-zoom-label" class="min-w-[34px] text-center font-mono text-[10px] text-cyan-300">
        {timelineZoom.toFixed(1).replace(/\.0$/, '')}x
      </span>
      <button type="button" data-testid="btn-zoom-in" onclick={() => onZoomChange(timelineZoom * 1.5)} disabled={timelineZoom >= 50} title="Phóng to (Ctrl+=)" class="rounded p-1 text-gray-300 hover:bg-white/10 disabled:opacity-30">
        <ZoomIn class="h-3 w-3" />
      </button>
      <button type="button" data-testid="btn-zoom-fit" onclick={() => onZoomChange(1)} disabled={timelineZoom <= 1} title="Vừa khung 1x (Ctrl+0)" class="rounded px-1 py-0.5 font-mono text-[10px] text-gray-300 hover:bg-white/10 disabled:opacity-30">
        <Maximize2 class="h-3 w-3" />
      </button>
    </div>

    {#if mode === 'edit' && duration > 0}
      <div data-testid="output-duration-badge" class="flex items-center gap-1.5 rounded-md border border-cyan-500/25 bg-cyan-500/10 px-2 py-1 font-mono text-[10px] text-cyan-200">
        <span>Xuất:</span>
        <strong>{formatTime(outputDuration)}</strong>
        <span class="text-cyan-400/80">({speed.toFixed(2)}x)</span>
      </div>
    {:else if mode === 'split'}
      <span class="font-mono text-[11px] text-purple-300">{splitSegmentCount} đoạn</span>
    {/if}
  </div>
</div>

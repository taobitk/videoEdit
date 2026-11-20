<script lang="ts">
  import { Eye, EyeOff, Film, GripVertical, Image as ImageIcon, Scissors, Type } from 'lucide-svelte';
  import type { OverlayLayer, TrimSegment } from '$lib/types/studioTypes';
  import { clamp } from '$lib/utils/geometryMath';
  import { buildSplitSegments } from '$lib/utils/splitMath';
  import { timelineClipInteract } from '$lib/utils/timelineInteract';
  import { formatTime, formatTimeShort } from '$lib/utils/timelineMath';

  interface Props {
    kind: 'video-trim' | 'video-split' | 'layer';
    duration: number;
    playhead: number;
    currentOutputPct?: number;
    trackWidthPx: number;
    trimEnabled?: boolean;
    trimSegments?: TrimSegment[];
    selectedTrimId?: string | null;
    layer?: OverlayLayer;
    isLayerSelected?: boolean;
    splitMarkers?: number[];
    splitMode?: 'markers' | 'uniform';
    onLaneScrub: (e: MouseEvent, laneEl: HTMLDivElement) => void;
    onSelectTrim?: (id: string) => void;
    onUpdateTrimSegment?: (id: string, start: number, end: number, field: 'start' | 'end') => void;
    onSelectLayer?: (id: string) => void;
    onToggleLayerVisible?: (id: string, visible: boolean) => void;
    onCommitHistory?: () => void;
    onUpdateLayer?: (id: string, patch: Partial<OverlayLayer>) => void;
    onTrackDragStart?: (id: string) => void;
    onTrackDrop?: (targetId: string) => void;
    onStartMarkerDrag?: (e: MouseEvent, idx: number, laneEl: HTMLDivElement) => void;
    onRemoveSplitMarker?: (idx: number) => void;
  }

  let {
    kind, duration, playhead, currentOutputPct = 0, trackWidthPx,
    trimEnabled = false, trimSegments = [], selectedTrimId = null,
    layer, isLayerSelected = false, splitMarkers = [], splitMode = 'markers',
    onLaneScrub, onSelectTrim, onUpdateTrimSegment,
    onSelectLayer, onToggleLayerVisible, onCommitHistory, onUpdateLayer,
    onTrackDragStart, onTrackDrop, onStartMarkerDrag, onRemoveSplitMarker
  }: Props = $props();

  let laneEl = $state<HTMLDivElement | null>(null);
  const splitSegs = $derived(kind === 'video-split' ? buildSplitSegments(splitMarkers, duration) : []);

  const SEG_COLORS = [
    'bg-purple-500/30 border-purple-400/50', 'bg-cyan-500/30 border-cyan-400/50',
    'bg-pink-500/30 border-pink-400/50', 'bg-emerald-500/30 border-emerald-400/50',
    'bg-amber-500/30 border-amber-400/50'
  ];

  function handleLaneDown(e: MouseEvent) {
    if (!laneEl) return;
    if ((e.target as HTMLElement).closest('[data-clip-item],[data-split-marker]')) return;
    onLaneScrub(e, laneEl);
  }
</script>

<div class="flex min-w-full border-b border-white/5 last:border-b-0">
  <!-- Sticky Left Track Header -->
  <div
    class="sticky left-0 z-20 flex w-[136px] shrink-0 items-center justify-between gap-1.5 border-r border-white/10 bg-[#0d101d] px-2.5 text-[11px] {isLayerSelected
      ? 'bg-purple-950/50 text-purple-100'
      : 'text-gray-300'}"
  >
    {#if kind === 'video-trim'}
      <div data-testid="timeline-track-video" class="flex min-w-0 items-center gap-1.5">
        <Film class="h-3.5 w-3.5 shrink-0 text-cyan-300" />
        <span class="truncate font-semibold">Video Trim</span>
      </div>
      {#if trimEnabled && trimSegments.length > 0}
        <span class="rounded bg-rose-500/20 px-1 font-mono text-[9px] text-rose-300">
          {trimSegments.length}
        </span>
      {/if}
    {:else if kind === 'video-split'}
      <div data-testid="timeline-track-split" class="flex min-w-0 items-center gap-1.5">
        <Scissors class="h-3.5 w-3.5 shrink-0 text-purple-300" />
        <span class="truncate font-semibold">Chia đoạn</span>
      </div>
    {:else if layer}
      <div
        draggable="true"
        role="listitem"
        ondragstart={() => onTrackDragStart?.(layer.id)}
        ondragover={(e) => e.preventDefault()}
        ondrop={() => onTrackDrop?.(layer.id)}
        class="flex min-w-0 flex-1 items-center gap-1"
      >
        <GripVertical class="h-3 w-3 shrink-0 cursor-grab text-gray-500 hover:text-gray-200" />
        <button
          type="button"
          data-testid={`timeline-track-layer-${layer.id}`}
          onclick={() => onSelectLayer?.(layer.id)}
          class="flex min-w-0 flex-1 items-center gap-1.5 py-1 text-left"
        >
          {#if layer.type === 'text'}
            <Type class="h-3 w-3 shrink-0 text-purple-300" />
            <span class="truncate">{layer.text || 'Chữ'}</span>
          {:else}
            <ImageIcon class="h-3 w-3 shrink-0 text-cyan-300" />
            <span class="truncate">{layer.displayName}</span>
          {/if}
        </button>
      </div>
      <button
        type="button"
        data-testid={`track-eye-${layer.id}`}
        onclick={() => onToggleLayerVisible?.(layer.id, !layer.visible)}
        title={layer.visible ? 'Ẩn lớp' : 'Hiện lớp'}
        class="rounded p-0.5 text-gray-400 hover:text-white"
      >
        {#if layer.visible}<Eye class="h-3 w-3" />{:else}<EyeOff class="h-3 w-3 text-gray-600" />{/if}
      </button>
    {/if}
  </div>

  <!-- Horizontal Clip Lane -->
  <div
    bind:this={laneEl}
    role="presentation"
    onmousedown={handleLaneDown}
    class="relative shrink-0 cursor-pointer select-none bg-black/35"
    style="width: {trackWidthPx}px; height: {kind === 'layer' ? '34px' : '40px'};"
  >
    {#if kind === 'video-trim'}
      <div class="pointer-events-none absolute inset-x-0 top-1.5 bottom-1.5 rounded bg-emerald-500/10 border border-emerald-500/15"></div>
      {#if trimEnabled && duration > 0}
        {#each trimSegments as seg (seg.id)}
          {@const left = clamp((seg.start / duration) * 100, 0, 100)}
          {@const width = clamp(((seg.end - seg.start) / duration) * 100, 0.8, 100 - left)}
          <div
            data-clip-item
            data-testid={`trim-clip-${seg.id}`}
            role="button"
            tabindex="0"
            onmousedown={() => onSelectTrim?.(seg.id)}
            onclick={() => onSelectTrim?.(seg.id)}
            onkeydown={(e) => e.key === 'Enter' && onSelectTrim?.(seg.id)}
            use:timelineClipInteract={{
              disabled: duration <= 0,
              start: seg.start,
              end: seg.end,
              minBound: 0,
              maxBound: duration,
              minSpan: Math.min(0.1, duration),
              playheadValue: playhead,
              trackWidthPx,
              onChange: (s, e, edge) => onUpdateTrimSegment?.(seg.id, s, e, edge === 'left' ? 'start' : 'end')
            }}
            class="absolute top-1.5 bottom-1.5 flex items-center justify-center overflow-hidden rounded border text-[10px] font-mono cursor-grab active:cursor-grabbing {seg.id === selectedTrimId
              ? 'border-white bg-rose-500/45 text-white ring-1 ring-rose-300'
              : 'border-rose-500/60 bg-rose-500/30 text-rose-100'}"
            style="left: {left}%; width: {width}%;"
            title="Loại bỏ: {formatTime(seg.start)} - {formatTime(seg.end)}"
          >
            <div data-edge="left" class="absolute inset-y-0 left-0 w-1.5 cursor-ew-resize bg-rose-300/85 hover:bg-white"></div>
            <span class="pointer-events-none truncate px-2">Bỏ {formatTimeShort(seg.start)}-{formatTimeShort(seg.end)}</span>
            <div data-edge="right" class="absolute inset-y-0 right-0 w-1.5 cursor-ew-resize bg-rose-300/85 hover:bg-white"></div>
          </div>
        {/each}
      {/if}
    {:else if kind === 'layer' && layer}
      {@const left = clamp(layer.startPct, 0, 100)}
      {@const width = clamp(Math.max(1.5, layer.endPct - layer.startPct), 1.5, 100 - left)}
      {@const isText = layer.type === 'text'}
      <div
        data-clip-item
        data-testid={`layer-clip-${layer.id}`}
        role="button"
        tabindex="0"
        onmousedown={() => onSelectLayer?.(layer.id)}
        onclick={() => onSelectLayer?.(layer.id)}
        onkeydown={(e) => e.key === 'Enter' && onSelectLayer?.(layer.id)}
        use:timelineClipInteract={{
          start: layer.startPct,
          end: layer.endPct,
          minBound: 0,
          maxBound: 100,
          minSpan: 0.5,
          playheadValue: currentOutputPct,
          trackWidthPx,
          onStart: () => onCommitHistory?.(),
          onChange: (s, e) => onUpdateLayer?.(layer.id, { startPct: s, endPct: e })
        }}
        class="absolute top-1 bottom-1 flex items-center justify-center overflow-hidden rounded border px-2 text-[10px] font-mono cursor-grab active:cursor-grabbing {!layer.visible ? 'opacity-35' : ''} {isText
          ? isLayerSelected ? 'border-white bg-purple-500/50 text-white ring-1 ring-purple-300' : 'border-purple-400/50 bg-purple-500/30 text-purple-100'
          : isLayerSelected ? 'border-white bg-cyan-500/50 text-white ring-1 ring-cyan-300' : 'border-cyan-400/50 bg-cyan-500/30 text-cyan-100'}"
        style="left: {left}%; width: {width}%;"
      >
        <div data-edge="left" class="absolute inset-y-0 left-0 w-1.5 cursor-ew-resize bg-white/60 hover:bg-white"></div>
        <span class="pointer-events-none truncate">
          {isText ? (layer.text || 'Chữ') : layer.displayName} ({layer.startPct}-{layer.endPct}%)
        </span>
        <div data-edge="right" class="absolute inset-y-0 right-0 w-1.5 cursor-ew-resize bg-white/60 hover:bg-white"></div>
      </div>
    {:else if kind === 'video-split' && duration > 0}
      {#each splitSegs as seg, i (seg.index)}
        {@const left = (seg.start / duration) * 100}
        {@const width = (seg.duration / duration) * 100}
        <div
          class="pointer-events-none absolute inset-y-0 flex items-center justify-center border-r border-white/10 {SEG_COLORS[i % SEG_COLORS.length]}"
          style="left: {left}%; width: {width}%;"
        >
          <span class="font-mono text-[10px] font-bold text-white/85">{String(seg.index).padStart(2, '0')}</span>
        </div>
      {/each}
      {#each splitMarkers as t, idx (idx)}
        {@const left = (t / duration) * 100}
        <div
          data-split-marker
          role="presentation"
          onmousedown={(e) => laneEl && onStartMarkerDrag?.(e, idx, laneEl)}
          ondblclick={() => splitMode === 'markers' && onRemoveSplitMarker?.(idx)}
          class="absolute -top-0.5 -bottom-0.5 z-10 flex w-3 -translate-x-1/2 cursor-ew-resize justify-center"
          style="left: {left}%;"
          title="Điểm cắt {formatTime(t)} (nhấp đúp để xóa)"
        >
          <div class="h-full w-0.5 rounded bg-white shadow-[0_0_8px_rgba(34,211,238,0.9)]"></div>
        </div>
      {/each}
    {/if}
  </div>
</div>

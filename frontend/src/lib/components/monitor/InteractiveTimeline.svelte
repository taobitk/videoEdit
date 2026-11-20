<script lang="ts">
  import type { OverlayLayer, TrimSegment, WorkspaceTab } from '$lib/types/studioTypes';
  import { clamp } from '$lib/utils/geometryMath';
  import { buildSplitSegments } from '$lib/utils/splitMath';
  import { computeRulerTicks } from '$lib/utils/timelineInteract';
  import { formatTimeShort } from '$lib/utils/timelineMath';
  import NleActionToolbar from './NleActionToolbar.svelte';
  import TimelineTrackRow from './TimelineTrackRow.svelte';

  interface Props {
    mode: WorkspaceTab;
    duration: number;
    playhead: number;
    isPlaying: boolean;
    speed?: number;
    outputDuration?: number;
    currentOutputPct?: number;
    timelineZoom?: number;
    trimEnabled?: boolean;
    trimSegments?: TrimSegment[];
    selectedTrimId?: string | null;
    layers?: OverlayLayer[];
    selectedLayerId?: string | null;
    canPaste?: boolean;
    canUndo?: boolean;
    canRedo?: boolean;
    splitMarkers?: number[];
    splitMode?: 'markers' | 'uniform';
    onSeek: (time: number) => void;
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
    onSelectTrim?: (id: string) => void;
    onUpdateTrimSegment?: (id: string, start: number, end: number, field: 'start' | 'end') => void;
    onSelectLayer?: (id: string) => void;
    onToggleLayerVisible?: (id: string, visible: boolean) => void;
    onCommitHistory?: () => void;
    onUpdateLayer?: (id: string, patch: Partial<OverlayLayer>) => void;
    onReorderLayer?: (fromId: string, toId: string) => void;
    onUpdateSplitMarkers?: (markers: number[]) => void;
    onRemoveSplitMarker?: (idx: number) => void;
  }

  let {
    mode, duration, playhead, isPlaying, speed = 1, outputDuration = 0,
    currentOutputPct = 0, timelineZoom = 1, trimEnabled = false,
    trimSegments = [], selectedTrimId = null, layers = [], selectedLayerId = null,
    canPaste = false, canUndo = false, canRedo = false,
    splitMarkers = [], splitMode = 'markers',
    onSeek, onTogglePlay, onStepSeek, onBladeCut, onSetIn, onSetOut,
    onAddText, onPickImage, onCopy, onPaste, onUndo, onRedo, onDelete, onZoomChange,
    onSelectTrim, onUpdateTrimSegment, onSelectLayer, onToggleLayerVisible,
    onCommitHistory, onUpdateLayer, onReorderLayer, onUpdateSplitMarkers, onRemoveSplitMarker
  }: Props = $props();

  let wrapEl = $state<HTMLDivElement | null>(null);
  let scrollEl = $state<HTMLDivElement | null>(null);
  let rulerEl = $state<HTMLDivElement | null>(null);
  let viewportW = $state(640);
  let timelineHeightPx = $state(180);
  let dragLayerId = $state<string | null>(null);

  const HEADER_W = 136;
  const baseTrackW = $derived(Math.max(200, viewportW - HEADER_W));
  const trackWidthPx = $derived(Math.max(baseTrackW, Math.round(baseTrackW * timelineZoom)));
  const playheadPct = $derived(duration > 0 ? clamp((playhead / duration) * 100, 0, 100) : 0);
  const ruler = $derived(computeRulerTicks(duration, timelineZoom));
  const reversedLayers = $derived([...layers].reverse());
  const splitSegCount = $derived(mode === 'split' ? buildSplitSegments(splitMarkers, duration).length : 0);

  $effect(() => {
    if (!scrollEl) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) viewportW = entry.contentRect.width;
    });
    ro.observe(scrollEl);
    return () => ro.disconnect();
  });

  $effect(() => {
    if (!wrapEl) return;
    const onWheel = (e: WheelEvent) => {
      if (!(e.ctrlKey || e.metaKey)) return;
      e.preventDefault();
      e.stopPropagation();
      const factor = e.deltaY < 0 ? 1.25 : 0.8;
      onZoomChange(clamp(timelineZoom * factor, 1, 50));
    };
    wrapEl.addEventListener('wheel', onWheel, { passive: false });
    return () => wrapEl?.removeEventListener('wheel', onWheel);
  });

  function timeFromX(clientX: number, target: HTMLElement): number {
    if (duration <= 0) return 0;
    const rect = target.getBoundingClientRect();
    if (rect.width <= 0) return 0;
    return clamp((clientX - rect.left) / rect.width, 0, 1) * duration;
  }

  function bindDrag(onMove: (me: MouseEvent) => void) {
    const onUp = () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }

  function startScrub(e: MouseEvent, target: HTMLElement) {
    if (duration <= 0) return;
    onSeek(timeFromX(e.clientX, target));
    bindDrag((me) => onSeek(timeFromX(me.clientX, target)));
  }

  function startMarkerDrag(e: MouseEvent, markerIdx: number, laneEl: HTMLDivElement) {
    if (splitMode !== 'markers' || !onUpdateSplitMarkers) return;
    e.stopPropagation();
    const sorted = [...splitMarkers].sort((a, b) => a - b);
    const minT = markerIdx > 0 ? sorted[markerIdx - 1] + 0.2 : 0.2;
    const maxT = markerIdx < sorted.length - 1 ? sorted[markerIdx + 1] - 0.2 : duration - 0.2;
    bindDrag((me) => {
      const next = [...sorted];
      next[markerIdx] = Math.round(clamp(timeFromX(me.clientX, laneEl), minT, maxT) * 10) / 10;
      onUpdateSplitMarkers(next);
    });
  }

  function startTimelineResize(e: MouseEvent) {
    e.preventDefault();
    const startY = e.clientY;
    const startH = timelineHeightPx;
    bindDrag((me) => (timelineHeightPx = clamp(startH + (startY - me.clientY), 120, 440)));
  }
</script>

<div
  bind:this={wrapEl}
  data-testid="interactive-timeline"
  class="relative w-full max-w-4xl rounded-xl border border-white/10 bg-[#0b0d17]/95 px-3.5 pb-2.5 pt-2 shadow-lg"
>
  <button
    type="button"
    aria-label="Chỉnh độ cao Timeline"
    data-testid="timeline-resize-handle"
    onmousedown={startTimelineResize}
    title="Kéo lên / xuống để chỉnh độ cao Timeline"
    class="mx-auto mb-1.5 flex h-2 w-24 cursor-ns-resize items-center justify-center rounded-full bg-white/10 hover:bg-purple-400/60"
  >
    <span class="h-0.5 w-10 rounded-full bg-white/40"></span>
  </button>
  <NleActionToolbar
    {mode} {duration} {playhead} {isPlaying} {speed} {outputDuration} {timelineZoom}
    splitSegmentCount={splitSegCount}
    hasLayerSelected={Boolean(selectedLayerId)}
    hasAnySelection={Boolean(selectedLayerId || selectedTrimId)}
    {canPaste} {canUndo} {canRedo}
    {onTogglePlay} {onStepSeek} {onBladeCut} {onSetIn} {onSetOut}
    {onAddText} {onPickImage} {onCopy} {onPaste} {onUndo} {onRedo} {onDelete} {onZoomChange}
  />

  <div
    bind:this={scrollEl}
    data-testid="timeline-scroll-area"
    style="height: {timelineHeightPx}px;"
    class="studio-scroll relative overflow-x-auto overflow-y-auto rounded-lg border border-white/10 bg-black/50"
  >
    <div class="relative" style="width: {HEADER_W + trackWidthPx}px;">
      <!-- Adaptive Time Ruler -->
      <div class="sticky top-0 z-30 flex border-b border-white/10 bg-[#0d101d]">
        <div class="sticky left-0 z-30 flex w-[136px] shrink-0 items-center justify-between border-r border-white/10 bg-[#0d101d] px-2.5 text-[10px] font-semibold uppercase tracking-wider text-gray-400">
          <span>Tracks</span>
          <span class="font-mono text-purple-300">{mode === 'edit' ? 1 + layers.length : 1}</span>
        </div>
        <div
          bind:this={rulerEl}
          role="slider"
          tabindex="0"
          aria-label="Timeline ruler"
          aria-valuemin={0}
          aria-valuemax={duration}
          aria-valuenow={playhead}
          onmousedown={(e) => rulerEl && startScrub(e, rulerEl)}
          class="relative h-6 shrink-0 cursor-pointer select-none overflow-hidden"
          style="width: {trackWidthPx}px;"
        >
          {#if duration > 0}
            {#each ruler.ticks as tick (tick)}
              {@const leftPct = clamp((tick / duration) * 100, 0, 100)}
              <div class="pointer-events-none absolute top-0 bottom-0 border-l border-white/15 pl-1 font-mono text-[9px] leading-6 text-gray-400" style="left: {leftPct}%;">
                {ruler.step < 1 ? `${tick.toFixed(1)}s` : formatTimeShort(tick)}
              </div>
            {/each}
          {/if}
        </div>
      </div>

      <!-- Multi-Track Lanes -->
      {#if mode === 'edit'}
        <TimelineTrackRow
          kind="video-trim" {duration} {playhead} {trackWidthPx}
          {trimEnabled} {trimSegments} {selectedTrimId}
          onLaneScrub={(e, el) => startScrub(e, el)}
          {onSelectTrim} {onUpdateTrimSegment}
        />
        {#each reversedLayers as layer (layer.id)}
          <TimelineTrackRow
            kind="layer" {duration} {playhead} {currentOutputPct} {trackWidthPx}
            {layer} isLayerSelected={layer.id === selectedLayerId}
            onLaneScrub={(e, el) => startScrub(e, el)}
            {onSelectLayer} {onToggleLayerVisible} {onCommitHistory} {onUpdateLayer}
            onTrackDragStart={(id) => (dragLayerId = id)}
            onTrackDrop={(targetId) => {
              if (dragLayerId && dragLayerId !== targetId) onReorderLayer?.(dragLayerId, targetId);
              dragLayerId = null;
            }}
          />
        {/each}
      {:else if mode === 'split'}
        <TimelineTrackRow
          kind="video-split" {duration} {playhead} {trackWidthPx}
          {splitMarkers} {splitMode}
          onLaneScrub={(e, el) => startScrub(e, el)}
          onStartMarkerDrag={startMarkerDrag} {onRemoveSplitMarker}
        />
      {/if}

      <!-- Continuous Playhead across Ruler + All Tracks -->
      {#if duration > 0}
        <div
          class="pointer-events-none absolute top-0 bottom-0 z-40 w-0.5 -translate-x-1/2 bg-pink-400 shadow-[0_0_10px_#ec4899]"
          style="left: {HEADER_W + (playheadPct / 100) * trackWidthPx}px;"
        >
          <div class="mx-auto h-2.5 w-2.5 -translate-x-[4px] rotate-45 bg-pink-400"></div>
        </div>
      {/if}
    </div>
  </div>
</div>

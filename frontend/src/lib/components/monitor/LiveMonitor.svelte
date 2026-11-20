<script lang="ts">
  import { Film, Layers, Link2 } from 'lucide-svelte';
  import { getPreviewUrl } from '$lib/api/studioClient';
  import { getStudioContext } from '$lib/composables/studioContext.svelte';
  import { getVideoCssFilter } from '$lib/utils/colorFilters';
  import { computeContainRect } from '$lib/utils/geometryMath';
  import { handleMonitorKeydown } from '$lib/utils/monitorShortcuts';
  import { findNextKeepTime, formatTime } from '$lib/utils/timelineMath';
  import InteractiveTimeline from './InteractiveTimeline.svelte';
  import KonvaOverlayStage from './KonvaOverlayStage.svelte';

  const ctx = getStudioContext();
  const { workspace, overlay, split, merge } = ctx;

  let viewportEl = $state<HTMLDivElement | null>(null);
  let videoEl = $state<HTMLVideoElement | null>(null);
  let boxW = $state(800);
  let boxH = $state(450);

  const activeTab = $derived(workspace.state.activeTab);
  const activePath = $derived(activeTab === 'split' ? split.state.path : workspace.state.videoPath);
  const activeMeta = $derived(activeTab === 'split' ? split.state.meta : workspace.state.videoMeta);
  const displayW = $derived(activeMeta && activeMeta.width > 0 ? activeMeta.width : 1920);
  const displayH = $derived(activeMeta && activeMeta.height > 0 ? activeMeta.height : 1080);
  const containRect = $derived(computeContainRect(boxW, boxH, displayW, displayH));
  const activeDuration = $derived(activeMeta?.duration ?? 0);

  $effect(() => {
    if (!viewportEl) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        boxW = entry.contentRect.width;
        boxH = entry.contentRect.height;
      }
    });
    ro.observe(viewportEl);
    return () => ro.disconnect();
  });

  $effect(() => {
    if (videoEl) {
      videoEl.playbackRate = activeTab === 'edit' ? workspace.state.speed : 1;
    }
  });

  function syncPlayStart() {
    workspace.state.isPlaying = true;
    if (!videoEl || activeTab !== 'edit' || !workspace.state.trimEnabled) return;
    const keeps = workspace.keepResult.keepSegments;
    const nextKeep = findNextKeepTime(videoEl.currentTime, keeps);
    if (nextKeep !== null) {
      videoEl.currentTime = nextKeep;
      workspace.state.playheadTime = nextKeep;
    }
  }

  function handleTimeUpdate() {
    if (!videoEl) return;
    const t = videoEl.currentTime;
    if (activeTab === 'split') {
      split.state.playhead = t;
      return;
    }
    const keeps = workspace.keepResult.keepSegments;
    if (workspace.state.trimEnabled && keeps.length > 0 && !videoEl.paused) {
      const lastEnd = keeps[keeps.length - 1].end;
      if (t >= lastEnd - 0.02) {
        videoEl.pause();
        videoEl.currentTime = keeps[0].start;
        workspace.state.playheadTime = keeps[0].start;
        return;
      }
      const nextKeep = findNextKeepTime(t, keeps);
      if (nextKeep !== null) {
        videoEl.currentTime = nextKeep;
        workspace.state.playheadTime = nextKeep;
        return;
      }
    }
    workspace.state.playheadTime = t;
  }

  function seekTo(t: number) {
    const next = Math.min(Math.max(0, t), Math.max(0, activeDuration));
    if (videoEl) videoEl.currentTime = next;
    if (activeTab === 'split') split.state.playhead = next;
    else workspace.state.playheadTime = next;
  }

  function stepSeek(delta: number) {
    seekTo((activeTab === 'split' ? split.state.playhead : workspace.state.playheadTime) + delta);
  }

  function togglePlay() {
    if (!videoEl) return;
    if (videoEl.paused) void videoEl.play();
    else videoEl.pause();
  }

  $effect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      handleMonitorKeydown(e, {
        activeTab,
        hasVideo: Boolean(activePath && activeMeta && activeDuration > 0),
        isInlineTextEditing: Boolean(overlay.editingTextLayerId),
        selectedLayerId: overlay.selectedLayerId,
        selectedTrimId: workspace.state.selectedTrimId,
        displayW,
        displayH,
        onTogglePlay: togglePlay,
        onSeekBy: stepSeek,
        onBladeCut: ctx.bladeCutAtPlayhead,
        onSetIn: () => ctx.setSelectedBoundary('start'),
        onSetOut: () => ctx.setSelectedBoundary('end'),
        onAddText: ctx.addTextAtPlayhead,
        onPickImage: overlay.pickAndAddImageLayer,
        onCopyLayer: overlay.copySelectedLayer,
        onPasteLayer: () => overlay.pasteLayer(false),
        onDuplicate: ctx.duplicateSelectedItem,
        onUndo: overlay.undo,
        onRedo: overlay.redo,
        onDeleteSelected: ctx.deleteSelectedItem,
        onNudge: overlay.nudgeSelected,
        onZoomIn: () => workspace.setTimelineZoom(workspace.state.timelineZoom * 1.5),
        onZoomOut: () => workspace.setTimelineZoom(workspace.state.timelineZoom / 1.5),
        onZoomReset: () => workspace.setTimelineZoom(1)
      });
    };
    window.addEventListener('keydown', onKeyDown, { capture: true });
    return () => window.removeEventListener('keydown', onKeyDown, { capture: true });
  });
</script>

<section class="relative flex min-w-0 flex-1 flex-col items-center justify-center gap-3 bg-black/25 p-5 overflow-hidden">
  {#if activeTab === 'merge'}
    <div class="flex max-w-md flex-col items-center text-center">
      <div class="mb-4 flex h-20 w-20 items-center justify-center rounded-full border border-cyan-500/30 bg-cyan-500/10 text-cyan-300">
        <Link2 class="h-9 w-9" />
      </div>
      <h3 class="text-sm font-bold uppercase tracking-wider text-gray-200">
        Chế độ Ghép Video ({merge.state.files.length} clip)
      </h3>
      <p class="mt-2 text-xs text-gray-400">
        Thêm từ 2 video trở lên ở thanh bên trái, sắp xếp thứ tự rồi nhấn Ghép ở bảng bên phải. Hệ thống tự chuẩn hóa độ phân giải &amp; âm thanh khi các clip khác thông số.
      </p>
    </div>
  {:else if !activePath || !activeMeta}
    <div class="flex flex-col items-center text-center">
      <div class="mb-4 flex h-20 w-20 items-center justify-center rounded-full border border-purple-500/25 bg-purple-500/10 text-purple-300">
        <Film class="h-9 w-9" />
      </div>
      <h3 class="text-sm font-bold uppercase tracking-wider text-gray-300">Monitor Chờ Tín Hiệu</h3>
      <p class="mt-1.5 max-w-xs text-xs text-gray-500">
        Chọn video từ thư viện bên trái hoặc dán đường dẫn tuyệt đối để kích hoạt màn hình biên tập.
      </p>
    </div>
  {:else}
    <div class="flex w-full max-w-4xl items-center justify-between px-1">
      <div class="flex items-center gap-2">
        <Layers class="h-4 w-4 text-purple-400" />
        <span class="text-xs font-bold uppercase tracking-wider text-purple-300">
          {activeTab === 'split' ? 'Split Timeline Preview' : 'Live Konva Monitor'}
        </span>
      </div>
      <div class="flex items-center gap-3 rounded-lg border border-white/10 bg-white/5 px-3 py-1 font-mono text-[11px] text-gray-300">
        <span class="max-w-[200px] truncate">{activePath.split(/[\\/]/).pop()}</span>
        <span class="text-white/20">|</span>
        <span class="text-cyan-300">{displayW}x{displayH}</span>
        <span class="text-white/20">|</span>
        <span class="text-purple-300">{formatTime(activeDuration)}</span>
      </div>
    </div>

    <div
      bind:this={viewportEl}
      data-testid="monitor-viewport"
      class="relative w-full max-w-4xl flex-1 min-h-[260px] overflow-hidden rounded-xl border border-white/10 bg-black shadow-2xl"
    >
      <video
        bind:this={videoEl}
        data-testid="preview-video"
        src={getPreviewUrl(activePath)}
        preload="metadata"
        onloadedmetadata={() => {
          if (videoEl) videoEl.playbackRate = activeTab === 'edit' ? workspace.state.speed : 1;
        }}
        ontimeupdate={handleTimeUpdate}
        onplay={syncPlayStart}
        onpause={() => (workspace.state.isPlaying = false)}
        class="pointer-events-none absolute object-contain"
        style="left: {containRect.left}px; top: {containRect.top}px; width: {containRect.width}px; height: {containRect.height}px; filter: {activeTab === 'edit' ? getVideoCssFilter(workspace.state.videoFilter) : 'none'};"
      >
        <track kind="captions" />
      </video>

      {#if activeTab === 'edit' && containRect.width > 0 && containRect.height > 0}
        <KonvaOverlayStage
          {containRect} videoW={displayW} videoH={displayH}
          layers={overlay.layers} selectedLayerId={overlay.selectedLayerId}
          currentOutputPct={workspace.currentOutputPct}
          onSelectLayer={ctx.selectLayerClean}
          onCommitHistory={overlay.commitHistory}
          onUpdateLayer={(id, patch) => overlay.updateLayer(id, patch)}
          onStartInlineEdit={overlay.startInlineTextEdit}
          onStopInlineEdit={overlay.stopInlineTextEdit}
        />
      {/if}
    </div>

    <InteractiveTimeline
      mode={activeTab} duration={activeDuration}
      playhead={activeTab === 'split' ? split.state.playhead : workspace.state.playheadTime}
      isPlaying={workspace.state.isPlaying} speed={workspace.state.speed}
      outputDuration={workspace.outputDuration} currentOutputPct={workspace.currentOutputPct}
      timelineZoom={workspace.state.timelineZoom} trimEnabled={workspace.state.trimEnabled}
      trimSegments={workspace.state.trimSegments} selectedTrimId={workspace.state.selectedTrimId}
      layers={overlay.layers} selectedLayerId={overlay.selectedLayerId}
      canPaste={Boolean(overlay.copiedLayer || overlay.selectedLayerId)}
      canUndo={overlay.canUndo} canRedo={overlay.canRedo}
      splitMarkers={split.effectiveMarkers} splitMode={split.state.mode}
      onSeek={seekTo} onTogglePlay={togglePlay} onStepSeek={stepSeek}
      onBladeCut={ctx.bladeCutAtPlayhead}
      onSetIn={() => ctx.setSelectedBoundary('start')}
      onSetOut={() => ctx.setSelectedBoundary('end')}
      onAddText={ctx.addTextAtPlayhead}
      onPickImage={overlay.pickAndAddImageLayer}
      onCopy={overlay.copySelectedLayer}
      onPaste={() => overlay.pasteLayer(false)}
      onUndo={overlay.undo} onRedo={overlay.redo} onDelete={ctx.deleteSelectedItem}
      onZoomChange={workspace.setTimelineZoom} onSelectTrim={ctx.selectTrimClean}
      onUpdateTrimSegment={workspace.updateTrimSegment}
      onSelectLayer={ctx.selectLayerClean}
      onToggleLayerVisible={(id, v) => overlay.updateLayer(id, { visible: v }, true)}
      onCommitHistory={overlay.commitHistory}
      onUpdateLayer={(id, patch) => overlay.updateLayer(id, patch)}
      onReorderLayer={overlay.reorderLayer}
      onUpdateSplitMarkers={split.setMarkers} onRemoveSplitMarker={split.removeMarkerAt}
    />
  {/if}
</section>

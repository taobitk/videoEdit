import { getContext, setContext } from 'svelte';
import { createMergeStore } from './mergeStore.svelte';
import { createOverlayEditorStore } from './overlayEditorStore.svelte';
import { createRenderJobStore } from './renderJobStore.svelte';
import { createSplitStore } from './splitStore.svelte';
import { createStudioWorkspace } from './studioWorkspace.svelte';

const STUDIO_CONTEXT_KEY = Symbol('velocity-studio-context');

export function createStudioContext() {
  const workspace = createStudioWorkspace();
  const overlay = createOverlayEditorStore();
  const split = createSplitStore();
  const merge = createMergeStore();
  const renderJob = createRenderJobStore();

  function startProcess() {
    if (!workspace.state.videoPath || !workspace.keepResult.isValid) return;
    void renderJob.launchProcess({
      source_type: 'local',
      local_path: workspace.state.videoPath,
      speed: workspace.state.speed,
      use_gpu: workspace.state.useGpu,
      codec: workspace.state.codec,
      preset: workspace.state.preset,
      quality_mode: workspace.state.qualityMode,
      video_filter: workspace.state.videoFilter,
      trim_enabled: workspace.state.trimEnabled,
      trim_segments: workspace.state.trimEnabled
        ? workspace.state.trimSegments.map((s) => ({ start: s.start, end: s.end }))
        : [],
      layers: overlay.toProcessPayloads(workspace.displayWidth)
    });
  }

  function startSplit() {
    if (!split.state.path || split.segments.length < 2) return;
    const isUniformParts = split.state.mode === 'uniform' && split.state.uniformType === 'parts';
    const isUniformDur = split.state.mode === 'uniform' && split.state.uniformType === 'duration';
    void renderJob.launchSplit({
      local_path: split.state.path,
      mode: split.state.mode,
      split_points: split.effectiveMarkers,
      segment_duration: isUniformDur ? split.state.uniformDuration : 0,
      parts: isUniformParts ? split.state.uniformParts : 0,
      accurate: split.state.accurate,
      use_gpu: split.state.useGpu,
      codec: split.state.codec,
      preset: split.state.preset,
      quality_mode: split.state.qualityMode
    });
  }

  function startMerge() {
    if (merge.state.files.length < 2) return;
    void renderJob.launchMerge({
      files: merge.state.files.map((f) => ({ path: f.path })),
      use_gpu: merge.state.useGpu,
      codec: merge.state.codec,
      preset: merge.state.preset,
      quality_mode: merge.state.qualityMode
    });
  }

  function requestClearMerge() {
    workspace.askConfirm({
      title: 'Xóa danh sách video ghép?',
      message: `Bạn có chắc muốn xóa toàn bộ ${merge.state.files.length} video khỏi danh sách ghép không?`,
      confirmLabel: 'Xóa tất cả',
      onConfirm: () => {
        merge.clearFiles();
        workspace.closeConfirm();
      }
    });
  }

  function selectLayerClean(id: string | null) {
    overlay.selectLayer(id);
    if (id) workspace.selectTrim(null);
  }

  function selectTrimClean(id: string | null) {
    workspace.selectTrim(id);
    if (id) overlay.selectLayer(null);
  }

  function bladeCutAtPlayhead() {
    if (workspace.state.activeTab === 'split') {
      split.addMarkerAtPlayhead();
    } else if (workspace.state.activeTab === 'edit') {
      workspace.cutAtPlayhead();
      overlay.selectLayer(null);
    }
  }

  function setSelectedBoundary(edge: 'start' | 'end') {
    if (overlay.selectedLayerId) overlay.setLayerInOutAtPct(edge, workspace.currentOutputPct);
    else if (workspace.state.selectedTrimId) workspace.setSelectedTrimBoundary(edge);
  }

  function duplicateSelectedItem() {
    if (overlay.selectedLayerId) overlay.pasteLayer(true);
    else if (workspace.state.selectedTrimId) workspace.duplicateSelectedTrim();
  }

  function deleteSelectedItem() {
    if (overlay.selectedLayerId) overlay.removeLayer(overlay.selectedLayerId);
    else if (workspace.state.selectedTrimId) workspace.removeTrimSegment(workspace.state.selectedTrimId);
  }

  function addTextAtPlayhead() {
    const startPct = workspace.sourceDuration > 0 && workspace.currentOutputPct < 95 ? workspace.currentOutputPct : 0;
    overlay.addTextLayer(workspace.displayHeight, startPct);
    workspace.selectTrim(null);
  }

  const ctx = {
    workspace,
    overlay,
    split,
    merge,
    renderJob,
    startProcess,
    startSplit,
    startMerge,
    requestClearMerge,
    selectLayerClean,
    selectTrimClean,
    bladeCutAtPlayhead,
    setSelectedBoundary,
    duplicateSelectedItem,
    deleteSelectedItem,
    addTextAtPlayhead
  };

  setContext(STUDIO_CONTEXT_KEY, ctx);
  return ctx;
}

export type StudioContext = ReturnType<typeof createStudioContext>;

export function getStudioContext(): StudioContext {
  return getContext<StudioContext>(STUDIO_CONTEXT_KEY);
}

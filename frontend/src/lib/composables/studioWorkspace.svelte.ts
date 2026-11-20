import { fetchCapabilities, probeMedia, selectFile } from '$lib/api/studioClient';
import type {
  CodecId,
  ConfirmModalState,
  PresetId,
  ProbeResult,
  QualityMode,
  SystemCapabilities,
  TrimSegment,
  VideoFilterId,
  WorkspaceTab
} from '$lib/types/studioTypes';
import { clamp } from '$lib/utils/geometryMath';
import {
  clampTrimSegment,
  computeCutSegmentBounds,
  computeDuplicateTrimBounds,
  computeKeepSegments,
  computeOutputDuration,
  mapSourceTimeToOutputPct
} from '$lib/utils/timelineMath';

export function createStudioWorkspace() {
  const state = $state({
    activeTab: 'edit' as WorkspaceTab,
    capabilities: null as SystemCapabilities | null,
    capabilitiesError: '',
    videoPath: '',
    videoMeta: null as ProbeResult | null,
    probeLoading: false,
    probeError: '',
    speed: 1.2,
    videoFilter: 'none' as VideoFilterId,
    useGpu: true,
    codec: 'h264' as CodecId,
    preset: 'p4' as PresetId,
    qualityMode: 'auto_cap' as QualityMode,
    trimEnabled: false,
    trimSegments: [] as TrimSegment[],
    selectedTrimId: null as string | null,
    timelineZoom: 1,
    playheadTime: 0,
    isPlaying: false,
    confirmModal: { open: false, title: '', message: '' } as ConfirmModalState
  });

  const sourceDuration = $derived(state.videoMeta?.duration ?? 0);
  const displayWidth = $derived(
    state.videoMeta && state.videoMeta.width > 0 ? state.videoMeta.width : 1920
  );
  const displayHeight = $derived(
    state.videoMeta && state.videoMeta.height > 0 ? state.videoMeta.height : 1080
  );
  const keepResult = $derived(
    computeKeepSegments(sourceDuration, state.trimEnabled, state.trimSegments)
  );
  const keptDuration = $derived(keepResult.keptDuration);
  const outputDuration = $derived(computeOutputDuration(keptDuration, state.speed));
  const currentOutputPct = $derived(
    mapSourceTimeToOutputPct(state.playheadTime, keepResult.keepSegments, keptDuration)
  );

  async function initCapabilities() {
    try {
      const caps = await fetchCapabilities();
      state.capabilities = caps;
      state.capabilitiesError = '';
      if (!caps.has_nvidia_gpu || (!caps.nvenc_h264 && !caps.nvenc_hevc)) {
        state.useGpu = false;
      }
    } catch (err) {
      state.capabilitiesError = err instanceof Error ? err.message : 'Lỗi kết nối backend';
    }
  }

  async function loadEditVideo(path: string) {
    const cleanPath = path.trim();
    state.videoPath = cleanPath;
    state.probeError = '';
    if (!cleanPath) {
      state.videoMeta = null;
      state.trimSegments = [];
      state.selectedTrimId = null;
      state.playheadTime = 0;
      return;
    }
    state.probeLoading = true;
    try {
      const meta = await probeMedia(cleanPath);
      state.videoMeta = meta;
      state.trimSegments = [];
      state.selectedTrimId = null;
      state.timelineZoom = 1;
      state.playheadTime = 0;
      state.isPlaying = false;
    } catch (err) {
      state.videoMeta = null;
      state.probeError = err instanceof Error ? err.message : 'Không thể đọc metadata video';
    } finally {
      state.probeLoading = false;
    }
  }

  async function browseEditVideo() {
    try {
      const res = await selectFile('video');
      if (res.success && res.path) await loadEditVideo(res.path);
    } catch (err) {
      state.probeError = err instanceof Error ? err.message : 'Không thể mở hộp thoại chọn tệp';
    }
  }

  function setSpeed(next: number) {
    state.speed = Math.round(clamp(next, 0.25, 8.0) * 100) / 100;
  }

  const makeTrimId = () => `trim-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
  function selectTrim(id: string | null) { state.selectedTrimId = id; }
  function setTimelineZoom(z: number) { state.timelineZoom = Math.round(clamp(z, 1, 50) * 100) / 100; }

  function addTrimSegment() {
    const dur = sourceDuration > 0 ? sourceDuration : 10;
    const defaultStart = clamp(state.playheadTime, 0, Math.max(0, dur - 0.5));
    const defaultEnd = clamp(defaultStart + Math.min(2, dur * 0.25), 0.1, dur);
    const id = makeTrimId();
    state.trimSegments = [
      ...state.trimSegments,
      { id, start: Math.round(defaultStart * 100) / 100, end: Math.round(defaultEnd * 100) / 100 }
    ];
    state.selectedTrimId = id;
  }

  function cutAtPlayhead() {
    state.trimEnabled = true;
    const bounds = computeCutSegmentBounds(state.playheadTime, sourceDuration);
    const id = makeTrimId();
    state.trimSegments = [...state.trimSegments, { id, start: bounds.start, end: bounds.end }];
    state.selectedTrimId = id;
  }

  function updateTrimSegment(id: string, start: number, end: number, editedField: 'start' | 'end') {
    const dur = sourceDuration > 0 ? sourceDuration : Math.max(start, end, 10);
    const clamped = clampTrimSegment(start, end, dur, editedField);
    state.trimSegments = state.trimSegments.map((seg) =>
      seg.id === id ? { ...seg, start: clamped.start, end: clamped.end } : seg
    );
  }

  function setSelectedTrimBoundary(edge: 'start' | 'end', targetId?: string) {
    const seg = state.trimSegments.find((s) => s.id === (targetId ?? state.selectedTrimId));
    if (!seg) return;
    updateTrimSegment(seg.id, edge === 'start' ? state.playheadTime : seg.start, edge === 'end' ? state.playheadTime : seg.end, edge);
  }

  function duplicateSelectedTrim() {
    const seg = state.trimSegments.find((s) => s.id === state.selectedTrimId);
    if (!seg) return;
    const bounds = computeDuplicateTrimBounds(seg, sourceDuration);
    const id = makeTrimId();
    state.trimEnabled = true;
    state.trimSegments = [...state.trimSegments, { id, start: bounds.start, end: bounds.end }];
    state.selectedTrimId = id;
  }

  function removeTrimSegment(id: string) {
    state.trimSegments = state.trimSegments.filter((seg) => seg.id !== id);
    if (state.selectedTrimId === id) state.selectedTrimId = state.trimSegments[0]?.id ?? null;
  }

  function askConfirm(opts: Omit<ConfirmModalState, 'open'>) { state.confirmModal = { ...opts, open: true }; }
  function closeConfirm() { state.confirmModal = { open: false, title: '', message: '' }; }

  return {
    state,
    get sourceDuration() { return sourceDuration; },
    get displayWidth() { return displayWidth; },
    get displayHeight() { return displayHeight; },
    get keepResult() { return keepResult; },
    get keptDuration() { return keptDuration; },
    get outputDuration() { return outputDuration; },
    get currentOutputPct() { return currentOutputPct; },
    initCapabilities, loadEditVideo, browseEditVideo, setSpeed,
    selectTrim, setTimelineZoom, addTrimSegment, cutAtPlayhead,
    updateTrimSegment, setSelectedTrimBoundary, duplicateSelectedTrim,
    removeTrimSegment, askConfirm, closeConfirm
  };
}

import { probeMedia, selectFile } from '$lib/api/studioClient';
import type {
  CodecId,
  PresetId,
  ProbeResult,
  QualityMode,
  SplitMode,
  UniformSplitMode
} from '$lib/types/studioTypes';
import {
  buildSplitSegments,
  computeUniformSplitPoints,
  normalizeSplitMarkers
} from '$lib/utils/splitMath';

export function createSplitStore() {
  const state = $state({
    path: '',
    meta: null as ProbeResult | null,
    probeLoading: false,
    error: '',
    mode: 'markers' as SplitMode,
    uniformType: 'parts' as UniformSplitMode,
    uniformParts: 3,
    uniformDuration: 30,
    markers: [] as number[],
    accurate: false,
    useGpu: true,
    codec: 'h264' as CodecId,
    preset: 'p4' as PresetId,
    qualityMode: 'high' as QualityMode,
    cq: 20,
    playhead: 0
  });

  const duration = $derived(state.meta?.duration ?? 0);
  const uniformCalc = $derived(
    computeUniformSplitPoints(
      duration,
      state.uniformType,
      state.uniformType === 'parts' ? state.uniformParts : state.uniformDuration
    )
  );
  const effectiveMarkers = $derived(
    state.mode === 'uniform'
      ? uniformCalc.splitPoints
      : normalizeSplitMarkers(state.markers, duration)
  );
  const segments = $derived(buildSplitSegments(effectiveMarkers, duration));

  async function loadVideo(path: string) {
    const clean = path.trim();
    state.path = clean;
    state.error = '';
    if (!clean) {
      state.meta = null;
      state.markers = [];
      state.playhead = 0;
      return;
    }
    state.probeLoading = true;
    try {
      const meta = await probeMedia(clean);
      state.meta = meta;
      state.markers = [];
      state.playhead = 0;
    } catch (err) {
      state.meta = null;
      state.error = err instanceof Error ? err.message : 'Không thể đọc thời lượng video';
    } finally {
      state.probeLoading = false;
    }
  }

  async function browseVideo() {
    try {
      const res = await selectFile('video');
      if (res.success && res.path) await loadVideo(res.path);
    } catch (err) {
      state.error = err instanceof Error ? err.message : 'Không thể chọn video';
    }
  }

  function addWidestMarker() {
    if (duration <= 0.4) return;
    state.mode = 'markers';
    const segs = buildSplitSegments(state.markers, duration);
    if (segs.length === 0) return;
    let widest = segs[0];
    for (const seg of segs) {
      if (seg.duration > widest.duration) widest = seg;
    }
    const mid = Math.round(((widest.start + widest.end) / 2) * 10) / 10;
    state.markers = normalizeSplitMarkers([...state.markers, mid], duration);
  }

  function addMarkerAt(time: number) {
    if (duration <= 0.4) return;
    state.mode = 'markers';
    state.markers = normalizeSplitMarkers([...state.markers, time], duration);
  }

  function addMarkerAtPlayhead() {
    if (duration <= 0.4) return;
    const t = state.playhead;
    const nearExisting = effectiveMarkers.some((m) => Math.abs(m - t) < 0.2);
    if (t > 0.2 && t < duration - 0.2 && !nearExisting) addMarkerAt(t);
    else addWidestMarker();
  }

  function setMarkers(next: number[]) {
    state.markers = normalizeSplitMarkers(next, duration);
  }

  function removeMarkerAt(idx: number) {
    const sorted = normalizeSplitMarkers(state.markers, duration);
    state.markers = sorted.filter((_, i) => i !== idx);
  }

  function clearMarkers() {
    state.markers = [];
  }

  return {
    state,
    get duration() { return duration; },
    get uniformSegmentDuration() { return uniformCalc.segmentDuration; },
    get effectiveMarkers() { return effectiveMarkers; },
    get segments() { return segments; },
    loadVideo,
    browseVideo,
    addWidestMarker,
    addMarkerAt,
    addMarkerAtPlayhead,
    setMarkers,
    removeMarkerAt,
    clearMarkers
  };
}

import { probeMedia, selectFile } from '$lib/api/studioClient';
import type { CodecId, MergeFileItem, PresetId, QualityMode } from '$lib/types/studioTypes';

export function createMergeStore() {
  const state = $state({
    files: [] as MergeFileItem[],
    error: '',
    useGpu: true,
    codec: 'h264' as CodecId,
    preset: 'p4' as PresetId,
    qualityMode: 'high' as QualityMode,
    cq: 20
  });

  async function addPath(path: string) {
    const clean = path.trim();
    if (!clean) return;
    state.error = '';
    const id = `mf-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const displayName = clean.split(/[\\/]/).pop() || clean;
    state.files = [...state.files, { id, path: clean, displayName }];
    try {
      const probe = await probeMedia(clean);
      state.files = state.files.map((f) => (f.id === id ? { ...f, probe } : f));
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Lỗi đọc file';
      state.files = state.files.map((f) => (f.id === id ? { ...f, probeError: msg } : f));
    }
  }

  async function browseAndAdd() {
    try {
      const res = await selectFile('video');
      if (res.success && res.path) await addPath(res.path);
    } catch (err) {
      state.error = err instanceof Error ? err.message : 'Không thể thêm video';
    }
  }

  function moveFile(id: string, dir: 'up' | 'down') {
    const idx = state.files.findIndex((f) => f.id === id);
    if (idx === -1) return;
    const target = dir === 'up' ? idx - 1 : idx + 1;
    if (target < 0 || target >= state.files.length) return;
    const next = [...state.files];
    [next[idx], next[target]] = [next[target], next[idx]];
    state.files = next;
  }

  function removeFile(id: string) {
    state.files = state.files.filter((f) => f.id !== id);
  }

  function clearFiles() {
    state.files = [];
  }

  return {
    state,
    addPath,
    browseAndAdd,
    moveFile,
    removeFile,
    clearFiles
  };
}

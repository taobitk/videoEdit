import {
  cancelRender,
  fetchStatus,
  resetEngine,
  showInExplorer,
  startMerge,
  startProcess,
  startSplit,
  subscribeStatusEvents
} from '$lib/api/studioClient';
import type {
  EngineStatus,
  MergePayload,
  ProcessPayload,
  SplitPayload
} from '$lib/types/studioTypes';

const INITIAL_STATUS: EngineStatus = {
  status: 'idle',
  mode: 'process',
  progress: 0,
  fps: 0,
  speed: '0.0x',
  time_processed: '00:00:00',
  eta: 'Calculating...',
  current_step: '',
  active_encoder: 'CPU',
  encoder_detail: '',
  error_message: '',
  output_filename: '',
  output_url: '',
  output_files: [],
  warnings: [],
  logs: []
};

export function createRenderJobStore() {
  let state = $state<EngineStatus>({ ...INITIAL_STATUS });
  let unsubscribeSse: (() => void) | null = null;
  let pollTimer: ReturnType<typeof setInterval> | null = null;

  function stopStream() {
    if (unsubscribeSse) {
      unsubscribeSse();
      unsubscribeSse = null;
    }
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  function applyStatus(next: EngineStatus) {
    state = {
      ...INITIAL_STATUS,
      ...next,
      output_files: Array.isArray(next.output_files) ? next.output_files : [],
      warnings: Array.isArray(next.warnings) ? next.warnings : [],
      logs: Array.isArray(next.logs) ? next.logs : []
    };
    if (next.status !== 'processing') stopStream();
  }

  function startPollingFallback() {
    if (pollTimer) return;
    pollTimer = setInterval(async () => {
      try {
        applyStatus(await fetchStatus());
      } catch {
        // Ignore transient drops while polling
      }
    }, 450);
  }

  function startLiveStream() {
    stopStream();
    if (typeof EventSource === 'undefined') {
      startPollingFallback();
      return;
    }
    unsubscribeSse = subscribeStatusEvents(applyStatus, () => {
      if (unsubscribeSse) {
        unsubscribeSse();
        unsubscribeSse = null;
      }
      if (state.status === 'processing') startPollingFallback();
    });
  }

  async function syncInitialStatus() {
    try {
      const current = await fetchStatus();
      applyStatus(current);
      if (current.status === 'processing') startLiveStream();
    } catch {
      // Backend may not be running during isolated unit check
    }
  }

  async function runJob(
    mode: EngineStatus['mode'],
    step: string,
    starter: () => Promise<void>
  ) {
    stopStream();
    state = {
      ...INITIAL_STATUS,
      status: 'processing',
      mode,
      current_step: step,
      logs: ['Đang kết nối VelocityEngine...']
    };
    try {
      await starter();
      startLiveStream();
    } catch (err) {
      applyStatus({
        ...INITIAL_STATUS,
        status: 'failed',
        mode,
        error_message: err instanceof Error ? err.message : 'Không thể bắt đầu tác vụ'
      });
    }
  }

  async function cancel() {
    try {
      await cancelRender();
    } catch {
      // Reflected via status update
    }
  }

  async function reset() {
    stopStream();
    try {
      await resetEngine();
    } catch {
      // Ignore if backend already idle
    }
    state = { ...INITIAL_STATUS };
  }

  return {
    get state() { return state; },
    syncInitialStatus,
    launchProcess: (p: ProcessPayload) =>
      runJob('process', 'Đang khởi tạo bộ xử lý...', () => startProcess(p)),
    launchSplit: (p: SplitPayload) =>
      runJob('split', 'Đang chuẩn bị chia video...', () => startSplit(p)),
    launchMerge: (p: MergePayload) =>
      runJob('merge', 'Đang phân tích và ghép video...', () => startMerge(p)),
    cancel,
    reset,
    revealOutput: (filename: string) =>
      filename ? showInExplorer(filename) : Promise.resolve(),
    dispose: stopStream
  };
}

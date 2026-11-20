import type {
  EngineStatus,
  MergePayload,
  ProbeResult,
  ProcessPayload,
  SelectFileResult,
  SplitPayload,
  SystemCapabilities
} from '$lib/types/studioTypes';

async function parseJsonOrThrow<T>(res: Response, fallbackMsg: string): Promise<T> {
  const text = await res.text();
  let data: Record<string, unknown> = {};
  if (text) {
    try {
      data = JSON.parse(text) as Record<string, unknown>;
    } catch {
      if (!res.ok) throw new Error(text || fallbackMsg);
    }
  }
  if (!res.ok) {
    const msg =
      (typeof data.error === 'string' && data.error) ||
      (typeof data.message === 'string' && data.message) ||
      fallbackMsg;
    throw new Error(msg);
  }
  return data as unknown as T;
}

export function getPreviewUrl(localPath: string): string {
  return `/api/preview?path=${encodeURIComponent(localPath)}`;
}

export async function fetchCapabilities(): Promise<SystemCapabilities> {
  const res = await fetch('/api/system/capabilities');
  return parseJsonOrThrow<SystemCapabilities>(res, 'Không thể kiểm tra cấu hình GPU/FFmpeg');
}

export async function fetchStatus(): Promise<EngineStatus> {
  const res = await fetch('/api/status');
  return parseJsonOrThrow<EngineStatus>(res, 'Không thể đọc trạng thái render');
}

export function subscribeStatusEvents(
  onStatus: (status: EngineStatus) => void,
  onError: () => void
): () => void {
  const source = new EventSource('/api/events');
  source.onmessage = (event) => {
    try {
      const parsed = JSON.parse(event.data) as EngineStatus;
      onStatus(parsed);
    } catch {
      // Ignore non-JSON keepalives
    }
  };
  source.onerror = () => {
    onError();
  };
  return () => {
    source.close();
  };
}

export async function selectFile(fileType: 'video' | 'image'): Promise<SelectFileResult> {
  const res = await fetch('/api/select_file', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ file_type: fileType })
  });
  return parseJsonOrThrow<SelectFileResult>(res, 'Không thể mở hộp thoại chọn tệp');
}

export async function probeMedia(path: string): Promise<ProbeResult> {
  const res = await fetch('/api/probe', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path })
  });
  const data = await parseJsonOrThrow<ProbeResult & { success?: boolean; error?: string }>(
    res,
    'Không thể phân tích tệp video'
  );
  if (data.success === false) {
    throw new Error(data.error || 'Không thể phân tích tệp video');
  }
  return data;
}

export async function startProcess(payload: ProcessPayload): Promise<void> {
  const res = await fetch('/api/process', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  });
  await parseJsonOrThrow<unknown>(res, 'Không thể bắt đầu xử lý video');
}

export async function startSplit(payload: SplitPayload): Promise<void> {
  const res = await fetch('/api/split', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  });
  await parseJsonOrThrow<unknown>(res, 'Không thể bắt đầu chia video');
}

export async function startMerge(payload: MergePayload): Promise<void> {
  const res = await fetch('/api/merge', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  });
  await parseJsonOrThrow<unknown>(res, 'Không thể bắt đầu ghép video');
}

export async function cancelRender(): Promise<void> {
  const res = await fetch('/api/cancel', { method: 'POST' });
  await parseJsonOrThrow<unknown>(res, 'Không thể hủy tác vụ');
}

export async function resetEngine(): Promise<void> {
  const res = await fetch('/api/reset', { method: 'POST' });
  await parseJsonOrThrow<unknown>(res, 'Không thể đặt lại trạng thái');
}

export async function showInExplorer(filename: string): Promise<void> {
  await fetch('/api/show_explorer', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ filename })
  });
}

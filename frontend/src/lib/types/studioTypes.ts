export type WorkspaceTab = 'edit' | 'split' | 'merge';

export type VideoFilterId = 'none' | 'grayscale' | 'sepia' | 'warm' | 'cool' | 'vibrant';

export type CodecId = 'h264' | 'hevc';
export type PresetId = 'p1' | 'p4' | 'p7';
export type QualityMode = 'auto_cap' | 'compact' | 'high';
export type FontWeightOption = 'bold' | 'normal';
export type SplitMode = 'markers' | 'uniform';
export type UniformSplitMode = 'parts' | 'duration';

export interface SystemCapabilities {
  ffmpeg_available: boolean;
  ffprobe_available: boolean;
  has_nvidia_gpu: boolean;
  gpu_name: string;
  nvenc_h264: boolean;
  nvenc_hevc: boolean;
}

export interface ProbeResult {
  width: number;
  height: number;
  raw_width?: number;
  raw_height?: number;
  rotation?: number;
  duration: number;
  fps?: number;
  video_codec?: string;
  has_audio?: boolean;
  audio_codec?: string;
  start_time?: number;
  size_mb?: number;
  bitrate_kbps?: number;
}

export interface SelectFileResult {
  success: boolean;
  path?: string;
  name?: string;
  size_mb?: number;
  message?: string;
}

export interface TrimSegment {
  id: string;
  start: number;
  end: number;
}

export interface BaseOverlayLayer {
  id: string;
  type: 'text' | 'image';
  visible: boolean;
  xRatio: number;
  yRatio: number;
  opacity: number;
  startPct: number;
  endPct: number;
}

export interface TextLayer extends BaseOverlayLayer {
  type: 'text';
  text: string;
  fontSize: number;
  fontWeight: FontWeightOption;
  fontColor: string;
  borderColor: string;
  borderWidth: number;
  bgEnabled: boolean;
  bgColor: string;
  bgOpacity: number;
  bgPadding: number;
}

export interface ImageLayer extends BaseOverlayLayer {
  type: 'image';
  filename: string;
  displayName: string;
  widthRatio: number;
}

export type OverlayLayer = TextLayer | ImageLayer;

export interface MergeFileItem {
  id: string;
  path: string;
  displayName: string;
  probe?: ProbeResult;
  probeError?: string;
}

export interface ProcessLayerPayload {
  type: 'image' | 'text';
  filename?: string;
  raster_data_url?: string;
  width_ratio: number;
  x_ratio: number;
  y_ratio: number;
  opacity: number;
  start_pct: number;
  end_pct: number;
}

export interface ProcessPayload {
  source_type: 'local';
  local_path: string;
  speed: number;
  use_gpu: boolean;
  codec: CodecId;
  preset: PresetId;
  quality_mode: QualityMode;
  video_filter: VideoFilterId;
  trim_enabled: boolean;
  trim_segments: Array<{ start: number; end: number }>;
  layers: ProcessLayerPayload[];
}

export interface SplitPayload {
  local_path: string;
  mode: SplitMode;
  split_points: number[];
  segment_duration: number;
  parts?: number;
  accurate: boolean;
  use_gpu: boolean;
  codec: CodecId;
  preset: PresetId;
  quality_mode: QualityMode;
}

export interface MergePayload {
  files: Array<{ path: string }>;
  use_gpu: boolean;
  codec: CodecId;
  preset: PresetId;
  quality_mode: QualityMode;
}

export interface OutputFileItem {
  filename: string;
  url: string;
}

export interface EngineStatus {
  status: 'idle' | 'processing' | 'completed' | 'failed';
  mode: 'process' | 'split' | 'merge';
  progress: number;
  fps: number;
  speed: string;
  time_processed: string;
  eta: string;
  current_step: string;
  active_encoder: string;
  encoder_detail?: string;
  error_message: string;
  output_filename: string;
  output_url: string;
  output_files: OutputFileItem[];
  warnings: string[];
  logs: string[];
}

export interface ConfirmModalState {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  onConfirm?: () => void;
}

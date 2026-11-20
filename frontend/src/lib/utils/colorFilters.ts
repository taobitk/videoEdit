import type { VideoFilterId } from '$lib/types/studioTypes';

export interface VideoFilterOption {
  id: VideoFilterId;
  label: string;
}

export interface SvgFilterDef {
  id: Exclude<VideoFilterId, 'none'>;
  svgId: string;
  type: 'saturate' | 'matrix';
  values: string;
}

export const VIDEO_FILTER_OPTIONS: VideoFilterOption[] = [
  { id: 'none', label: 'Mặc định (Original)' },
  { id: 'grayscale', label: 'Đen trắng (Grayscale)' },
  { id: 'sepia', label: 'Cổ điển (Sepia)' },
  { id: 'warm', label: 'Nắng ấm (Warm Tint)' },
  { id: 'cool', label: 'Lạnh xanh (Cool Blue)' },
  { id: 'vibrant', label: 'Rực rỡ (Vibrant +40%)' }
];

/**
 * 1:1 color matrix parity with Go FFmpeg filters:
 * - grayscale: hue=s=0
 * - sepia: colorchannelmixer=.393:.769:.189:0:.349:.686:.168:0:.272:.534:.131
 * - warm: colorchannelmixer=rr=1.12:gg=1.02:bb=0.86
 * - cool: colorchannelmixer=rr=0.86:gg=1.00:bb=1.14
 * - vibrant: hue=s=1.4
 */
export const SVG_COLOR_FILTERS: SvgFilterDef[] = [
  {
    id: 'grayscale',
    svgId: 've-filter-grayscale',
    type: 'saturate',
    values: '0'
  },
  {
    id: 'sepia',
    svgId: 've-filter-sepia',
    type: 'matrix',
    values: '0.393 0.769 0.189 0 0  0.349 0.686 0.168 0 0  0.272 0.534 0.131 0 0  0 0 0 1 0'
  },
  {
    id: 'warm',
    svgId: 've-filter-warm',
    type: 'matrix',
    values: '1.12 0 0 0 0  0 1.02 0 0 0  0 0 0.86 0 0  0 0 0 1 0'
  },
  {
    id: 'cool',
    svgId: 've-filter-cool',
    type: 'matrix',
    values: '0.86 0 0 0 0  0 1.00 0 0 0  0 0 1.14 0 0  0 0 0 1 0'
  },
  {
    id: 'vibrant',
    svgId: 've-filter-vibrant',
    type: 'saturate',
    values: '1.4'
  }
];

export function getVideoCssFilter(filterId: VideoFilterId): string {
  if (filterId === 'none') return 'none';
  return `url(#ve-filter-${filterId})`;
}

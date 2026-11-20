<script lang="ts">
  import { ArrowLeftToLine, ArrowRightToLine, Maximize2 } from 'lucide-svelte';

  interface Props {
    xRatio: number;
    yRatio: number;
    startPct: number;
    endPct: number;
    accent?: 'purple' | 'cyan';
    onSnap: (xRatio: number, yRatio: number) => void;
    onSetIn: () => void;
    onSetOut: () => void;
    onFullSpan: () => void;
  }

  let {
    xRatio, yRatio, startPct, endPct, accent = 'purple',
    onSnap, onSetIn, onSetOut, onFullSpan
  }: Props = $props();

  const ALIGN_POINTS = [
    { key: 'tl', x: 0.15, y: 0.15, align: 'items-start justify-start', label: 'Trên trái' },
    { key: 'tc', x: 0.5, y: 0.15, align: 'items-start justify-center', label: 'Trên giữa' },
    { key: 'tr', x: 0.85, y: 0.15, align: 'items-start justify-end', label: 'Trên phải' },
    { key: 'ml', x: 0.15, y: 0.5, align: 'items-center justify-start', label: 'Giữa trái' },
    { key: 'mc', x: 0.5, y: 0.5, align: 'items-center justify-center', label: 'Chính giữa' },
    { key: 'mr', x: 0.85, y: 0.5, align: 'items-center justify-end', label: 'Giữa phải' },
    { key: 'bl', x: 0.15, y: 0.85, align: 'items-end justify-start', label: 'Dưới trái' },
    { key: 'bc', x: 0.5, y: 0.85, align: 'items-end justify-center', label: 'Dưới giữa' },
    { key: 'br', x: 0.85, y: 0.85, align: 'items-end justify-end', label: 'Dưới phải' }
  ];

  const activeCls = $derived(
    accent === 'cyan'
      ? 'border-cyan-400 bg-cyan-500/25 text-cyan-200'
      : 'border-purple-400 bg-purple-500/25 text-purple-200'
  );
  const dotCls = $derived(accent === 'cyan' ? 'bg-cyan-300' : 'bg-purple-300');
</script>

<div class="flex flex-col gap-2 border-t border-white/10 pt-2">
  <div class="flex items-start justify-between gap-2">
    <div class="flex flex-col gap-1">
      <span class="text-[10px] font-semibold text-gray-400">Căn nhanh 9 điểm</span>
      <div class="grid grid-cols-3 gap-1" data-testid="align-grid-9pt">
        {#each ALIGN_POINTS as pt (pt.key)}
          {@const isActive = Math.abs(xRatio - pt.x) < 0.03 && Math.abs(yRatio - pt.y) < 0.03}
          <button
            type="button"
            data-testid={`align-pt-${pt.key}`}
            title={pt.label}
            onclick={() => onSnap(pt.x, pt.y)}
            class="flex h-6 w-6 rounded border p-1 transition-colors {pt.align} {isActive
              ? activeCls
              : 'border-white/10 bg-black/40 hover:bg-white/15'}"
          >
            <span class="h-1.5 w-1.5 rounded-full {isActive ? dotCls : 'bg-gray-400'}"></span>
          </button>
        {/each}
      </div>
    </div>

    <div class="flex flex-1 flex-col gap-1.5">
      <div class="flex items-center justify-between text-[10px] text-gray-400">
        <span>Xuất hiện</span>
        <span data-testid="layer-span-badge" class="font-mono {accent === 'cyan' ? 'text-cyan-300' : 'text-purple-300'}">
          {startPct}% - {endPct}%
        </span>
      </div>
      <div class="grid grid-cols-3 gap-1">
        <button
          type="button"
          data-testid="btn-layer-in"
          onclick={onSetIn}
          title="Đặt điểm bắt đầu tại Playhead (I / [)"
          class="flex items-center justify-center gap-1 rounded-md border border-white/10 bg-white/5 py-1.5 text-[10px] font-semibold text-gray-200 hover:bg-white/15"
        >
          <ArrowRightToLine class="h-3 w-3 text-cyan-300" /> Đầu
        </button>
        <button
          type="button"
          data-testid="btn-layer-out"
          onclick={onSetOut}
          title="Đặt điểm kết thúc tại Playhead (O / ])"
          class="flex items-center justify-center gap-1 rounded-md border border-white/10 bg-white/5 py-1.5 text-[10px] font-semibold text-gray-200 hover:bg-white/15"
        >
          <ArrowLeftToLine class="h-3 w-3 text-cyan-300" /> Cuối
        </button>
        <button
          type="button"
          data-testid="btn-layer-full-span"
          onclick={onFullSpan}
          title="Hiển thị toàn bộ 0% - 100%"
          class="flex items-center justify-center gap-1 rounded-md border border-white/10 bg-white/5 py-1.5 text-[10px] font-semibold text-gray-200 hover:bg-white/15"
        >
          <Maximize2 class="h-3 w-3 text-purple-300" /> Full
        </button>
      </div>
    </div>
  </div>
</div>

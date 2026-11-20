<script lang="ts">
  import { Cpu, Zap } from 'lucide-svelte';
  import type { SystemCapabilities } from '$lib/types/studioTypes';

  interface Props {
    capabilities: SystemCapabilities | null;
    error: string;
  }

  let { capabilities, error }: Props = $props();

  const gpuReady = $derived(
    Boolean(capabilities?.has_nvidia_gpu && (capabilities.nvenc_h264 || capabilities.nvenc_hevc))
  );
  const ffmpegMissing = $derived(capabilities !== null && !capabilities.ffmpeg_available);
</script>

<header
  class="flex shrink-0 items-center justify-between border-b border-white/10 bg-[#0a0b12]/80 px-6 py-2.5 backdrop-blur-md z-10"
>
  <div class="flex items-center gap-3.5">
    <div
      class="flex h-10 w-10 items-center justify-center rounded-xl border border-purple-500/30 bg-purple-500/15 shadow-[0_0_20px_rgba(139,92,246,0.2)]"
    >
      <Zap class="h-5 w-5 text-purple-400" />
    </div>
    <div>
      <h1 class="m-0 flex items-center gap-2 text-base font-bold leading-none text-gray-100">
        VelocityEngine
        <span
          class="rounded border border-cyan-500/30 bg-cyan-500/15 px-1.5 py-0.5 text-[11px] font-extrabold tracking-wide text-cyan-300"
        >
          v2 RTX
        </span>
      </h1>
      <p class="mt-1 text-[11px] text-gray-400">
        GPU-Accelerated Video Studio &bull; Konva WYSIWYG
      </p>
    </div>
  </div>

  <div
    data-testid="gpu-status-badge"
    class="flex items-center gap-2.5 rounded-full border border-white/10 bg-white/5 px-3.5 py-1.5 text-xs"
  >
    {#if error}
      <span class="relative h-2.5 w-2.5 rounded-full bg-rose-500"></span>
      <span class="font-medium text-rose-300">Mất kết nối Engine</span>
    {:else if !capabilities}
      <span class="relative h-2.5 w-2.5 animate-pulse rounded-full bg-gray-400"></span>
      <span class="text-gray-400">Đang kiểm tra GPU...</span>
    {:else if ffmpegMissing}
      <span class="relative h-2.5 w-2.5 rounded-full bg-rose-500"></span>
      <span class="font-semibold text-rose-300">Thiếu FFmpeg</span>
    {:else if gpuReady}
      <span class="relative h-2.5 w-2.5 rounded-full bg-emerald-400 text-emerald-400 status-ripple"></span>
      <span class="font-semibold text-emerald-300">
        {capabilities.gpu_name || 'NVIDIA GPU'} &bull; NVENC Ready
      </span>
    {:else}
      <span class="relative h-2.5 w-2.5 rounded-full bg-amber-400"></span>
      <Cpu class="h-3.5 w-3.5 text-amber-300" />
      <span class="font-medium text-amber-200">
        {capabilities.gpu_name ? `${capabilities.gpu_name} (CPU Render)` : 'CPU Mode'}
      </span>
    {/if}
  </div>
</header>

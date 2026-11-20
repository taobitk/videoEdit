<script lang="ts">
  import { AlertTriangle } from 'lucide-svelte';

  interface Props {
    open: boolean;
    title: string;
    message: string;
    confirmLabel?: string;
    cancelLabel?: string;
    onConfirm: () => void;
    onCancel: () => void;
  }

  let {
    open,
    title,
    message,
    confirmLabel = 'Xác nhận',
    cancelLabel = 'Hủy bỏ',
    onConfirm,
    onCancel
  }: Props = $props();

  function handleKeydown(e: KeyboardEvent) {
    if (open && e.key === 'Escape') {
      onCancel();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
  >
    <div
      class="w-full max-w-md rounded-2xl border border-purple-500/30 bg-[#0f111c] p-5 shadow-2xl"
    >
      <div class="flex items-start gap-3">
        <div
          class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-amber-500/30 bg-amber-500/15 text-amber-400"
        >
          <AlertTriangle class="h-5 w-5" />
        </div>
        <div class="flex-1">
          <h3 class="text-sm font-bold text-gray-100">{title}</h3>
          <p class="mt-1.5 text-xs leading-relaxed text-gray-400">{message}</p>
        </div>
      </div>

      <div class="mt-5 flex justify-end gap-2.5">
        <button
          type="button"
          onclick={onCancel}
          class="rounded-lg border border-white/10 bg-white/5 px-4 py-2 text-xs font-semibold text-gray-300 hover:bg-white/10"
        >
          {cancelLabel}
        </button>
        <button
          type="button"
          onclick={onConfirm}
          class="btn-neon rounded-lg px-4 py-2 text-xs font-bold text-white"
        >
          {confirmLabel}
        </button>
      </div>
    </div>
  </div>
{/if}

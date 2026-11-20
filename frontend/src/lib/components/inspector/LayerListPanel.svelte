<script lang="ts">
  import {
    ArrowDown,
    ArrowUp,
    Eye,
    EyeOff,
    GripVertical,
    Image as ImageIcon,
    Redo2,
    Trash2,
    Type,
    Undo2
  } from 'lucide-svelte';
  import type { OverlayLayer } from '$lib/types/studioTypes';

  interface Props {
    layers: OverlayLayer[];
    selectedLayerId: string | null;
    canUndo: boolean;
    canRedo: boolean;
    errorMsg: string;
    onAddText: () => void;
    onPickImage: () => void;
    onAddImagePath: (path: string) => void;
    onSelect: (id: string) => void;
    onMoveZ: (id: string, dir: 'up' | 'down') => void;
    onReorder?: (fromId: string, toId: string) => void;
    onToggleVisible: (id: string, visible: boolean) => void;
    onRemove: (id: string) => void;
    onUndo: () => void;
    onRedo: () => void;
  }

  let {
    layers,
    selectedLayerId,
    canUndo,
    canRedo,
    errorMsg,
    onAddText,
    onPickImage,
    onAddImagePath,
    onSelect,
    onMoveZ,
    onReorder,
    onToggleVisible,
    onRemove,
    onUndo,
    onRedo
  }: Props = $props();

  let showImgPathBox = $state(false);
  let imgPathDraft = $state('');
  let dragFromId = $state<string | null>(null);

  const displayLayers = $derived([...layers].reverse());

  function submitImagePath() {
    const clean = imgPathDraft.trim();
    if (!clean) return;
    onAddImagePath(clean);
    imgPathDraft = '';
    showImgPathBox = false;
  }
</script>

<div class="flex flex-col gap-2 rounded-xl border border-white/10 bg-black/30 p-3">
  <div class="flex items-center justify-between gap-2">
    <span class="text-xs font-bold text-purple-300">Lớp phủ (Z-Order)</span>
    <div class="flex items-center gap-1">
      <button
        type="button"
        data-testid="btn-undo-layer"
        onclick={onUndo}
        disabled={!canUndo}
        class="rounded border border-white/10 bg-white/5 p-1 text-gray-300 hover:bg-white/10 disabled:opacity-30"
        title="Hoàn tác (Ctrl+Z)"
      >
        <Undo2 class="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        data-testid="btn-redo-layer"
        onclick={onRedo}
        disabled={!canRedo}
        class="rounded border border-white/10 bg-white/5 p-1 text-gray-300 hover:bg-white/10 disabled:opacity-30"
        title="Làm lại (Ctrl+Y)"
      >
        <Redo2 class="h-3.5 w-3.5" />
      </button>
    </div>
  </div>

  <div class="grid grid-cols-2 gap-1.5">
    <button
      type="button"
      data-testid="btn-add-text-layer"
      onclick={onAddText}
      class="flex items-center justify-center gap-1.5 rounded-lg border border-purple-500/35 bg-purple-500/15 py-1.5 text-xs font-semibold text-purple-200 hover:bg-purple-500/25"
    >
      <Type class="h-3.5 w-3.5" /> + Thêm Chữ
    </button>
    <div class="flex gap-1">
      <button
        type="button"
        data-testid="btn-add-image-layer"
        onclick={onPickImage}
        class="flex flex-1 items-center justify-center gap-1 rounded-lg border border-cyan-500/35 bg-cyan-500/15 py-1.5 text-xs font-semibold text-cyan-200 hover:bg-cyan-500/25"
      >
        <ImageIcon class="h-3.5 w-3.5" /> + Ảnh
      </button>
      <button
        type="button"
        data-testid="btn-toggle-image-path"
        onclick={() => (showImgPathBox = !showImgPathBox)}
        class="rounded-lg border border-white/10 bg-white/5 px-2 text-[10px] font-mono text-gray-300 hover:bg-white/10"
        title="Nhập đường dẫn ảnh"
      >
        Path
      </button>
    </div>
  </div>

  {#if showImgPathBox}
    <div class="flex gap-1">
      <input
        type="text"
        data-testid="input-image-layer-path"
        bind:value={imgPathDraft}
        onkeydown={(e) => e.key === 'Enter' && submitImagePath()}
        placeholder="D:\logo.png"
        class="w-full rounded border border-white/15 bg-black/50 px-2 py-1 font-mono text-[11px] text-gray-100 outline-none focus:border-cyan-500"
      />
      <button
        type="button"
        data-testid="btn-confirm-image-path"
        onclick={submitImagePath}
        class="rounded border border-cyan-500/35 bg-cyan-500/20 px-2 text-[11px] font-semibold text-cyan-200"
      >
        OK
      </button>
    </div>
  {/if}

  {#if errorMsg}
    <p class="text-[11px] text-rose-400">{errorMsg}</p>
  {/if}

  {#if layers.length === 0}
    <p class="py-2 text-center text-[11px] text-gray-500">
      Chưa có lớp phủ nào. Nhấn + Thêm Chữ hoặc + Ảnh.
    </p>
  {:else}
    <div class="flex max-h-40 flex-col gap-1 overflow-y-auto pr-1 studio-scroll" data-testid="layers-list">
      {#each displayLayers as layer, revIdx (layer.id)}
        {@const isFront = revIdx === 0}
        {@const isBack = revIdx === displayLayers.length - 1}
        <div
          role="button"
          tabindex="0"
          draggable="true"
          data-testid={`layer-row-${layer.id}`}
          ondragstart={() => (dragFromId = layer.id)}
          ondragover={(e) => e.preventDefault()}
          ondrop={() => {
            if (dragFromId && dragFromId !== layer.id) onReorder?.(dragFromId, layer.id);
            dragFromId = null;
          }}
          onclick={() => onSelect(layer.id)}
          onkeydown={(e) => e.key === 'Enter' && onSelect(layer.id)}
          class="flex items-center gap-1.5 rounded-lg border px-2 py-1.5 text-xs transition-colors cursor-pointer {layer.id === selectedLayerId
            ? 'border-purple-500/50 bg-purple-500/20 text-purple-100'
            : 'border-white/5 bg-white/5 text-gray-300 hover:bg-white/10'}"
        >
          <GripVertical class="h-3 w-3 shrink-0 cursor-grab text-gray-500" />
          {#if layer.type === 'text'}
            <Type class="h-3.5 w-3.5 shrink-0 text-purple-300" />
            <span class="min-w-0 flex-1 truncate font-medium">{layer.text || '(Trống)'}</span>
          {:else}
            <ImageIcon class="h-3.5 w-3.5 shrink-0 text-cyan-300" />
            <span class="min-w-0 flex-1 truncate font-medium">{layer.displayName}</span>
          {/if}

          <span class="font-mono text-[10px] text-gray-400">
            {layer.startPct}-{layer.endPct}%
          </span>

          <button
            type="button"
            onclick={(e) => {
              e.stopPropagation();
              onMoveZ(layer.id, 'up');
            }}
            disabled={isFront}
            class="rounded p-0.5 text-gray-400 hover:text-white disabled:opacity-25"
            title="Đưa lên trên"
          >
            <ArrowUp class="h-3 w-3" />
          </button>
          <button
            type="button"
            onclick={(e) => {
              e.stopPropagation();
              onMoveZ(layer.id, 'down');
            }}
            disabled={isBack}
            class="rounded p-0.5 text-gray-400 hover:text-white disabled:opacity-25"
            title="Đưa xuống dưới"
          >
            <ArrowDown class="h-3 w-3" />
          </button>
          <button
            type="button"
            onclick={(e) => {
              e.stopPropagation();
              onToggleVisible(layer.id, !layer.visible);
            }}
            class="rounded p-0.5 text-gray-400 hover:text-white"
            title={layer.visible ? 'Ẩn lớp' : 'Hiện lớp'}
          >
            {#if layer.visible}
              <Eye class="h-3 w-3" />
            {:else}
              <EyeOff class="h-3 w-3 text-gray-600" />
            {/if}
          </button>
          <button
            type="button"
            onclick={(e) => {
              e.stopPropagation();
              onRemove(layer.id);
            }}
            class="rounded p-0.5 text-rose-400 hover:bg-rose-500/15"
            title="Xóa lớp"
          >
            <Trash2 class="h-3 w-3" />
          </button>
        </div>
      {/each}
    </div>
  {/if}
</div>

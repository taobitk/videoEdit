<script lang="ts">
  import { getStudioContext } from '$lib/composables/studioContext.svelte';
  import EditStudioPanel from './EditStudioPanel.svelte';
  import MergeActionPanel from './MergeActionPanel.svelte';
  import RenderStatusView from './RenderStatusView.svelte';
  import SplitActionPanel from './SplitActionPanel.svelte';

  const ctx = getStudioContext();
  const { workspace, overlay, split, merge, renderJob, startProcess, startSplit, startMerge } = ctx;
</script>

<aside
  class="studio-scroll flex h-full w-[380px] shrink-0 flex-col gap-4 overflow-y-auto border-l border-white/10 bg-[#090a12]/75 p-4"
>
  {#if renderJob.state.status !== 'idle'}
    <RenderStatusView
      status={renderJob.state}
      onCancel={renderJob.cancel}
      onReset={renderJob.reset}
      onReveal={renderJob.revealOutput}
    />
  {:else if workspace.state.activeTab === 'edit'}
    <EditStudioPanel
      canStart={Boolean(workspace.state.videoPath && workspace.state.videoMeta && workspace.keepResult.isValid)}
      speed={workspace.state.speed}
      videoFilter={workspace.state.videoFilter}
      useGpu={workspace.state.useGpu}
      codec={workspace.state.codec}
      preset={workspace.state.preset}
      qualityMode={workspace.state.qualityMode}
      capabilities={workspace.state.capabilities}
      layers={overlay.layers}
      selectedLayer={overlay.selectedLayer}
      canUndo={overlay.canUndo}
      canRedo={overlay.canRedo}
      layerError={overlay.errorMsg}
      onSpeedChange={workspace.setSpeed}
      onFilterChange={(f) => (workspace.state.videoFilter = f)}
      onChangeGpu={(v) => (workspace.state.useGpu = v)}
      onChangeCodec={(c) => (workspace.state.codec = c)}
      onChangePreset={(p) => (workspace.state.preset = p)}
      onChangeQualityMode={(q) => (workspace.state.qualityMode = q)}
      onAddText={ctx.addTextAtPlayhead}
      onPickImage={overlay.pickAndAddImageLayer}
      onAddImagePath={(p) => overlay.addImageLayer(p)}
      onSelectLayer={ctx.selectLayerClean}
      onMoveZ={overlay.moveLayerZ}
      onReorderLayer={overlay.reorderLayer}
      onToggleVisible={(id, v) => overlay.updateLayer(id, { visible: v }, true)}
      onRemoveLayer={overlay.removeLayer}
      onUpdateLayer={(id, patch) => overlay.updateLayer(id, patch)}
      onSnapLayer={(xr, yr) =>
        overlay.snapSelectedLayerTo(xr, yr, workspace.displayWidth, workspace.displayHeight)
      }
      onSetLayerIn={() => overlay.setLayerInOutAtPct('start', workspace.currentOutputPct)}
      onSetLayerOut={() => overlay.setLayerInOutAtPct('end', workspace.currentOutputPct)}
      onLayerFullSpan={() =>
        overlay.selectedLayerId &&
        overlay.updateLayer(overlay.selectedLayerId, { startPct: 0, endPct: 100 }, true)
      }
      onUndo={overlay.undo}
      onRedo={overlay.redo}
      onStartRender={startProcess}
    />
  {:else if workspace.state.activeTab === 'split'}
    <SplitActionPanel
      canStart={Boolean(split.state.path && split.segments.length >= 2)}
      segmentCount={split.segments.length}
      accurate={split.state.accurate}
      useGpu={split.state.useGpu}
      codec={split.state.codec}
      preset={split.state.preset}
      qualityMode={split.state.qualityMode}
      capabilities={workspace.state.capabilities}
      onChangeAccurate={(v) => (split.state.accurate = v)}
      onChangeGpu={(v) => (split.state.useGpu = v)}
      onChangeCodec={(c) => (split.state.codec = c)}
      onChangePreset={(p) => (split.state.preset = p)}
      onChangeQualityMode={(q) => (split.state.qualityMode = q)}
      onStartSplit={startSplit}
    />
  {:else}
    <MergeActionPanel
      clipCount={merge.state.files.length}
      useGpu={merge.state.useGpu}
      codec={merge.state.codec}
      preset={merge.state.preset}
      qualityMode={merge.state.qualityMode}
      capabilities={workspace.state.capabilities}
      onChangeGpu={(v) => (merge.state.useGpu = v)}
      onChangeCodec={(c) => (merge.state.codec = c)}
      onChangePreset={(p) => (merge.state.preset = p)}
      onChangeQualityMode={(q) => (merge.state.qualityMode = q)}
      onStartMerge={startMerge}
    />
  {/if}
</aside>

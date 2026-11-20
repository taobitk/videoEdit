<script lang="ts">
  import { FolderOpen, Link2, Scissors } from 'lucide-svelte';
  import { getStudioContext } from '$lib/composables/studioContext.svelte';
  import type { WorkspaceTab } from '$lib/types/studioTypes';
  import EditSourcePanel from './EditSourcePanel.svelte';
  import MergePanel from './MergePanel.svelte';
  import SplitPanel from './SplitPanel.svelte';

  const ctx = getStudioContext();
  const { workspace, split, merge, requestClearMerge } = ctx;

  const tabs: Array<{ id: WorkspaceTab; label: string; icon: WorkspaceTab }> = [
    { id: 'edit', label: 'Biên tập', icon: 'edit' },
    { id: 'split', label: 'Chia', icon: 'split' },
    { id: 'merge', label: 'Ghép', icon: 'merge' }
  ];
</script>

<aside
  class="studio-scroll flex h-full w-[340px] shrink-0 flex-col gap-4 overflow-y-auto border-r border-white/10 bg-[#090a12]/75 p-4"
>
  <div>
    <h2 class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-purple-300">
      <FolderOpen class="h-4 w-4" /> Thư viện &amp; Nguồn
    </h2>
    <p class="mt-0.5 text-[11px] text-gray-400">Nạp video và thiết lập cắt ghép</p>
  </div>

  <div class="grid grid-cols-3 gap-1 rounded-xl border border-white/10 bg-black/35 p-1">
    {#each tabs as tab (tab.id)}
      <button
        type="button"
        data-testid={`tab-${tab.id}`}
        onclick={() => (workspace.state.activeTab = tab.id)}
        class="flex items-center justify-center gap-1.5 rounded-lg px-2 py-2 text-xs font-semibold transition-all {workspace.state.activeTab === tab.id
          ? 'border border-purple-500/35 bg-purple-500/20 text-purple-200 shadow-sm'
          : 'border border-transparent text-gray-400 hover:text-gray-200'}"
      >
        {#if tab.icon === 'edit'}
          <FolderOpen class="h-3.5 w-3.5" />
        {:else if tab.icon === 'split'}
          <Scissors class="h-3.5 w-3.5" />
        {:else}
          <Link2 class="h-3.5 w-3.5" />
        {/if}
        <span>{tab.label}</span>
      </button>
    {/each}
  </div>

  <div class="flex flex-1 flex-col gap-3">
    {#if workspace.state.activeTab === 'edit'}
      <EditSourcePanel
        videoPath={workspace.state.videoPath}
        videoMeta={workspace.state.videoMeta}
        probeLoading={workspace.state.probeLoading}
        probeError={workspace.state.probeError}
        trimEnabled={workspace.state.trimEnabled}
        trimSegments={workspace.state.trimSegments}
        selectedTrimId={workspace.state.selectedTrimId}
        keptDuration={workspace.keptDuration}
        trimValid={workspace.keepResult.isValid}
        onBrowse={workspace.browseEditVideo}
        onLoadPath={workspace.loadEditVideo}
        onToggleTrim={(v) => (workspace.state.trimEnabled = v)}
        onAddTrim={workspace.addTrimSegment}
        onCutAtPlayhead={workspace.cutAtPlayhead}
        onSelectTrim={ctx.selectTrimClean}
        onSetTrimBoundary={(id, edge) => workspace.setSelectedTrimBoundary(edge, id)}
        onRemoveTrim={workspace.removeTrimSegment}
      />
    {:else if workspace.state.activeTab === 'split'}
      <SplitPanel
        path={split.state.path}
        duration={split.duration}
        loading={split.state.probeLoading}
        error={split.state.error}
        mode={split.state.mode}
        uniformType={split.state.uniformType}
        uniformParts={split.state.uniformParts}
        uniformDuration={split.state.uniformDuration}
        segments={split.segments}
        onBrowse={split.browseVideo}
        onLoadPath={split.loadVideo}
        onModeChange={(m) => (split.state.mode = m)}
        onUniformTypeChange={(u) => (split.state.uniformType = u)}
        onUniformPartsChange={(n) => (split.state.uniformParts = n)}
        onUniformDurationChange={(s) => (split.state.uniformDuration = s)}
        onAddMarker={split.addMarkerAtPlayhead}
        onClearMarkers={split.clearMarkers}
      />
    {:else}
      <MergePanel
        files={merge.state.files}
        error={merge.state.error}
        onBrowseAdd={merge.browseAndAdd}
        onAddPath={merge.addPath}
        onMoveFile={merge.moveFile}
        onRemoveFile={merge.removeFile}
        onRequestClearAll={requestClearMerge}
      />
    {/if}
  </div>
</aside>

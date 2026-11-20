<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import HeaderBar from '$lib/components/HeaderBar.svelte';
  import InspectorPanel from '$lib/components/inspector/InspectorPanel.svelte';
  import MediaLibrarySidebar from '$lib/components/library/MediaLibrarySidebar.svelte';
  import LiveMonitor from '$lib/components/monitor/LiveMonitor.svelte';
  import { createStudioContext } from '$lib/composables/studioContext.svelte';

  const { workspace, renderJob } = createStudioContext();

  onMount(() => {
    void workspace.initCapabilities();
    void renderJob.syncInitialStatus();
  });

  onDestroy(() => renderJob.dispose());
</script>

<div class="ambient-glow -top-24 -left-24 h-[460px] w-[460px] bg-purple-600"></div>
<div class="ambient-glow -bottom-32 -right-24 h-[520px] w-[520px] bg-pink-600"></div>
<div class="ambient-glow top-1/3 left-1/2 h-[380px] w-[380px] bg-cyan-500"></div>

<div class="relative z-10 flex h-screen w-screen flex-col overflow-hidden">
  <HeaderBar capabilities={workspace.state.capabilities} error={workspace.state.capabilitiesError} />

  <main class="flex min-h-0 flex-1 overflow-hidden">
    <MediaLibrarySidebar />
    <LiveMonitor />
    <InspectorPanel />
  </main>
</div>

<ConfirmModal
  open={workspace.state.confirmModal.open}
  title={workspace.state.confirmModal.title}
  message={workspace.state.confirmModal.message}
  confirmLabel={workspace.state.confirmModal.confirmLabel}
  onConfirm={() => workspace.state.confirmModal.onConfirm?.()}
  onCancel={workspace.closeConfirm}
/>

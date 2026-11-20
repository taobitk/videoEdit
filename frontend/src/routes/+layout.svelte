<script lang="ts">
  import '../app.css';
  import SvgFilterDefs from '$lib/components/SvgFilterDefs.svelte';

  let { children } = $props();
</script>

<SvgFilterDefs />
{@render children()}

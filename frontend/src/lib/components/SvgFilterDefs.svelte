<script lang="ts">
  import { SVG_COLOR_FILTERS } from '$lib/utils/colorFilters';
</script>

<svg class="pointer-events-none absolute h-0 w-0 overflow-hidden" aria-hidden="true">
  <defs>
    {#each SVG_COLOR_FILTERS as f (f.svgId)}
      <filter id={f.svgId} color-interpolation-filters="sRGB">
        <feColorMatrix type={f.type} values={f.values} />
      </filter>
    {/each}
  </defs>
</svg>

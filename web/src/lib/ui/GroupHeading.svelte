<script lang="ts">
// The one subheading: h2 above a page-level group of sections, h3 inside a Section or tile.
let { label, level = 3, id }: { label: string; level?: 2 | 3; id?: string } = $props();
</script>

{#if level === 2}
  <h2 {id} class="text-sm font-medium text-muted-foreground">{label}</h2>
{:else}
  <h3 {id} class="text-sm font-medium text-muted-foreground">{label}</h3>
{/if}

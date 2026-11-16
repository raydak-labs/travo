<script lang="ts">
import type { Snippet } from 'svelte';
import Badge from '$lib/components/ui/badge/badge.svelte';
import { type StatusKind, statusClass } from './status';

let { kind, children }: { kind: StatusKind; children: Snippet } = $props();
</script>

<Badge class={statusClass(kind)}>{@render children()}</Badge>

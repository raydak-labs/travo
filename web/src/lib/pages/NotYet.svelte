<script lang="ts">
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
</script>

<Card title={m.not_yet_title()}>
  <p class="text-muted-foreground">{m.not_yet_body()}</p>
</Card>

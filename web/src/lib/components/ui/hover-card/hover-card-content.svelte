<script lang="ts">
import { LinkPreview as HoverCardPrimitive } from 'bits-ui';
import type { ComponentProps } from 'svelte';
import { cn, type WithoutChildrenOrChild } from '$lib/utils.js';
import HoverCardPortal from './hover-card-portal.svelte';

let {
  ref = $bindable(null),
  class: className,
  side = 'bottom',
  sideOffset = 4,
  align = 'center',
  portalProps,
  ...restProps
}: HoverCardPrimitive.ContentProps & {
  portalProps?: WithoutChildrenOrChild<ComponentProps<typeof HoverCardPortal>>;
} = $props();
</script>

<HoverCardPortal {...portalProps}>
	<HoverCardPrimitive.Content
		bind:ref
		data-slot="hover-card-content"
		{side}
		{sideOffset}
		{align}
		class={cn(
			"data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2 ring-foreground/10 bg-popover text-popover-foreground w-64 rounded-lg p-4 text-sm shadow-md ring-1 duration-100 z-50 origin-(--bits-link-preview-content-transform-origin) outline-hidden",
			className
		)}
		{...restProps}
	/>
</HoverCardPortal>

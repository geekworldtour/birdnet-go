<script lang="ts">
  import { cn } from '$lib/utils/cn';
  import type { Snippet } from 'svelte';
  import { navigationIcons } from '$lib/utils/icons';

  interface Props {
    title: string;
    description?: string;
    defaultOpen?: boolean;
    className?: string;
    hasChanges?: boolean;
    children?: Snippet;
  }

  let {
    title,
    description,
    defaultOpen = true,
    className = '',
    hasChanges = false,
    children,
  }: Props = $props();

  let isOpen = $state(defaultOpen);

  function toggleOpen() {
    isOpen = !isOpen;
  }
</script>

<div class={cn('collapse bg-base-100 shadow-xs', { 'collapse-open': isOpen }, className)}>
  <button
    type="button"
    class="collapse-title px-6 py-4 min-h-0 text-left w-full cursor-pointer hover:bg-base-200/50 transition-colors"
    onclick={toggleOpen}
    aria-expanded={isOpen}
    aria-controls="collapse-content-{title.toLowerCase().replace(/\s+/g, '-')}"
  >
    <div class="flex items-center gap-2">
      <h3 class="text-xl font-semibold">{title}</h3>
      {#if hasChanges}
        <span class="badge badge-primary badge-sm" role="status" aria-label="Settings changed">
          changed
        </span>
      {/if}
      <!-- Collapse indicator -->
      <div
        class="ml-auto w-4 h-4 transition-transform duration-200"
        class:rotate-180={isOpen}
        aria-hidden="true"
      >
        {@html navigationIcons.chevronDown}
      </div>
    </div>
    {#if description}
      <p class="text-sm text-base-content/70 mt-1">{description}</p>
    {/if}
  </button>

  <div
    class="collapse-content px-6 pb-6"
    id="collapse-content-{title.toLowerCase().replace(/\s+/g, '-')}"
  >
    {#if children}
      {@render children()}
    {/if}
  </div>
</div>

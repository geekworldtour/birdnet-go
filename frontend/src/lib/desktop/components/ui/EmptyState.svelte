<script lang="ts">
  import { cn } from '$lib/utils/cn';
  import type { Snippet } from 'svelte';
  import type { HTMLAttributes } from 'svelte/elements';
  import { dataIcons } from '$lib/utils/icons';

  interface ActionConfig {
    label: string;
    onClick: () => void;
  }

  interface Props extends HTMLAttributes<HTMLDivElement> {
    icon?: Snippet;
    title?: string;
    description?: string;
    action?: ActionConfig | null;
    className?: string;
    children?: Snippet;
  }

  let {
    icon,
    title = '',
    description = '',
    action = null,
    className = '',
    children,
    ...rest
  }: Props = $props();
</script>

<div
  class={cn('flex flex-col items-center justify-center py-12 px-4 text-center', className)}
  {...rest}
>
  {#if icon}
    {@render icon()}
  {:else}
    {@html dataIcons.inbox}
  {/if}

  {#if title}
    <h3 class="mt-4 text-lg font-semibold text-base-content">{title}</h3>
  {/if}

  {#if description}
    <p class="mt-2 text-sm text-base-content/70 max-w-md">{description}</p>
  {/if}

  {#if children}
    <div class="mt-4">
      {@render children()}
    </div>
  {/if}

  {#if action}
    <div class="mt-6">
      <button type="button" class="btn btn-primary" onclick={action.onClick}>
        {action.label}
      </button>
    </div>
  {/if}
</div>

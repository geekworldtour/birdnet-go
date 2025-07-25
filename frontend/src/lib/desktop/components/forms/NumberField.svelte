<script lang="ts">
  import FormField from './FormField.svelte';
  import type { HTMLAttributes } from 'svelte/elements';

  interface Props extends HTMLAttributes<HTMLDivElement> {
    label: string;
    value: number;
    onUpdate: (_value: number) => void;
    min?: number;
    max?: number;
    step?: number;
    placeholder?: string;
    helpText?: string;
    required?: boolean;
    disabled?: boolean;
    error?: string;
    className?: string;
  }

  let {
    label,
    value = $bindable(),
    onUpdate,
    min,
    max,
    step = 1,
    placeholder = '',
    helpText = '',
    required = false,
    disabled = false,
    error,
    className = '',
    ...rest
  }: Props = $props();

  function handleChange(newValue: string | number | boolean | string[]) {
    // Explicitly handle different input types
    if (Array.isArray(newValue)) {
      return; // Arrays are not valid for number fields
    }

    if (typeof newValue === 'boolean') {
      return; // Booleans are not valid for number fields
    }

    // Convert to number with robust validation
    let numValue: number;
    if (typeof newValue === 'number') {
      numValue = newValue;
    } else {
      const stringValue = String(newValue).trim();
      if (stringValue === '' || stringValue === null || stringValue === undefined) {
        return; // Empty values should not update state
      }
      numValue = parseFloat(stringValue);
    }

    // Validate the parsed number and ensure it's within bounds if specified
    if (!isNaN(numValue) && isFinite(numValue)) {
      // Check min/max constraints if specified
      if (min !== undefined && numValue < min) {
        return; // Don't update if below minimum
      }
      if (max !== undefined && numValue > max) {
        return; // Don't update if above maximum
      }

      value = numValue;
      onUpdate(numValue);
    }
  }
</script>

<div class={className} {...rest}>
  <FormField
    type="number"
    name={label
      .toLowerCase()
      .replace(/\s+/g, '-')
      .replace(/[^a-z0-9-]/g, '')}
    {label}
    bind:value
    {min}
    {max}
    {step}
    {placeholder}
    {helpText}
    {required}
    {disabled}
    onChange={handleChange}
    inputClassName={error ? 'input-error' : ''}
  />

  {#if error}
    <div class="label">
      <span class="label-text-alt text-error">{error}</span>
    </div>
  {/if}
</div>

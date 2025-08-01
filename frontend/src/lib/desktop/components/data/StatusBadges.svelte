<!--
  StatusBadges.svelte
  
  A component that displays verification status badges for bird detection records.
  Provides visual indicators for detection verification states (correct, false positive, unverified).
  
  Usage:
  - Detection rows and tables
  - Detection detail views
  - Administrative detection management
  - Review interfaces
  
  Features:
  - Color-coded status badges
  - Handles multiple verification states
  - Consistent styling with design system
  - Responsive display
  - Size variants for different contexts
  
  Props:
  - detection: Detection - The detection object containing verification status
  - className?: string - Additional CSS classes
  - size?: 'sm' | 'md' | 'lg' - Badge size variant (default: 'md')
  
  Status Types:
  - correct: Green badge for verified correct detections
  - false_positive: Red badge for verified false positives
  - unverified: Gray badge for unverified detections
-->
<script lang="ts">
  import { cn } from '$lib/utils/cn';
  import type { Detection } from '$lib/types/detection.types';

  type Size = 'sm' | 'md' | 'lg';
  type VerificationStatus = Detection['verified'];

  interface Props {
    detection: Detection;
    className?: string;
    size?: Size;
  }

  let { detection, className = '', size = 'md' }: Props = $props();

  const sizeClasses: Record<Size, string> = {
    sm: 'status-badge-sm',
    md: 'status-badge-md',
    lg: 'status-badge-lg',
  };

  const statusBadgeClassMap: Partial<Record<VerificationStatus, string>> = {
    correct: 'status-badge correct',
    false_positive: 'status-badge false',
  };

  const statusTextMap: Partial<Record<VerificationStatus, string>> = {
    correct: 'correct',
    false_positive: 'false',
  };

  function getStatusBadgeClass(verified: VerificationStatus): string {
    const baseClass = statusBadgeClassMap[verified] || 'status-badge unverified';
    return `${baseClass} ${sizeClasses[size]}`;
  }

  function getStatusText(verified: VerificationStatus): string {
    return statusTextMap[verified] || 'unverified';
  }
</script>

<div class={cn('flex flex-wrap gap-1', className)}>
  <!-- Verification status badge -->
  <div class={getStatusBadgeClass(detection.verified)}>
    {getStatusText(detection.verified)}
  </div>

  <!-- Locked badge -->
  {#if detection.locked}
    <div class="status-badge locked {sizeClasses[size]}">locked</div>
  {/if}

  <!-- Comments badge -->
  {#if detection.comments && detection.comments.length > 0}
    <div class="status-badge comment {sizeClasses[size]}">comment</div>
  {/if}
</div>

<style>
  .status-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 0.25rem 0.75rem;
    border-radius: 9999px;
    font-size: 0.75rem;
    font-weight: 500;
    white-space: nowrap;
    border: 1px solid;
  }

  /* Size variants */
  .status-badge-sm {
    padding: 0.125rem 0.5rem;
    font-size: 0.625rem; /* 10px - smaller for compact contexts */
  }

  .status-badge-md {
    padding: 0.25rem 0.75rem;
    font-size: 0.75rem; /* 12px - default size */
  }

  .status-badge-lg {
    padding: 0.5rem 1rem;
    font-size: 0.875rem; /* 14px - larger for emphasis */
  }

  .status-badge.unverified {
    color: #6b7280;
    border-color: #6b7280;
    background-color: #f3f4f6;
  }

  .status-badge.correct {
    color: #059669;
    border-color: #059669;
    background-color: #ecfdf5;
  }

  .status-badge.false {
    color: #dc2626;
    border-color: #dc2626;
    background-color: #fef2f2;
  }

  .status-badge.locked {
    color: #d97706;
    border-color: #d97706;
    background-color: #fffbeb;
  }

  .status-badge.comment {
    color: #2563eb;
    border-color: #2563eb;
    background-color: #eff6ff;
  }

  /* Dark theme status badges */
  :global([data-theme='dark']) .status-badge.unverified {
    color: #9ca3af;
    border-color: #9ca3af;
    background-color: rgba(156, 163, 175, 0.1);
  }

  :global([data-theme='dark']) .status-badge.correct {
    color: #34d399;
    border-color: #34d399;
    background-color: rgba(52, 211, 153, 0.1);
  }

  :global([data-theme='dark']) .status-badge.false {
    color: #f87171;
    border-color: #f87171;
    background-color: rgba(248, 113, 113, 0.1);
  }

  :global([data-theme='dark']) .status-badge.locked {
    color: #fbbf24;
    border-color: #fbbf24;
    background-color: rgba(251, 191, 36, 0.1);
  }

  :global([data-theme='dark']) .status-badge.comment {
    color: #60a5fa;
    border-color: #60a5fa;
    background-color: rgba(96, 165, 250, 0.1);
  }
</style>

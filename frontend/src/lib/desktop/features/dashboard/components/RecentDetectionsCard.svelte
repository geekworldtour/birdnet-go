<script lang="ts">
  import AudioPlayer from '$lib/desktop/components/media/AudioPlayer.svelte';
  import ConfidenceCircle from '$lib/desktop/components/data/ConfidenceCircle.svelte';
  import ActionMenu from '$lib/desktop/components/ui/ActionMenu.svelte';
  import ReviewModal from '$lib/desktop/components/modals/ReviewModal.svelte';
  import ConfirmModal from '$lib/desktop/components/modals/ConfirmModal.svelte';
  import { fetchWithCSRF } from '$lib/utils/api';
  import type { Detection } from '$lib/types/detection.types';
  import { handleBirdImageError } from '$lib/desktop/components/ui/image-utils.js';
  import { actionIcons, alertIconsSvg } from '$lib/utils/icons';
  import { t } from '$lib/i18n';

  interface Props {
    data: Detection[];
    loading?: boolean;
    error?: string | null;
    onRowClick?: (_detection: Detection) => void;
    onRefresh: () => void;
    limit?: number;
    onLimitChange?: (limit: number) => void;
    newDetectionIds?: Set<number>;
    detectionArrivalTimes?: Map<number, number>;
  }

  let {
    data = [],
    loading = false,
    error = null,
    onRowClick,
    onRefresh,
    limit = 5,
    onLimitChange,
    newDetectionIds = new Set(),
    detectionArrivalTimes: _detectionArrivalTimes = new Map(), // Reserved for future staggered animations
  }: Props = $props();

  // State for number of detections to show
  let selectedLimit = $state(limit);

  // Update selectedLimit when prop changes
  $effect(() => {
    selectedLimit = limit;
  });

  // Updates the number of detections to display and persists the preference
  function handleLimitChange(newLimit: number) {
    selectedLimit = newLimit;

    // Save to localStorage
    if (typeof window !== 'undefined') {
      try {
        localStorage.setItem('recentDetectionLimit', newLimit.toString());
      } catch (e) {
        console.error('Failed to save detection limit:', e);
      }
    }

    // Call parent callback
    if (onLimitChange) {
      onLimitChange(newLimit);
    }
  }

  // Modal state for expanded audio player (removed - not currently used)

  // Handles clicking on a detection row to trigger parent callback
  function handleRowClick(detection: Detection) {
    if (onRowClick) {
      onRowClick(detection);
    }
  }

  // Returns appropriate badge configuration based on detection verification and lock status
  function getStatusBadge(verified: string, locked: boolean) {
    if (locked) {
      return {
        type: 'locked',
        text: t('dashboard.recentDetections.status.locked'),
        class: 'status-badge locked',
      };
    }

    switch (verified) {
      case 'correct':
        return {
          type: 'correct',
          text: t('dashboard.recentDetections.status.verified'),
          class: 'status-badge correct',
        };
      case 'false_positive':
        return {
          type: 'false',
          text: t('dashboard.recentDetections.status.false'),
          class: 'status-badge false',
        };
      default:
        return {
          type: 'unverified',
          text: t('dashboard.recentDetections.status.unverified'),
          class: 'status-badge unverified',
        };
    }
  }

  // Modal states
  let showReviewModal = $state(false);
  let showConfirmModal = $state(false);
  let selectedDetection = $state<Detection | null>(null);
  let confirmModalConfig = $state({
    title: '',
    message: '',
    confirmLabel: 'Confirm',
    onConfirm: async () => {},
  });

  // Action handlers
  // Opens the review modal for manual verification of a detection
  function handleReview(detection: Detection) {
    selectedDetection = detection;
    showReviewModal = true;
  }

  // Toggles whether a species should be ignored in future detections
  function handleToggleSpecies(detection: Detection) {
    const isExcluded = false; // TODO: determine if species is excluded
    confirmModalConfig = {
      title: isExcluded
        ? t('dashboard.recentDetections.modals.showSpecies', { species: detection.commonName })
        : t('dashboard.recentDetections.modals.ignoreSpecies', { species: detection.commonName }),
      message: isExcluded
        ? t('dashboard.recentDetections.modals.showSpeciesConfirm', {
            species: detection.commonName,
          })
        : t('dashboard.recentDetections.modals.ignoreSpeciesConfirm', {
            species: detection.commonName,
          }),
      confirmLabel: t('common.buttons.confirm'),
      onConfirm: async () => {
        try {
          await fetchWithCSRF('/api/v2/detections/ignore', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              common_name: detection.commonName,
            }),
          });
          onRefresh();
        } catch (error) {
          console.error('Error toggling species exclusion:', error);
        }
      },
    };
    selectedDetection = detection;
    showConfirmModal = true;
  }

  // Toggles the lock status of a detection to prevent/allow automatic cleanup
  function handleToggleLock(detection: Detection) {
    confirmModalConfig = {
      title: detection.locked
        ? t('dashboard.recentDetections.modals.unlockDetection')
        : t('dashboard.recentDetections.modals.lockDetection'),
      message: detection.locked
        ? t('dashboard.recentDetections.modals.unlockDetectionConfirm', {
            species: detection.commonName,
          })
        : t('dashboard.recentDetections.modals.lockDetectionConfirm', {
            species: detection.commonName,
          }),
      confirmLabel: t('common.buttons.confirm'),
      onConfirm: async () => {
        try {
          await fetchWithCSRF(`/api/v2/detections/${detection.id}/lock`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              locked: !detection.locked,
            }),
          });
          onRefresh();
        } catch (error) {
          console.error('Error toggling lock status:', error);
        }
      },
    };
    selectedDetection = detection;
    showConfirmModal = true;
  }

  // Permanently deletes a detection after user confirmation
  function handleDelete(detection: Detection) {
    confirmModalConfig = {
      title: t('dashboard.recentDetections.modals.deleteDetection', {
        species: detection.commonName,
      }),
      message: t('dashboard.recentDetections.modals.deleteDetectionConfirm', {
        species: detection.commonName,
      }),
      confirmLabel: t('common.buttons.delete'),
      onConfirm: async () => {
        try {
          await fetchWithCSRF(`/api/v2/detections/${detection.id}`, {
            method: 'DELETE',
          });
          onRefresh();
        } catch (error) {
          console.error('Error deleting detection:', error);
        }
      },
    };
    selectedDetection = detection;
    showConfirmModal = true;
  }
</script>

<section class="card col-span-12 bg-base-100 shadow-sm">
  <!-- Card Header -->
  <div class="card-body grow-0 p-2 sm:p-4 sm:pt-3">
    <div class="flex items-center justify-between mb-4">
      <div class="flex items-center gap-3">
        <span class="card-title grow text-base sm:text-xl"
          >{t('dashboard.recentDetections.title')}</span
        >
      </div>
      <div class="flex items-center gap-2">
        <label for="numDetections" class="label-text text-sm"
          >{t('dashboard.recentDetections.controls.show')}</label
        >
        <select
          id="numDetections"
          bind:value={selectedLimit}
          onchange={e => handleLimitChange(parseInt(e.currentTarget.value, 10))}
          class="select select-sm focus-visible:outline-none"
        >
          <option value={5}>5</option>
          <option value={10}>10</option>
          <option value={25}>25</option>
          <option value={50}>50</option>
        </select>
        <button
          onclick={onRefresh}
          class="btn btn-sm btn-ghost"
          disabled={loading}
          aria-label={t('dashboard.recentDetections.controls.refresh')}
        >
          <div class="h-4 w-4" class:animate-spin={loading}>
            {@html actionIcons.refresh}
          </div>
        </button>
      </div>
    </div>

    <!-- Content -->
    {#if loading}
      <div class="flex justify-center py-8">
        <span class="loading loading-spinner loading-md"></span>
      </div>
    {:else if error}
      <div class="alert alert-error">
        {@html alertIconsSvg.error}
        <span>{error}</span>
      </div>
    {:else}
      <!-- Desktop Layout -->
      <div>
        <!-- Header Row -->
        <div
          class="grid grid-cols-12 gap-4 text-xs font-medium text-base-content/70 pb-2 border-b border-base-300 px-4"
        >
          <div class="col-span-2">{t('dashboard.recentDetections.headers.dateTime')}</div>
          <div class="col-span-3">{t('dashboard.recentDetections.headers.species')}</div>
          <div class="col-span-1">{t('dashboard.recentDetections.headers.confidence')}</div>
          <div class="col-span-2">{t('dashboard.recentDetections.headers.status')}</div>
          <div class="col-span-3">{t('dashboard.recentDetections.headers.recording')}</div>
          <div class="col-span-1">{t('dashboard.recentDetections.headers.actions')}</div>
        </div>

        <!-- Detection Rows -->
        <div class="divide-y divide-base-200">
          {#each data.slice(0, selectedLimit) as detection}
            {@const badge = getStatusBadge(detection.verified, detection.locked)}
            {@const isNew = newDetectionIds.has(detection.id)}
            <div
              class="grid grid-cols-12 gap-4 items-center px-4 py-1 hover:bg-base-200/30 transition-colors detection-row"
              class:cursor-pointer={onRowClick}
              class:new-detection={isNew}
              style=""
              role="button"
              tabindex="0"
              onclick={() => handleRowClick(detection)}
              onkeydown={e =>
                e.key === 'Enter' || e.key === ' ' ? handleRowClick(detection) : null}
            >
              <!-- Date & Time -->
              <div class="col-span-2 text-sm">
                <div class="text-xs">{detection.date} {detection.time}</div>
              </div>

              <!-- Combined Species Column with Confidence -->
              <div class="col-span-3 rd-species-container">
                <!-- Thumbnail -->
                <div class="rd-thumbnail-wrapper">
                  <button
                    class="rd-thumbnail-button"
                    onclick={() => handleRowClick(detection)}
                    tabindex="-1"
                  >
                    <img
                      loading="lazy"
                      src="/api/v2/media/species-image?name={encodeURIComponent(
                        detection.scientificName
                      )}"
                      alt={detection.commonName}
                      class="rd-thumbnail-image"
                      onerror={handleBirdImageError}
                    />
                  </button>
                </div>
                
                <!-- Species Names and Confidence -->
                <div class="rd-species-info-wrapper">
                  <div class="rd-species-names">
                    <div class="rd-species-common-name">{detection.commonName}</div>
                    <div class="rd-species-scientific-name">{detection.scientificName}</div>
                  </div>
                </div>
              </div>

              <!-- Confidence -->
              <div class="col-span-1">
                <ConfidenceCircle confidence={detection.confidence} size="sm" />
              </div>

              <!-- Status -->
              <div class="col-span-2">
                <div class="flex flex-wrap gap-1">
                  <span class="status-badge {badge.class}">
                    {badge.text}
                  </span>
                </div>
              </div>

              <!-- Recording -->
              <div class="col-span-3">
                <div class="rd-audio-player-container relative min-w-[50px]">
                  <AudioPlayer
                    audioUrl="/api/v2/audio/{detection.id}"
                    detectionId={detection.id.toString()}
                    showSpectrogram={true}
                    className="w-full h-auto"
                  />
                </div>
              </div>

              <!-- Action Menu -->
              <div class="col-span-1 flex justify-end">
                <ActionMenu
                  {detection}
                  isExcluded={false}
                  onReview={() => handleReview(detection)}
                  onToggleSpecies={() => handleToggleSpecies(detection)}
                  onToggleLock={() => handleToggleLock(detection)}
                  onDelete={() => handleDelete(detection)}
                />
              </div>
            </div>
          {/each}
        </div>
      </div>

      {#if data.length === 0}
        <div class="text-center py-8 text-base-content/60">
          {t('dashboard.recentDetections.noDetections')}
        </div>
      {/if}
    {/if}
  </div>
</section>

<!-- Modals -->
{#if selectedDetection}
  <ReviewModal
    isOpen={showReviewModal}
    detection={selectedDetection}
    isExcluded={false}
    onClose={() => {
      showReviewModal = false;
      selectedDetection = null;
    }}
    onSave={async (verified, lockDetection, ignoreSpecies, comment) => {
      if (!selectedDetection) return;

      await fetchWithCSRF(`/api/v2/detections/${selectedDetection.id}/review`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          verified,
          lock_detection: lockDetection,
          ignore_species: ignoreSpecies ? selectedDetection.commonName : null,
          comment,
        }),
      });
      onRefresh();
      showReviewModal = false;
      selectedDetection = null;
    }}
  />

  <ConfirmModal
    isOpen={showConfirmModal}
    title={confirmModalConfig.title}
    message={confirmModalConfig.message}
    confirmLabel={confirmModalConfig.confirmLabel}
    onClose={() => {
      showConfirmModal = false;
      selectedDetection = null;
    }}
    onConfirm={async () => {
      await confirmModalConfig.onConfirm();
      showConfirmModal = false;
      selectedDetection = null;
    }}
  />
{/if}

<style>
  /* Use existing confidence circle styles from custom.css - no additional styles needed */

  /* RD prefix to avoid conflicts with global CSS */
  
  /* Species container layout */
  .rd-species-container {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0; /* Allow container to shrink */
  }

  /* Thumbnail wrapper - responsive width */
  .rd-thumbnail-wrapper {
    flex: 0 0 45%; /* Reduced to give more space to names */
    min-width: 40px; /* Minimum size on very small screens */
    max-width: 120px; /* Maximum size on large screens */
  }

  /* Thumbnail button - maintains aspect ratio */
  .rd-thumbnail-button {
    display: block;
    width: 100%;
    aspect-ratio: 4/3; /* Consistent aspect ratio */
    position: relative;
    overflow: hidden;
    border-radius: 0.375rem;
    background-color: hsl(var(--b2) / 0.3);
  }

  /* Thumbnail image */
  .rd-thumbnail-image {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
  }

  /* Species info wrapper - contains names and confidence */
  .rd-species-info-wrapper {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 0.375rem; /* Small gap between names and confidence */
    min-width: 0;
  }

  /* Species names container */
  .rd-species-names {
    flex: 1;
    min-width: 0; /* Allow text to shrink */
    text-align: left;
  }

  /* Common name - wraps instead of truncating */
  .rd-species-common-name {
    font-weight: 500;
    line-height: 1.2;
    word-wrap: break-word;
    overflow-wrap: break-word;
    hyphens: auto;
    cursor: pointer;
  }
  
  .rd-species-common-name:hover {
    color: hsl(var(--p));
  }

  /* Scientific name - smaller, can wrap on very narrow screens */
  .rd-species-scientific-name {
    font-size: 0.75rem;
    line-height: 1.2;
    color: hsl(var(--bc) / 0.6);
    word-wrap: break-word;
    overflow-wrap: break-word;
    hyphens: auto;
  }

  /* Confidence indicator - stays close to species names */
  .rd-confidence-indicator {
    flex: 0 0 auto;
  }

  /* Responsive adjustments */
  @media (max-width: 768px) {
    .rd-species-container {
      gap: 0.375rem;
    }
    
    .rd-thumbnail-wrapper {
      flex: 0 0 35%; /* Slightly larger on mobile */
    }
    
    .rd-species-common-name {
      font-size: 0.875rem;
    }
    
    .rd-species-scientific-name {
      font-size: 0.7rem;
    }
    
    .rd-species-info-wrapper {
      gap: 0.25rem;
    }
  }

  /* RD Audio Player Container */
  .rd-audio-player-container {
    position: relative;
    width: 100%;
    background: linear-gradient(to bottom, rgba(128, 128, 128, 0.4), rgba(128, 128, 128, 0.1));
    border-radius: 0.5rem;
  }
  
  /* Audio player skeleton (before content loads) */
  .rd-audio-player-container::before {
    content: "";
    width: 1px;
    margin-left: -1px;
    float: left;
    height: 0;
    padding-top: 50%; /* Maintains a 2:1 ratio */
  }
  
  .rd-audio-player-container::after {
    content: "";
    display: table;
    clear: both;
  }

  /* Ensure AudioPlayer fills container width */
  .rd-audio-player-container :global(.group) {
    width: 100% !important;
    height: auto !important;
  }

  /* Responsive spectrogram sizing - let it maintain natural aspect ratio */
  .rd-audio-player-container :global(img) {
    object-fit: contain !important;
    height: auto !important;
    width: 100% !important;
    max-width: 400px;
  }

  /* Grid alignment - items-center is handled by Tailwind class */

  /* Detection row theme-aware styling with hover effects */
  .detection-row {
    border-bottom: 1px solid hsl(var(--bc) / 0.1);
    transition:
      transform 0.3s ease-out,
      background-color 0.15s ease-in-out;
  }

  /* New detection animations - theme-aware fade-in */
  .new-detection {
    animation: slideInFade 0.8s cubic-bezier(0.25, 0.46, 0.45, 0.94) both;
  }

  @keyframes slideInFade {
    0% {
      transform: translateY(-30px);
      opacity: 0;
      background-color: hsl(var(--p) / 0.2);
      border-left: 4px solid hsl(var(--p));
    }

    50% {
      background-color: hsl(var(--p) / 0.15);
      border-left: 4px solid hsl(var(--p));
    }

    100% {
      transform: translateY(0);
      opacity: 1;
      background-color: transparent;
      border-left: none;
    }
  }

  /* Smooth transitions handled above in .detection-row */
</style>

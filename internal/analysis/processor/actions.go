// processor/actions.go

package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tphakala/birdnet-go/internal/analysis/jobqueue"
	"github.com/tphakala/birdnet-go/internal/birdnet"
	"github.com/tphakala/birdnet-go/internal/birdweather"
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/datastore"
	"github.com/tphakala/birdnet-go/internal/events"
	"github.com/tphakala/birdnet-go/internal/imageprovider"
	"github.com/tphakala/birdnet-go/internal/mqtt"
	"github.com/tphakala/birdnet-go/internal/myaudio"
	"github.com/tphakala/birdnet-go/internal/notification"
	"github.com/tphakala/birdnet-go/internal/observation"
)

type Action interface {
	Execute(data interface{}) error
	GetDescription() string
}

type LogAction struct {
	Settings     *conf.Settings
	Note         datastore.Note
	EventTracker *EventTracker
	Description  string
	mu           sync.Mutex // Protect concurrent access to Note
}

type DatabaseAction struct {
	Settings          *conf.Settings
	Ds                datastore.Interface
	Note              datastore.Note
	Results           []datastore.Results
	EventTracker      *EventTracker
	NewSpeciesTracker *NewSpeciesTracker // Add reference to new species tracker
	Description       string
	mu                sync.Mutex // Protect concurrent access to Note and Results
}

type SaveAudioAction struct {
	Settings     *conf.Settings
	ClipName     string
	pcmData      []byte
	EventTracker *EventTracker
	Description  string
	mu           sync.Mutex // Protect concurrent access to pcmData
}

type BirdWeatherAction struct {
	Settings     *conf.Settings
	Note         datastore.Note
	pcmData      []byte
	BwClient     *birdweather.BwClient
	EventTracker *EventTracker
	RetryConfig  jobqueue.RetryConfig // Configuration for retry behavior
	Description  string
	mu           sync.Mutex // Protect concurrent access to Note and pcmData
}

type MqttAction struct {
	Settings       *conf.Settings
	Note           datastore.Note
	BirdImageCache *imageprovider.BirdImageCache
	MqttClient     mqtt.Client
	EventTracker   *EventTracker
	RetryConfig    jobqueue.RetryConfig // Configuration for retry behavior
	Description    string
	mu             sync.Mutex // Protect concurrent access to Note
}

type UpdateRangeFilterAction struct {
	Bn          *birdnet.BirdNET
	Settings    *conf.Settings
	Description string
	mu          sync.Mutex // Protect concurrent access to Settings
}

type SSEAction struct {
	Settings       *conf.Settings
	Note           datastore.Note
	BirdImageCache *imageprovider.BirdImageCache
	EventTracker   *EventTracker
	RetryConfig    jobqueue.RetryConfig // Configuration for retry behavior
	Description    string
	mu             sync.Mutex // Protect concurrent access to Note
	// SSEBroadcaster is a function that broadcasts detection data
	// This allows the action to be independent of the specific API implementation
	SSEBroadcaster func(note *datastore.Note, birdImage *imageprovider.BirdImage) error
	// Datastore interface for querying the database to get the assigned ID
	Ds datastore.Interface
}

// GetDescription returns a human-readable description of the LogAction
func (a *LogAction) GetDescription() string {
	if a.Description != "" {
		return a.Description
	}
	return "Log bird detection to file"
}

// GetDescription returns a human-readable description of the DatabaseAction
func (a *DatabaseAction) GetDescription() string {
	if a.Description != "" {
		return a.Description
	}
	return "Save bird detection to database"
}

// GetDescription returns a human-readable description of the SaveAudioAction
func (a *SaveAudioAction) GetDescription() string {
	if a.Description != "" {
		return a.Description
	}
	return "Save audio clip to file"
}

// GetDescription returns a human-readable description of the BirdWeatherAction
func (a *BirdWeatherAction) GetDescription() string {
	if a.Description != "" {
		return a.Description
	}
	return "Upload detection to BirdWeather"
}

// GetDescription returns a human-readable description of the MqttAction
func (a *MqttAction) GetDescription() string {
	if a.Description != "" {
		return a.Description
	}
	return "Publish detection to MQTT"
}

// GetDescription returns a human-readable description of the UpdateRangeFilterAction
func (a *UpdateRangeFilterAction) GetDescription() string {
	if a.Description != "" {
		return a.Description
	}
	return "Update BirdNET range filter"
}

// GetDescription returns a human-readable description of the SSEAction
func (a *SSEAction) GetDescription() string {
	if a.Description != "" {
		return a.Description
	}
	return "Broadcast detection via Server-Sent Events"
}

// Execute logs the note to the chag log file
func (a *LogAction) Execute(data interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	species := strings.ToLower(a.Note.CommonName)

	// Check if the event should be handled for this species
	if !a.EventTracker.TrackEvent(species, LogToFile) {
		return nil
	}

	// Log note to file
	if err := observation.LogNoteToFile(a.Settings, &a.Note); err != nil {
		// If an error occurs when logging to a file, wrap and return the error.
		log.Printf("❌ Failed to log note to file: %v", err)
	}
	fmt.Printf("%s %s %.2f\n", a.Note.Time, a.Note.CommonName, a.Note.Confidence)

	return nil
}

// Execute saves the note to the database
func (a *DatabaseAction) Execute(data interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	species := strings.ToLower(a.Note.CommonName)

	// Check event frequency
	if !a.EventTracker.TrackEvent(species, DatabaseSave) {
		return nil
	}

	// Check if this is a new species and update atomically to prevent race conditions
	var isNewSpecies bool
	var daysSinceFirstSeen int
	if a.NewSpeciesTracker != nil {
		// Use atomic check-and-update to prevent duplicate "new species" notifications
		// when multiple detections of the same species arrive concurrently
		isNewSpecies, daysSinceFirstSeen = a.NewSpeciesTracker.CheckAndUpdateSpecies(a.Note.ScientificName, time.Now())
	}
	
	// Save note to database
	if err := a.Ds.Save(&a.Note, a.Results); err != nil {
		log.Printf("❌ Failed to save note and results to database: %v", err)
		return err
	}
	
	// After successful save, publish detection event for new species
	a.publishNewSpeciesDetectionEvent(isNewSpecies, daysSinceFirstSeen)

	// Save audio clip to file if enabled
	if a.Settings.Realtime.Audio.Export.Enabled {
		// export audio clip from capture buffer
		pcmData, err := myaudio.ReadSegmentFromCaptureBuffer(a.Note.Source, a.Note.BeginTime, 15)
		if err != nil {
			log.Printf("❌ Failed to read audio segment from buffer: %v", err)
			return err
		}

		// Create a SaveAudioAction and execute it
		saveAudioAction := &SaveAudioAction{
			Settings: a.Settings,
			ClipName: a.Note.ClipName,
			pcmData:  pcmData,
		}

		if err := saveAudioAction.Execute(nil); err != nil {
			log.Printf("❌ Failed to save audio clip: %v", err)
			return err
		}

		if a.Settings.Debug {
			log.Printf("✅ Saved audio clip to %s\n", a.Note.ClipName)
			log.Printf("detection time %v, begin time %v, end time %v\n", a.Note.Time, a.Note.BeginTime, time.Now())
		}
	}

	return nil
}

// publishNewSpeciesDetectionEvent publishes a detection event for new species
// This helper method handles event bus retrieval, event creation, publishing, and debug logging
func (a *DatabaseAction) publishNewSpeciesDetectionEvent(isNewSpecies bool, daysSinceFirstSeen int) {
	if !isNewSpecies || !events.IsInitialized() {
		return
	}

	eventBus := events.GetEventBus()
	if eventBus == nil {
		return
	}

	detectionEvent, err := events.NewDetectionEvent(
		a.Note.CommonName,
		a.Note.ScientificName,
		float64(a.Note.Confidence),
		a.Note.Source,
		isNewSpecies,
		daysSinceFirstSeen,
	)
	if err != nil {
		if a.Settings.Debug {
			log.Printf("❌ Failed to create detection event: %v", err)
		}
		return
	}

	// Publish the detection event
	if published := eventBus.TryPublishDetection(detectionEvent); published {
		if a.Settings.Debug {
			log.Printf("🌟 Published new species detection event: %s", a.Note.CommonName)
		}
	}
}

// Execute saves the audio clip to a file
func (a *SaveAudioAction) Execute(data interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Get the full path by joining the export path with the relative clip name
	outputPath := filepath.Join(a.Settings.Realtime.Audio.Export.Path, a.ClipName)

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		log.Printf("❌ error creating directory for audio clip: %s\n", err)
		return err
	}

	if a.Settings.Realtime.Audio.Export.Type == "wav" {
		if err := myaudio.SavePCMDataToWAV(outputPath, a.pcmData); err != nil {
			log.Printf("❌ error saving audio clip to WAV: %s\n", err)
			return err
		}
	} else {
		if err := myaudio.ExportAudioWithFFmpeg(a.pcmData, outputPath, &a.Settings.Realtime.Audio); err != nil {
			log.Printf("❌ error exporting audio clip with FFmpeg: %s\n", err)
			return err
		}
	}

	return nil
}

// Execute sends the note to the BirdWeather API
func (a *BirdWeatherAction) Execute(data interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	species := strings.ToLower(a.Note.CommonName)

	// Check event frequency
	if !a.EventTracker.TrackEvent(species, BirdWeatherSubmit) {
		return nil
	}

	// Early check if BirdWeather is still enabled in settings
	if !a.Settings.Realtime.Birdweather.Enabled {
		return nil // Silently exit if BirdWeather was disabled after this action was created
	}

	// Add threshold check here
	if a.Note.Confidence < float64(a.Settings.Realtime.Birdweather.Threshold) {
		if a.Settings.Debug {
			log.Printf("⛔ Skipping BirdWeather upload for %s: confidence %.2f below threshold %.2f\n",
				species, a.Note.Confidence, a.Settings.Realtime.Birdweather.Threshold)
		}
		return nil
	}

	// Safe check for nil BwClient
	if a.BwClient == nil {
		return fmt.Errorf("BirdWeather client is not initialized")
	}

	// Copy data locally to reduce lock duration if needed
	note := a.Note
	pcmData := a.pcmData

	// Try to publish with appropriate error handling
	if err := a.BwClient.Publish(&note, pcmData); err != nil {
		// Log the error with retry information if retries are enabled
		// Sanitize error before logging
		sanitizedErr := sanitizeError(err)
		if a.RetryConfig.Enabled {
			log.Printf("❌ Error uploading %s (%s) to BirdWeather (confidence: %.2f, clip: %s) (will retry): %v\n",
				note.CommonName, note.ScientificName, note.Confidence, note.ClipName, sanitizedErr)
		} else {
			log.Printf("❌ Error uploading %s (%s) to BirdWeather (confidence: %.2f, clip: %s): %v\n",
				note.CommonName, note.ScientificName, note.Confidence, note.ClipName, sanitizedErr)
			// Send notification for non-retryable failures
			notification.NotifyIntegrationFailure("BirdWeather", err)
		}
		return fmt.Errorf("failed to upload %s to BirdWeather: %w", note.CommonName, err) // Return wrapped error with context
	}

	if a.Settings.Debug {
		log.Printf("✅ Successfully uploaded %s to BirdWeather\n", a.Note.ClipName)
	}
	return nil
}

type NoteWithBirdImage struct {
	datastore.Note
	BirdImage imageprovider.BirdImage
}

// Execute sends the note to the MQTT broker
func (a *MqttAction) Execute(data interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Rely on background reconnect; fail action if not currently connected.
	if !a.MqttClient.IsConnected() {
		// Log slightly differently to indicate it's waiting for background reconnect
		log.Printf("🟡 MQTT client is not connected, skipping publish for %s (%s). Waiting for automatic reconnect.", a.Note.CommonName, a.Note.ScientificName)
		// Return an error that indicates a retry might be useful later
		// The job queue should handle retries based on this error.
		return fmt.Errorf("MQTT client not connected")
	}

	species := strings.ToLower(a.Note.CommonName)

	// Check event frequency
	if !a.EventTracker.TrackEvent(species, MQTTPublish) {
		return nil
	}

	// Validate MQTT settings
	if a.Settings.Realtime.MQTT.Topic == "" {
		return fmt.Errorf("MQTT topic is not specified")
	}

	// Get bird image of detected bird
	birdImage := imageprovider.BirdImage{} // Default to empty image
	// Add nil check for BirdImageCache before calling Get
	if a.BirdImageCache != nil {
		var err error
		birdImage, err = a.BirdImageCache.Get(a.Note.ScientificName)
		if err != nil {
			log.Printf("⚠️ Error getting bird image from cache for %s: %v", a.Note.ScientificName, err)
			// Continue with the default empty image
		}
	} else {
		// Log if the cache is nil, maybe helpful for debugging setup issues
		log.Printf("🟡 BirdImageCache is nil, cannot fetch image for %s", a.Note.ScientificName)
	}

	// Create a copy of the Note with sanitized RTSP URL
	noteCopy := a.Note
	noteCopy.Source = conf.SanitizeRTSPUrl(noteCopy.Source)

	// Wrap note with bird image
	noteWithBirdImage := NoteWithBirdImage{Note: a.Note, BirdImage: birdImage}

	// Create a JSON representation of the note
	noteJson, err := json.Marshal(noteWithBirdImage)
	if err != nil {
		log.Printf("❌ Error marshalling note to JSON: %s\n", err)
		return err
	}

	// Create a context with timeout for publishing
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Publish the note to the MQTT broker
	err = a.MqttClient.Publish(ctx, a.Settings.Realtime.MQTT.Topic, string(noteJson))
	if err != nil {
		// Log the error with retry information if retries are enabled
		// Sanitize error before logging
		sanitizedErr := sanitizeError(err)
		if a.RetryConfig.Enabled {
			log.Printf("❌ Error publishing %s (%s) to MQTT topic %s (confidence: %.2f, clip: %s) (will retry): %v\n",
				a.Note.CommonName, a.Note.ScientificName, a.Settings.Realtime.MQTT.Topic, a.Note.Confidence, a.Note.ClipName, sanitizedErr)
		} else {
			log.Printf("❌ Error publishing %s (%s) to MQTT topic %s (confidence: %.2f, clip: %s): %v\n",
				a.Note.CommonName, a.Note.ScientificName, a.Settings.Realtime.MQTT.Topic, a.Note.Confidence, a.Note.ClipName, sanitizedErr)
			// Send notification for non-retryable failures
			notification.NotifyIntegrationFailure("MQTT", err)
		}
		return fmt.Errorf("failed to publish %s to MQTT topic %s: %w", a.Note.CommonName, a.Settings.Realtime.MQTT.Topic, err)
	}

	if a.Settings.Debug {
		log.Printf("✅ Successfully published %s to MQTT topic %s\n",
			a.Note.CommonName, a.Settings.Realtime.MQTT.Topic)
	}
	return nil
}

// Execute updates the range filter species list, this is run every day
func (a *UpdateRangeFilterAction) Execute(data interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	today := time.Now().Truncate(24 * time.Hour)
	if today.After(a.Settings.BirdNET.RangeFilter.LastUpdated) {
		// Update location based species list
		speciesScores, err := a.Bn.GetProbableSpecies(today, 0.0)
		if err != nil {
			return err
		}

		// Convert the speciesScores slice to a slice of species labels
		var includedSpecies []string
		for _, speciesScore := range speciesScores {
			includedSpecies = append(includedSpecies, speciesScore.Label)
		}

		a.Settings.UpdateIncludedSpecies(includedSpecies)
	}
	return nil
}

// Execute broadcasts the detection via Server-Sent Events
func (a *SSEAction) Execute(data interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if SSE broadcaster is available
	if a.SSEBroadcaster == nil {
		return nil // Silently skip if no broadcaster is configured
	}

	species := strings.ToLower(a.Note.CommonName)

	// Check event frequency
	if !a.EventTracker.TrackEvent(species, SSEBroadcast) {
		return nil
	}

	// Wait for audio file to be available if this detection has an audio clip assigned
	// This properly handles per-species audio settings and avoids false positives
	if a.Note.ClipName != "" {
		if err := a.waitForAudioFile(); err != nil {
			// Log warning but don't fail the SSE broadcast
			log.Printf("⚠️ Audio file not ready for %s, broadcasting without waiting: %v", a.Note.CommonName, err)
		}
	}

	// Wait for database ID to be assigned if Note.ID is 0 (new detection)
	// This ensures the frontend can properly load audio/spectrogram via API endpoints
	if a.Note.ID == 0 {
		if err := a.waitForDatabaseID(); err != nil {
			// Log warning but don't fail the SSE broadcast
			log.Printf("⚠️ Database ID not ready for %s, broadcasting with ID=0: %v", a.Note.CommonName, err)
		}
	}

	// Get bird image of detected bird
	birdImage := imageprovider.BirdImage{} // Default to empty image
	// Add nil check for BirdImageCache before calling Get
	if a.BirdImageCache != nil {
		var err error
		birdImage, err = a.BirdImageCache.Get(a.Note.ScientificName)
		if err != nil {
			log.Printf("⚠️ Error getting bird image from cache for %s: %v", a.Note.ScientificName, err)
			// Continue with the default empty image
		}
	} else {
		// Log if the cache is nil, maybe helpful for debugging setup issues
		log.Printf("🟡 BirdImageCache is nil, cannot fetch image for %s", a.Note.ScientificName)
	}

	// Create a copy of the Note with sanitized RTSP URL
	noteCopy := a.Note
	noteCopy.Source = conf.SanitizeRTSPUrl(noteCopy.Source)

	// Broadcast the detection with error handling
	if err := a.SSEBroadcaster(&noteCopy, &birdImage); err != nil {
		// Log the error with retry information if retries are enabled
		// Sanitize error before logging
		sanitizedErr := sanitizeError(err)
		if a.RetryConfig.Enabled {
			log.Printf("❌ Error broadcasting %s (%s) via SSE (confidence: %.2f, clip: %s) (will retry): %v\n",
				a.Note.CommonName, a.Note.ScientificName, a.Note.Confidence, a.Note.ClipName, sanitizedErr)
		} else {
			log.Printf("❌ Error broadcasting %s (%s) via SSE (confidence: %.2f, clip: %s): %v\n",
				a.Note.CommonName, a.Note.ScientificName, a.Note.Confidence, a.Note.ClipName, sanitizedErr)
		}
		return fmt.Errorf("failed to broadcast %s via SSE: %w", a.Note.CommonName, err)
	}

	if a.Settings.Debug {
		log.Printf("✅ Successfully broadcasted %s via SSE\n", a.Note.CommonName)
	}

	return nil
}

// waitForAudioFile waits for the audio file to be written to disk with a timeout
func (a *SSEAction) waitForAudioFile() error {
	if a.Note.ClipName == "" {
		return nil // No audio file expected
	}

	// Build the full path to the audio file using the configured export path
	audioPath := filepath.Join(a.Settings.Realtime.Audio.Export.Path, a.Note.ClipName)
	
	// Wait up to 5 seconds for file to be written
	timeout := 5 * time.Second
	deadline := time.Now().Add(timeout)
	checkInterval := 100 * time.Millisecond

	for time.Now().Before(deadline) {
		// Check if file exists and has content
		if info, err := os.Stat(audioPath); err == nil {
			// File exists, check if it has reasonable size (audio files should be > 1KB)
			if info.Size() > 1024 {
				if a.Settings.Debug {
					log.Printf("🎵 Audio file ready for SSE broadcast: %s (size: %d bytes)", a.Note.ClipName, info.Size())
				}
				return nil
			}
			// File exists but might still be writing, wait a bit more
		}
		
		time.Sleep(checkInterval)
	}

	// Timeout reached
	return fmt.Errorf("audio file %s not ready after %v timeout", a.Note.ClipName, timeout)
}

// waitForDatabaseID waits for the Note to be saved to database and ID assigned
func (a *SSEAction) waitForDatabaseID() error {
	// We need to query the database to find this note by unique characteristics
	// Since we don't have the ID yet, we'll search by time, species, and confidence
	timeout := 10 * time.Second
	deadline := time.Now().Add(timeout)
	checkInterval := 200 * time.Millisecond

	for time.Now().Before(deadline) {
		// Query database for a note matching our characteristics
		// Use a small time window around the detection time to find the record
		if updatedNote, err := a.findNoteInDatabase(); err == nil && updatedNote.ID > 0 {
			// Found the note with an ID, update our copy
			a.Note.ID = updatedNote.ID
			if a.Settings.Debug {
				log.Printf("🔍 Found database ID %d for SSE broadcast: %s", updatedNote.ID, a.Note.CommonName)
			}
			return nil
		}
		
		time.Sleep(checkInterval)
	}

	// Timeout reached
	return fmt.Errorf("database ID not assigned for %s after %v timeout", a.Note.CommonName, timeout)
}

// findNoteInDatabase searches for the note in database by unique characteristics
func (a *SSEAction) findNoteInDatabase() (*datastore.Note, error) {
	if a.Ds == nil {
		return nil, fmt.Errorf("datastore not available")
	}

	// Search for notes with matching characteristics
	// The SearchNotes method expects a search query string that will match against
	// common_name or scientific_name fields
	query := a.Note.ScientificName
	
	// Search for notes, sorted by ID descending to get the most recent
	notes, err := a.Ds.SearchNotes(query, false, 10, 0) // false = sort descending, limit 10, offset 0
	if err != nil {
		return nil, fmt.Errorf("failed to search for note: %w", err)
	}

	// Filter results to find the exact match based on date and time
	for i := range notes {
		note := &notes[i]
		// Check if this note matches our expected characteristics
		if note.Date == a.Note.Date && 
		   note.ScientificName == a.Note.ScientificName &&
		   note.Time == a.Note.Time { // Exact time match
			return note, nil
		}
	}

	return nil, fmt.Errorf("note not found in database")
}

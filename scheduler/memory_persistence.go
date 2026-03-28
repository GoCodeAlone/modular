package scheduler

import (
	"encoding/json"
	"fmt"
	"sync"
)

// MemoryPersistenceHandler implements PersistenceHandler using in-memory storage
// This is useful for testing and scenarios where file system persistence isn't needed
type MemoryPersistenceHandler struct {
	mu   sync.RWMutex
	data []byte
}

// NewMemoryPersistenceHandler creates a new memory-based persistence handler
func NewMemoryPersistenceHandler() *MemoryPersistenceHandler {
	return &MemoryPersistenceHandler{}
}

// Save persists jobs to memory storage
func (h *MemoryPersistenceHandler) Save(jobs []Job) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create data structure for persistence
	persistedData := struct {
		Jobs []Job `json:"jobs"`
	}{
		Jobs: make([]Job, len(jobs)),
	}

	// Copy jobs without JobFunc (can't be serialized)
	for i, job := range jobs {
		jobCopy := job
		jobCopy.JobFunc = nil
		persistedData.Jobs[i] = jobCopy
	}

	// Marshal to JSON
	data, err := json.Marshal(persistedData)
	if err != nil {
		return fmt.Errorf("failed to marshal scheduler jobs to JSON: %w", err)
	}

	h.data = data
	return nil
}

// Load retrieves jobs from memory storage
func (h *MemoryPersistenceHandler) Load() ([]Job, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.data) == 0 {
		return nil, nil
	}

	// Parse the JSON
	var persistedData struct {
		Jobs []Job `json:"jobs"`
	}

	if err := json.Unmarshal(h.data, &persistedData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scheduler jobs from JSON: %w", err)
	}

	// Clear JobFunc from loaded jobs (will be reinitialized when job is resumed)
	for i := range persistedData.Jobs {
		persistedData.Jobs[i].JobFunc = nil
	}

	return persistedData.Jobs, nil
}

// GetStoredData returns the raw stored data for inspection (testing purposes)
func (h *MemoryPersistenceHandler) GetStoredData() []byte {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.data) == 0 {
		return nil
	}

	result := make([]byte, len(h.data))
	copy(result, h.data)
	return result
}

// Clear removes all stored data
func (h *MemoryPersistenceHandler) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.data = nil
}

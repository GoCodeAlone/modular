package modular

import (
	"github.com/GoCodeAlone/modular/feeders"
)

// FieldTrackerBridge adapts between the main package's FieldTracker interface
// and the feeders package's FieldTracker interface
type FieldTrackerBridge struct {
	mainTracker FieldTracker
}

// NewFieldTrackerBridge creates a new bridge adapter
func NewFieldTrackerBridge(mainTracker FieldTracker) *FieldTrackerBridge {
	return &FieldTrackerBridge{
		mainTracker: mainTracker,
	}
}

// RecordFieldPopulation implements the feeders.FieldTracker interface
// by converting feeders.FieldPopulation to the main package's FieldPopulation
func (b *FieldTrackerBridge) RecordFieldPopulation(fp feeders.FieldPopulation) {
	// Convert from feeders.FieldPopulation to main package FieldPopulation
	mainFP := FieldPopulation{
		FieldPath:   fp.FieldPath,
		FieldName:   fp.FieldName,
		FieldType:   fp.FieldType,
		FeederType:  fp.FeederType,
		SourceType:  fp.SourceType,
		SourceKey:   fp.SourceKey,
		Value:       fp.Value,
		InstanceKey: fp.InstanceKey,
		SearchKeys:  fp.SearchKeys,
		FoundKey:    fp.FoundKey,
	}

	// Record to the main tracker
	b.mainTracker.RecordFieldPopulation(mainFP)
}

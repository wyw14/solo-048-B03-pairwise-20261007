package protection

import (
	"fmt"
	"sync"
	"time"

	"heatpulse/internal/store"
)

type Latches struct {
	StationID    string    `json:"station_id"`
	Revision     uint64    `json:"revision"`
	HighReturn   bool      `json:"high_return"`
	FreezeRisk   bool      `json:"freeze_risk"`
	PressureTrip bool      `json:"pressure_trip"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type LatchStore struct {
	snapshots *store.StationSnapshotStore
	mu        sync.RWMutex
	latches   map[string]Latches
}

func NewLatchStore(snapshots *store.StationSnapshotStore) *LatchStore {
	return &LatchStore{snapshots: snapshots, latches: make(map[string]Latches)}
}

func (s *LatchStore) Apply(candidate Latches) (Latches, error) {
	if candidate.StationID == "" || candidate.Revision == 0 {
		return Latches{}, fmt.Errorf("latch identity and revision are required")
	}
	candidate.UpdatedAt = time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.latches[candidate.StationID]
	if candidate.Revision < current.Revision {
		return current, fmt.Errorf("stale latch revision %d", candidate.Revision)
	}

	// Persist first and only commit into memory once the snapshot write has
	// succeeded. The lock stays held for the whole commit, so a failed write
	// leaves both Get and any retry observing the previous protection state;
	// retrying the same revision is an idempotent replay in the snapshot store.
	merged, err := s.snapshots.MergeProtection(candidate.StationID, store.ProtectionSnapshot{
		Revision: candidate.Revision, HighReturn: candidate.HighReturn,
		FreezeRisk: candidate.FreezeRisk, PressureTrip: candidate.PressureTrip,
		UpdatedAt: candidate.UpdatedAt,
	})
	if err != nil {
		return current, err
	}
	committed := Latches{
		StationID: candidate.StationID, Revision: merged.Protection.Revision,
		HighReturn: merged.Protection.HighReturn, FreezeRisk: merged.Protection.FreezeRisk,
		PressureTrip: merged.Protection.PressureTrip, UpdatedAt: merged.Protection.UpdatedAt,
	}
	if committed.Revision >= s.latches[candidate.StationID].Revision {
		s.latches[candidate.StationID] = committed
	}
	return committed, nil
}

func (s *LatchStore) Get(stationID string) Latches {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latches[stationID]
}

func (s *LatchStore) Reset(stationID string, revision uint64) (Latches, error) {
	return s.Apply(Latches{StationID: stationID, Revision: revision})
}

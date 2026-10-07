package store

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"
)

type ProtectionSnapshot struct {
	Revision     uint64    `json:"revision"`
	HighReturn   bool      `json:"high_return"`
	FreezeRisk   bool      `json:"freeze_risk"`
	PressureTrip bool      `json:"pressure_trip"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type StationSnapshot struct {
	StationID       string             `json:"station_id"`
	Online          bool               `json:"online"`
	Maintenance     bool               `json:"maintenance"`
	TargetCelsius   float64            `json:"target_celsius"`
	ControllerEpoch uint64             `json:"controller_epoch"`
	Protection      ProtectionSnapshot `json:"protection"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

type StationSnapshotStore struct {
	file *File[map[string]StationSnapshot]
}

func NewStationSnapshotStore(directory string) *StationSnapshotStore {
	return &StationSnapshotStore{file: NewFile[map[string]StationSnapshot](filepath.Join(directory, "stations.json"))}
}

func (s *StationSnapshotStore) Put(snapshot StationSnapshot) error {
	if snapshot.StationID == "" {
		return fmt.Errorf("station id is required")
	}
	snapshot.UpdatedAt = time.Now().UTC()
	_, err := s.file.Update(func(all *map[string]StationSnapshot) error {
		if *all == nil {
			*all = make(map[string]StationSnapshot)
		}
		(*all)[snapshot.StationID] = snapshot
		return nil
	})
	return err
}

func (s *StationSnapshotStore) Get(stationID string) (StationSnapshot, bool, error) {
	all, err := s.file.Load()
	if err != nil {
		return StationSnapshot{}, false, err
	}
	snapshot, ok := all[stationID]
	return snapshot, ok, nil
}

func (s *StationSnapshotStore) List() ([]StationSnapshot, error) {
	all, err := s.file.Load()
	if err != nil {
		return nil, err
	}
	result := make([]StationSnapshot, 0, len(all))
	for _, snapshot := range all {
		result = append(result, snapshot)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StationID < result[j].StationID })
	return result, nil
}

func (s *StationSnapshotStore) MergeProtection(stationID string, candidate ProtectionSnapshot) (StationSnapshot, error) {
	var result StationSnapshot
	_, err := s.file.Update(func(all *map[string]StationSnapshot) error {
		if *all == nil {
			*all = make(map[string]StationSnapshot)
		}
		current := (*all)[stationID]
		if candidate.Revision < current.Protection.Revision {
			return fmt.Errorf("stale protection revision %d", candidate.Revision)
		}
		current.StationID = stationID
		current.Protection = candidate
		current.UpdatedAt = time.Now().UTC()
		(*all)[stationID] = current
		result = current
		return nil
	})
	return result, err
}

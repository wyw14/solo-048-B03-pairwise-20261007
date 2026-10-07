package protection

import (
	"os"
	"path/filepath"
	"testing"

	"heatpulse/internal/store"
)

// brokenSnapshotStore returns a store whose directory path is occupied by a
// regular file, so every persistence attempt fails.
func brokenSnapshotStore(t *testing.T) *store.StationSnapshotStore {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocker, []byte("occupies the directory path"), 0o600); err != nil {
		t.Fatalf("create blocker: %v", err)
	}
	return store.NewStationSnapshotStore(blocker)
}

func TestApplyPersistFailureKeepsPreviousState(t *testing.T) {
	s := NewLatchStore(store.NewStationSnapshotStore(t.TempDir()))
	if _, err := s.Apply(Latches{StationID: "st-1", Revision: 3, FreezeRisk: true}); err != nil {
		t.Fatalf("seed apply: %v", err)
	}

	s.snapshots = brokenSnapshotStore(t)
	if _, err := s.Apply(Latches{StationID: "st-1", Revision: 4, PressureTrip: true}); err == nil {
		t.Fatal("expected persist error")
	}

	got := s.Get("st-1")
	if got.Revision != 3 || !got.FreezeRisk || got.PressureTrip {
		t.Fatalf("failed apply changed in-memory state: %+v", got)
	}
}

func TestApplyRetrySameRevisionAfterFailure(t *testing.T) {
	s := NewLatchStore(brokenSnapshotStore(t))
	candidate := Latches{StationID: "st-1", Revision: 7, HighReturn: true}
	if _, err := s.Apply(candidate); err == nil {
		t.Fatal("expected persist error")
	}
	if got := s.Get("st-1"); got.Revision != 0 {
		t.Fatalf("failed apply consumed revision: %+v", got)
	}

	s.snapshots = store.NewStationSnapshotStore(t.TempDir())
	applied, err := s.Apply(candidate)
	if err != nil {
		t.Fatalf("retry with same revision: %v", err)
	}
	if applied.Revision != 7 || !applied.HighReturn {
		t.Fatalf("retry not applied: %+v", applied)
	}
	snapshot, ok, err := s.snapshots.Get("st-1")
	if err != nil || !ok {
		t.Fatalf("load snapshot: ok=%v err=%v", ok, err)
	}
	if snapshot.Protection.Revision != 7 || !snapshot.Protection.HighReturn {
		t.Fatalf("retry not persisted: %+v", snapshot.Protection)
	}
}

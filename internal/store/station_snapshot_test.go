package store

import "testing"

func TestMergeProtectionRejectsStaleRevision(t *testing.T) {
	snapshots := NewStationSnapshotStore(t.TempDir())
	if _, err := snapshots.MergeProtection("st-1", ProtectionSnapshot{Revision: 5, HighReturn: true}); err != nil {
		t.Fatalf("initial merge: %v", err)
	}

	if _, err := snapshots.MergeProtection("st-1", ProtectionSnapshot{Revision: 4, PressureTrip: true}); err == nil {
		t.Fatal("expected stale revision error")
	}
	snapshot, ok, err := snapshots.Get("st-1")
	if err != nil || !ok {
		t.Fatalf("load snapshot: ok=%v err=%v", ok, err)
	}
	if snapshot.Protection.Revision != 5 || !snapshot.Protection.HighReturn || snapshot.Protection.PressureTrip {
		t.Fatalf("stale merge changed snapshot: %+v", snapshot.Protection)
	}

	// An equal revision is an idempotent retry and must still apply.
	if _, err := snapshots.MergeProtection("st-1", ProtectionSnapshot{Revision: 5, HighReturn: true}); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
}

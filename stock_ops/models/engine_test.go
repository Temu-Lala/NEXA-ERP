package models

import "testing"

func slot(id, prio int, capKG, loadKG float64) PutawaySlot {
	return PutawaySlot{LocID: id, Name: "L", PosPriority: prio, CapacityKG: capKG, LoadKG: loadKG}
}

func TestRankPutawayFiltersOverCapacity(t *testing.T) {
	cands := []PutawaySlot{
		slot(1, 0, 10, 5),  // 5kg free — too small for 6kg
		slot(2, 0, 100, 0), // 100kg free — fits
		slot(3, 0, 0, 0),   // unlimited — fits
	}
	ranked := rankPutaway(cands, 6, 0)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 fitting slots, got %d", len(ranked))
	}
	if ranked[0].LocID != 2 || ranked[1].LocID != 3 {
		t.Errorf("best-fit then unlimited expected, got %d,%d", ranked[0].LocID, ranked[1].LocID)
	}
}

func TestRankPutawayPriorityFirst(t *testing.T) {
	cands := []PutawaySlot{
		slot(1, 20, 500, 0),
		slot(2, 1, 500, 0),
		slot(3, 10, 500, 0),
	}
	ranked := rankPutaway(cands, 10, 0)
	if ranked[0].LocID != 2 || ranked[1].LocID != 3 || ranked[2].LocID != 1 {
		t.Errorf("pos_priority ascending expected, got %d,%d,%d",
			ranked[0].LocID, ranked[1].LocID, ranked[2].LocID)
	}
}

func TestRankPutawayVolumeLimit(t *testing.T) {
	cands := []PutawaySlot{
		{LocID: 1, CapacityM3: 2, LoadM3: 1.5}, // only 0.5 m3 free
		{LocID: 2, CapacityM3: 2, LoadM3: 0},   // 2 m3 free
	}
	ranked := rankPutaway(cands, 0, 1)
	if len(ranked) != 1 || ranked[0].LocID != 2 {
		t.Errorf("volume overflow slot must drop out, got %+v", ranked)
	}
}

func TestRankPutawayTiesDeterministic(t *testing.T) {
	cands := []PutawaySlot{slot(7, 1, 0, 0), slot(3, 1, 10, 0)}
	ranked := rankPutaway(cands, 1, 0)
	// finite headroom (slot 3) ranks before unlimited (slot 7)
	if ranked[0].LocID != 3 || ranked[1].LocID != 7 {
		t.Errorf("finite-before-unlimited expected, got %d,%d", ranked[0].LocID, ranked[1].LocID)
	}
}

func TestScanMatches(t *testing.T) {
	if !scanMatches(" ABC-123 ", "abc-123", "") {
		t.Error("trim + case-insensitive match expected")
	}
	if !scanMatches("REF9", "", "ref9") {
		t.Error("internal reference should match")
	}
	if scanMatches("", "abc", "ref") {
		t.Error("empty scan must never match")
	}
	if scanMatches("xyz", "abc", "ref") {
		t.Error("unrelated code must not match")
	}
}

func TestExpectedCodes(t *testing.T) {
	if got := expectedCodes("B1", "R1"); got != "B1 / R1" {
		t.Errorf("both codes expected, got %q", got)
	}
	if got := expectedCodes("", "R1"); got != "R1" {
		t.Errorf("single code expected, got %q", got)
	}
}

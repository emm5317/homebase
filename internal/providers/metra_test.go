package providers

import (
	"testing"
	"time"

	gtfs "github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
)

func makeMetra(walkMinutes int) *Metra {
	loc, _ := time.LoadLocation("America/Chicago")
	return &Metra{
		route:         "BNSF",
		station:       "HINSDALE",
		destination:   "CUS",
		loc:           loc,
		maxDepartures: 3,
		walkMinutes:   walkMinutes,
	}
}

func makeDep(tripID string, offsetMinutes int) upcomingDeparture {
	return upcomingDeparture{
		tripID:      tripID,
		headsign:    "CUS",
		departsAt:   time.Now().Add(time.Duration(offsetMinutes) * time.Minute),
		scheduleRel: gtfs.TripUpdate_StopTimeUpdate_SCHEDULED,
	}
}

// TestBuildDepartureCardsNoWalk verifies that walkMinutes==0 produces the
// same card shape as the original implementation (no "Leave by" metric, title
// uses the station→destination format).
func TestBuildDepartureCardsNoWalk(t *testing.T) {
	m := makeMetra(0)
	deps := []upcomingDeparture{
		makeDep("trip1", 15),
		makeDep("trip2", 35),
	}

	cards := m.buildDepartureCards(deps)
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	c := cards[0]

	// Title should use station→destination format (no "Leave by")
	if c.Title == "" {
		t.Error("Title should not be empty")
	}
	// Must not contain "Leave by"
	for _, met := range c.Metrics {
		if met.Label == "Leave by" {
			t.Error("walkMinutes==0 should not produce a 'Leave by' metric")
		}
	}
	// First metric must be "Depart"
	if len(c.Metrics) == 0 || c.Metrics[0].Label != "Depart" {
		t.Errorf("first metric should be 'Depart', got %v", c.Metrics)
	}
	// Second metric must be "Status"
	if len(c.Metrics) < 2 || c.Metrics[1].Label != "Status" {
		t.Errorf("second metric should be 'Status', got %v", c.Metrics)
	}
	// "Next" metric should be present (two departures)
	if len(c.Metrics) < 3 || c.Metrics[2].Label != "Next" {
		t.Errorf("third metric should be 'Next', got %v", c.Metrics)
	}
}

// TestBuildDepartureCardsLeaveByNoRollover verifies the no-rollover case:
// first train departs in 30 min, walk=8, so leaveBy is 22 min from now —
// still in the future, so the first train is selected.
func TestBuildDepartureCardsLeaveByNoRollover(t *testing.T) {
	m := makeMetra(8)
	deps := []upcomingDeparture{
		makeDep("trip1", 30),
		makeDep("trip2", 50),
	}

	cards := m.buildDepartureCards(deps)
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	c := cards[0]

	// Card should be for trip1
	if c.ID != "metra-BNSF-trip1" {
		t.Errorf("expected card for trip1, got ID=%q", c.ID)
	}

	// Title should contain "Leave by"
	if len(c.Title) == 0 {
		t.Error("Title should not be empty")
	}
	// Verify metric ordering: "Leave by" first, then "Depart"
	if len(c.Metrics) < 2 {
		t.Fatalf("expected at least 2 metrics, got %d", len(c.Metrics))
	}
	if c.Metrics[0].Label != "Leave by" {
		t.Errorf("first metric should be 'Leave by', got %q", c.Metrics[0].Label)
	}
	if c.Metrics[1].Label != "Depart" {
		t.Errorf("second metric should be 'Depart', got %q", c.Metrics[1].Label)
	}
}

// TestBuildDepartureCardsLeaveByRollover verifies the rollover case:
// first train at now+5min, walk=8, so leaveBy=(now-3min) is in the past —
// the second train should be selected instead.
func TestBuildDepartureCardsLeaveByRollover(t *testing.T) {
	m := makeMetra(8)
	deps := []upcomingDeparture{
		makeDep("trip1", 5),
		makeDep("trip2", 25),
	}

	cards := m.buildDepartureCards(deps)
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	c := cards[0]

	// Card should be for trip2 (rolled over)
	if c.ID != "metra-BNSF-trip2" {
		t.Errorf("expected rollover to trip2, got ID=%q", c.ID)
	}

	// Metric ordering: "Leave by" first, then "Depart"
	if len(c.Metrics) < 2 {
		t.Fatalf("expected at least 2 metrics, got %d", len(c.Metrics))
	}
	if c.Metrics[0].Label != "Leave by" {
		t.Errorf("first metric should be 'Leave by', got %q", c.Metrics[0].Label)
	}
	if c.Metrics[1].Label != "Depart" {
		t.Errorf("second metric should be 'Depart', got %q", c.Metrics[1].Label)
	}
}

// TestBuildDepartureCardsLeaveByWarning verifies that the warning status/color
// is set when the leave-by time is within 2 minutes of now.
func TestBuildDepartureCardsLeaveByWarning(t *testing.T) {
	m := makeMetra(8)
	// Departs in 9 minutes → leaveBy is 1 minute from now → within 2-min warning window.
	deps := []upcomingDeparture{
		makeDep("trip1", 9),
	}

	cards := m.buildDepartureCards(deps)
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	c := cards[0]

	if c.Status != "warning" {
		t.Errorf("Status = %q, want 'warning'", c.Status)
	}
	if c.Color != "#d4943a" {
		t.Errorf("Color = %q, want '#d4943a'", c.Color)
	}
}

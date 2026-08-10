package core

import (
	"testing"
	"time"

	"github.com/knadh/listmonk/models"
)

func TestIsInsideSequenceSchedule(t *testing.T) {
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("failed loading America/New_York location: %v", err)
	}

	sched := models.SequenceSchedule{
		Enabled:   true,
		StartTime: "09:00",
		EndTime:   "17:00",
		Days:      []string{"mon", "tue", "wed", "thu", "fri"},
	}

	// Case 1: Monday at 10:00 AM NY (Inside schedule)
	mon10am := time.Date(2025, time.March, 10, 10, 0, 0, 0, nyLoc)
	inside, _ := IsInsideSequenceSchedule(sched, nyLoc, mon10am)
	if !inside {
		t.Errorf("expected inside schedule on Monday 10:00 AM, got false")
	}

	// Case 2: Monday at 08:00 AM NY (Before start time)
	mon8am := time.Date(2025, time.March, 10, 8, 0, 0, 0, nyLoc)
	inside, nextStart := IsInsideSequenceSchedule(sched, nyLoc, mon8am)
	if inside {
		t.Errorf("expected outside schedule on Monday 8:00 AM, got true")
	}
	expectedStart := time.Date(2025, time.March, 10, 9, 0, 0, 0, nyLoc)
	if !nextStart.Equal(expectedStart) {
		t.Errorf("expected next start %v, got %v", expectedStart, nextStart)
	}

	// Case 3: Saturday at 12:00 PM NY (Non-sending day -> Defers to Monday 9:00 AM)
	sat12pm := time.Date(2025, time.March, 15, 12, 0, 0, 0, nyLoc)
	inside, nextStartSat := IsInsideSequenceSchedule(sched, nyLoc, sat12pm)
	if inside {
		t.Errorf("expected outside schedule on Saturday 12:00 PM, got true")
	}
	expectedMon := time.Date(2025, time.March, 17, 9, 0, 0, 0, nyLoc)
	if !nextStartSat.Equal(expectedMon) {
		t.Errorf("expected next start Monday %v, got %v", expectedMon, nextStartSat)
	}
}

func TestCalculatePacedInterval(t *testing.T) {
	sched := models.SequenceSchedule{
		Enabled:            true,
		MinIntervalSeconds: 30,
	}

	// 120 contacts in 8 hours (28,800 seconds) -> 240 seconds per message
	interval := CalculatePacedInterval(sched, 120, 28800)
	if interval != 240 {
		t.Errorf("expected interval 240s, got %d", interval)
	}

	// 1,000 contacts in 1 hour (3,600 seconds) -> 3.6 seconds -> Capped at MinIntervalSeconds (30s)
	cappedInterval := CalculatePacedInterval(sched, 1000, 3600)
	if cappedInterval != 30 {
		t.Errorf("expected floor capped interval 30s, got %d", cappedInterval)
	}
}

func TestCalculatePacedScheduleTimestamps(t *testing.T) {
	tokyoLoc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("failed loading Asia/Tokyo location: %v", err)
	}

	sched := models.SequenceSchedule{
		Enabled:            true,
		StartTime:          "09:00",
		EndTime:            "17:00",
		MinIntervalSeconds: 60,
		JitterSeconds:      0, // Zero jitter for deterministic unit test
	}

	start := time.Date(2025, time.March, 10, 9, 0, 0, 0, tokyoLoc)
	timestamps := CalculatePacedScheduleTimestamps(sched, tokyoLoc, start, 5)

	if len(timestamps) != 5 {
		t.Fatalf("expected 5 timestamps, got %d", len(timestamps))
	}

	// Total remaining time: 8 hours = 28,800 sec / 5 = 5,760 sec (96 min) interval
	diff := timestamps[1].Sub(timestamps[0]).Seconds()
	if diff != 5760 {
		t.Errorf("expected 5760s gap between timestamps, got %f", diff)
	}
}

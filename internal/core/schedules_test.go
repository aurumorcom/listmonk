package core

import (
	"testing"
	"time"

	"github.com/knadh/listmonk/models"
)

func TestIsInsideSchedule_Apollo(t *testing.T) {
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("failed to load location: %v", err)
	}

	sched := &models.Schedule{
		Timezone:           "America/New_York",
		UseContactTimezone: false,
		SkipHolidays:       true,
		SendingWindows:     models.JSON{"mon": []map[string]string{{"start": "08:00", "end": "17:00"}}},
	}

	// Mon March 10, 2025 at 10:00 AM NY time
	mon10am := time.Date(2025, time.March, 10, 10, 0, 0, 0, nyLoc)
	inside, _ := IsInsideSchedule(sched, nil, mon10am)
	if !inside {
		t.Errorf("expected inside schedule window at 10 AM Monday")
	}

	// Mon March 10, 2025 at 6:00 PM NY time (outside)
	mon6pm := time.Date(2025, time.March, 10, 18, 0, 0, 0, nyLoc)
	inside6, _ := IsInsideSchedule(sched, nil, mon6pm)
	if inside6 {
		t.Errorf("expected outside schedule window at 6 PM Monday")
	}

	// July 4, 2025 (Independence Day holiday)
	july4 := time.Date(2025, time.July, 4, 10, 0, 0, 0, nyLoc)
	insideHol, _ := IsInsideSchedule(sched, nil, july4)
	if insideHol {
		t.Errorf("expected holiday skip on July 4")
	}
}

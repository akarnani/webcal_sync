package main

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/apognu/gocal"
	"google.golang.org/api/calendar/v3"
)

func TestGetIDForEvent(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		event     gocal.Event
		expectUID bool
		expectURL bool
	}{
		{
			name: "empty IDFormat uses UID",
			cfg: Config{
				IDFormat: "",
			},
			event: gocal.Event{
				Uid: "test-uid-123",
				URL: "https://example.com/event",
			},
			expectUID: true,
		},
		{
			name: "url IDFormat uses SHA256 of URL",
			cfg: Config{
				IDFormat: "url",
			},
			event: gocal.Event{
				Uid: "test-uid-123",
				URL: "https://example.com/event",
			},
			expectURL: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIDForEvent(tt.cfg, tt.event)

			if tt.expectUID {
				if result != tt.event.Uid {
					t.Errorf("Expected UID %q, got %q", tt.event.Uid, result)
				}
			}

			if tt.expectURL {
				expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(tt.event.URL)))
				if result != expectedHash {
					t.Errorf("Expected URL hash %q, got %q", expectedHash, result)
				}
			}
		})
	}

	t.Run("invalid IDFormat panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid IDFormat")
			}
		}()

		cfg := Config{IDFormat: "invalid"}
		event := gocal.Event{Uid: "test"}
		getIDForEvent(cfg, event)
	})
}

func TestIsAllDayEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    gocal.Event
		expected bool
	}{
		{
			name: "all-day event with DATE value in start",
			event: gocal.Event{
				RawStart: gocal.RawDate{
					Params: map[string]string{"VALUE": "DATE"},
				},
			},
			expected: true,
		},
		{
			name: "all-day event with DATE value in end",
			event: gocal.Event{
				RawEnd: gocal.RawDate{
					Params: map[string]string{"VALUE": "DATE"},
				},
			},
			expected: true,
		},
		{
			name: "timed event with no DATE value",
			event: gocal.Event{
				RawStart: gocal.RawDate{
					Params: map[string]string{},
				},
				RawEnd: gocal.RawDate{
					Params: map[string]string{},
				},
			},
			expected: false,
		},
		{
			name: "timed event with DATETIME value",
			event: gocal.Event{
				RawStart: gocal.RawDate{
					Params: map[string]string{"VALUE": "DATETIME"},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAllDayEvent(tt.event)
			if result != tt.expected {
				t.Errorf("isAllDayEvent() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCompareDateTime(t *testing.T) {
	tests := []struct {
		name     string
		t1       time.Time
		t2       time.Time
		expected bool
	}{
		{
			name:     "exact same time",
			t1:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			t2:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "same time different subsecond",
			t1:       time.Date(2024, 1, 1, 12, 0, 0, 500000000, time.UTC),
			t2:       time.Date(2024, 1, 1, 12, 0, 0, 100000000, time.UTC),
			expected: true,
		},
		{
			name:     "different seconds",
			t1:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			t2:       time.Date(2024, 1, 1, 12, 0, 1, 0, time.UTC),
			expected: false,
		},
		{
			name:     "different days",
			t1:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			t2:       time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareDateTime(tt.t1, tt.t2)
			if result != tt.expected {
				t.Errorf("compareDateTime() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetEventTime(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 14, 30, 45, 123456789, time.UTC)

	t.Run("all-day event", func(t *testing.T) {
		result := getEventTime(testTime, true)

		if result.Date != "2024-01-15" {
			t.Errorf("Expected Date '2024-01-15', got %q", result.Date)
		}
		if result.DateTime != "" {
			t.Errorf("Expected empty DateTime, got %q", result.DateTime)
		}
		if len(result.NullFields) == 0 || result.NullFields[0] != "DateTime" {
			t.Errorf("Expected NullFields to contain 'DateTime'")
		}
	})

	t.Run("timed event", func(t *testing.T) {
		result := getEventTime(testTime, false)

		expectedDateTime := "2024-01-15T14:30:45Z"
		if result.DateTime != expectedDateTime {
			t.Errorf("Expected DateTime %q, got %q", expectedDateTime, result.DateTime)
		}
		if result.Date != "" {
			t.Errorf("Expected empty Date, got %q", result.Date)
		}
		if len(result.NullFields) == 0 || result.NullFields[0] != "Date" {
			t.Errorf("Expected NullFields to contain 'Date'")
		}
	})
}

func TestParseGCalTime(t *testing.T) {
	t.Run("parse date only", func(t *testing.T) {
		eventTime := &calendar.EventDateTime{
			Date: "2024-01-15",
		}

		result := parseGCalTime(eventTime)
		expected := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

		if !result.Equal(expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("parse datetime", func(t *testing.T) {
		eventTime := &calendar.EventDateTime{
			DateTime: "2024-01-15T14:30:45Z",
		}

		result := parseGCalTime(eventTime)
		expected := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)

		if !result.Equal(expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("invalid date panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid date")
			}
		}()

		eventTime := &calendar.EventDateTime{
			Date: "not-a-date",
		}
		parseGCalTime(eventTime)
	})

	t.Run("invalid datetime panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid datetime")
			}
		}()

		eventTime := &calendar.EventDateTime{
			DateTime: "not-a-datetime",
		}
		parseGCalTime(eventTime)
	})
}

func TestGetReminderForConfig(t *testing.T) {
	t.Run("no reminder configured", func(t *testing.T) {
		cfg := Config{Reminder: 0}
		result := getReminderForConfig(cfg)

		if result != nil {
			t.Errorf("Expected nil reminder, got %v", result)
		}
	})

	t.Run("reminder configured", func(t *testing.T) {
		cfg := Config{Reminder: 30}
		result := getReminderForConfig(cfg)

		if result == nil {
			t.Fatal("Expected non-nil reminder")
		}

		if len(result.Overrides) != 1 {
			t.Fatalf("Expected 1 override, got %d", len(result.Overrides))
		}

		if result.Overrides[0].Method != "popup" {
			t.Errorf("Expected method 'popup', got %q", result.Overrides[0].Method)
		}

		if result.Overrides[0].Minutes != 30 {
			t.Errorf("Expected 30 minutes, got %d", result.Overrides[0].Minutes)
		}

		if len(result.ForceSendFields) == 0 || result.ForceSendFields[0] != "UseDefault" {
			t.Errorf("Expected ForceSendFields to contain 'UseDefault'")
		}
	})
}

func TestICalToGEvent(t *testing.T) {
	startTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 15, 15, 30, 0, 0, time.UTC)

	cfg := Config{
		URL:      "https://example.com/calendar.ics",
		ColorID:  "3",
		IDFormat: "",
		Reminder: 15,
	}

	event := gocal.Event{
		Uid:         "event-123",
		Summary:     "Test Event\\nWith Newline",
		Description: "Test Description",
		Location:    "Test Location",
		Start:       &startTime,
		End:         &endTime,
		RawStart: gocal.RawDate{
			Params: map[string]string{},
		},
		RawEnd: gocal.RawDate{
			Params: map[string]string{},
		},
	}

	result := iCalToGEvent(cfg, event)

	t.Run("summary is unescaped", func(t *testing.T) {
		expected := "Test Event\nWith Newline"
		if result.Summary != expected {
			t.Errorf("Expected summary %q, got %q", expected, result.Summary)
		}
	})

	t.Run("other fields are set", func(t *testing.T) {
		if result.Description != "Test Description" {
			t.Errorf("Expected description 'Test Description', got %q", result.Description)
		}
		if result.Location != "Test Location" {
			t.Errorf("Expected location 'Test Location', got %q", result.Location)
		}
		if result.ICalUID != "event-123" {
			t.Errorf("Expected ICalUID 'event-123', got %q", result.ICalUID)
		}
		if result.ColorId != "3" {
			t.Errorf("Expected ColorId '3', got %q", result.ColorId)
		}
	})

	t.Run("extended properties include URL hash", func(t *testing.T) {
		if result.ExtendedProperties == nil {
			t.Fatal("Expected ExtendedProperties to be set")
		}
		if result.ExtendedProperties.Private == nil {
			t.Fatal("Expected Private extended properties to be set")
		}

		expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.URL)))
		if result.ExtendedProperties.Private["url"] != expectedHash {
			t.Errorf("Expected url hash %q, got %q", expectedHash, result.ExtendedProperties.Private["url"])
		}
	})

	t.Run("reminder is set", func(t *testing.T) {
		if result.Reminders == nil {
			t.Fatal("Expected Reminders to be set")
		}
		if len(result.Reminders.Overrides) != 1 {
			t.Fatalf("Expected 1 reminder override, got %d", len(result.Reminders.Overrides))
		}
		if result.Reminders.Overrides[0].Minutes != 15 {
			t.Errorf("Expected 15 minute reminder, got %d", result.Reminders.Overrides[0].Minutes)
		}
	})
}

func TestDiffEvents(t *testing.T) {
	futureTime := time.Now().Add(24 * time.Hour)
	pastTime := time.Now().Add(-24 * time.Hour)

	cfg := Config{
		URL:      "https://example.com/calendar.ics",
		ColorID:  "1",
		IDFormat: "",
		Reminder: 30,
	}

	t.Run("creates new events", func(t *testing.T) {
		upstreamEvents := []gocal.Event{
			{
				Uid:     "new-event",
				Summary: "New Event",
				Start:   &futureTime,
				End:     &futureTime,
				RawStart: gocal.RawDate{
					Params: map[string]string{},
				},
				RawEnd: gocal.RawDate{
					Params: map[string]string{},
				},
			},
		}

		gcalEvents := []*calendar.Event{}

		create, update, delete := diffEvents(cfg, upstreamEvents, gcalEvents)

		if len(create) != 1 {
			t.Errorf("Expected 1 create, got %d", len(create))
		}
		if len(update) != 0 {
			t.Errorf("Expected 0 updates, got %d", len(update))
		}
		if len(delete) != 0 {
			t.Errorf("Expected 0 deletes, got %d", len(delete))
		}

		if len(create) > 0 && create[0].ICalUID != "new-event" {
			t.Errorf("Expected ICalUID 'new-event', got %q", create[0].ICalUID)
		}
	})

	t.Run("updates existing events with changes", func(t *testing.T) {
		upstreamEvents := []gocal.Event{
			{
				Uid:     "existing-event",
				Summary: "Updated Summary",
				Start:   &futureTime,
				End:     &futureTime,
				RawStart: gocal.RawDate{
					Params: map[string]string{},
				},
				RawEnd: gocal.RawDate{
					Params: map[string]string{},
				},
			},
		}

		gcalEvents := []*calendar.Event{
			{
				Id:       "gcal-id-123",
				ICalUID:  "existing-event",
				Summary:  "Old Summary",
				Start:    getEventTime(futureTime, false),
				End:      getEventTime(futureTime, false),
				ColorId:  "1",
				Status:   "confirmed",
				Reminders: &calendar.EventReminders{},
			},
		}

		create, update, delete := diffEvents(cfg, upstreamEvents, gcalEvents)

		if len(create) != 0 {
			t.Errorf("Expected 0 creates, got %d", len(create))
		}
		if len(update) != 1 {
			t.Errorf("Expected 1 update, got %d", len(update))
		}
		if len(delete) != 0 {
			t.Errorf("Expected 0 deletes, got %d", len(delete))
		}

		if len(update) > 0 {
			if update[0].Summary != "Updated Summary" {
				t.Errorf("Expected summary 'Updated Summary', got %q", update[0].Summary)
			}
			if update[0].Id != "gcal-id-123" {
				t.Errorf("Expected Id 'gcal-id-123', got %q", update[0].Id)
			}
		}
	})

	t.Run("deletes events not in upstream", func(t *testing.T) {
		upstreamEvents := []gocal.Event{}

		gcalEvents := []*calendar.Event{
			{
				Id:      "gcal-id-123",
				ICalUID: "deleted-event",
				Summary: "To Be Deleted",
				Start: &calendar.EventDateTime{
					DateTime: futureTime.Format(time.RFC3339),
				},
				Status: "confirmed",
			},
		}

		create, update, delete := diffEvents(cfg, upstreamEvents, gcalEvents)

		if len(create) != 0 {
			t.Errorf("Expected 0 creates, got %d", len(create))
		}
		if len(update) != 0 {
			t.Errorf("Expected 0 updates, got %d", len(update))
		}
		if len(delete) != 1 {
			t.Errorf("Expected 1 delete, got %d", len(delete))
		}

		if len(delete) > 0 && delete[0] != "gcal-id-123" {
			t.Errorf("Expected delete ID 'gcal-id-123', got %q", delete[0])
		}
	})

	t.Run("ignores past events", func(t *testing.T) {
		upstreamEvents := []gocal.Event{
			{
				Uid:     "past-event",
				Summary: "Past Event",
				Start:   &pastTime,
				End:     &pastTime,
				RawStart: gocal.RawDate{
					Params: map[string]string{},
				},
				RawEnd: gocal.RawDate{
					Params: map[string]string{},
				},
			},
		}

		gcalEvents := []*calendar.Event{}

		create, _, _ := diffEvents(cfg, upstreamEvents, gcalEvents)

		if len(create) != 0 {
			t.Errorf("Expected 0 creates for past event, got %d", len(create))
		}
	})

	t.Run("skips duplicate IDs", func(t *testing.T) {
		upstreamEvents := []gocal.Event{
			{
				Uid:     "duplicate-event",
				Summary: "Event 1",
				Start:   &futureTime,
				End:     &futureTime,
				RawStart: gocal.RawDate{
					Params: map[string]string{},
				},
				RawEnd: gocal.RawDate{
					Params: map[string]string{},
				},
			},
			{
				Uid:     "duplicate-event",
				Summary: "Event 2",
				Start:   &futureTime,
				End:     &futureTime,
				RawStart: gocal.RawDate{
					Params: map[string]string{},
				},
				RawEnd: gocal.RawDate{
					Params: map[string]string{},
				},
			},
		}

		gcalEvents := []*calendar.Event{}

		create, _, _ := diffEvents(cfg, upstreamEvents, gcalEvents)

		// Should only create one event, skipping the duplicate
		if len(create) != 1 {
			t.Errorf("Expected 1 create (duplicate skipped), got %d", len(create))
		}
	})

	t.Run("does not delete already started events", func(t *testing.T) {
		recentPast := time.Now().Add(-1 * time.Hour)
		upstreamEvents := []gocal.Event{}

		gcalEvents := []*calendar.Event{
			{
				Id:      "gcal-id-123",
				ICalUID: "started-event",
				Summary: "Already Started",
				Start: &calendar.EventDateTime{
					DateTime: recentPast.Format(time.RFC3339),
				},
				Status: "confirmed",
			},
		}

		_, _, delete := diffEvents(cfg, upstreamEvents, gcalEvents)

		if len(delete) != 0 {
			t.Errorf("Expected 0 deletes for already started event, got %d", len(delete))
		}
	})
}

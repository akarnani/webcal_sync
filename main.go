package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // embed timezone info in the binary to handle calendars that don't use UTC

	"github.com/akarnani/webcal_sync/gcal"
	"github.com/apognu/gocal"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
)

var dateFormatFix = regexp.MustCompile(`(?m)^(DTSTAMP:.*)T(.*)$`)

func parseICal(url string) []gocal.Event {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}

	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read calendar body", "error", err)
		panic("Failed to read calendar body")
	}

	//This is really remarkably dumb but some sources give back garbage
	body := bytes.NewReader(dateFormatFix.ReplaceAll(b, []byte("$1")))

	c := gocal.NewParser(body)
	c.Strict = gocal.StrictParams{
		Mode: gocal.StrictModeFailAttribute,
	}
	if err := c.Parse(); err != nil {
		panic(err)
	}

	//"clever" way to trim whitespace on all string fields
	for _, e := range c.Events {
		v := reflect.ValueOf(&e).Elem()
		for i := 0; i < v.Type().NumField(); i++ {
			f := v.Field(i)
			if f.Kind() == reflect.String {
				f.SetString(strings.TrimSpace(f.String()))
			}
		}
	}

	return c.Events
}

func diffEvents(cfg Config, up []gocal.Event, gevent []*calendar.Event) ([]*calendar.Event, []*calendar.Event, []string) {
	var create []*calendar.Event
	var update []*calendar.Event

	ids := make(map[string]*calendar.Event)
	for _, e := range gevent {
		ids[e.ICalUID] = e
	}

	seenIds := make(map[string]any)

	for _, e := range up {
		if (*e.Start).Before(time.Now()) {
			continue
		}

		i := getIDForEvent(cfg, e)

		if _, ok := seenIds[i]; ok {
			slog.Warn("ID is a duplicate, not processing", "id", i)
			continue
		}

		seenIds[i] = nil

		g, ok := ids[i]
		delete(ids, i)
		if !ok {
			//create event
			create = append(create, iCalToGEvent(cfg, e))
			continue
		}
		changed := false
		n := &calendar.Event{
			Id: g.Id,
		}

		if unescapeICalText(e.Summary) != g.Summary {
			n.Summary = unescapeICalText(e.Summary)
			changed = true
		}

		ts := parseGCalTime(g.Start)
		te := parseGCalTime(g.End)

		allDay := isAllDayEvent(e)

		if !compareDateTime(ts, *e.Start) || (allDay && g.Start.DateTime != "") {
			n.Start = getEventTime(*e.Start, allDay)
			changed = true
		}

		// if all day only the date matters.  However gocal's end time is 1 milisecond before
		// the next day which makes comparing dates hard.  So, just truncate the time for simplicity
		if allDay {
			*e.End = e.End.Truncate(24 * time.Hour)
		}
		if !compareDateTime(te, *e.End) || (allDay && g.End.DateTime != "") {
			n.End = getEventTime(*e.End, allDay)
			changed = true
		}
		if unescapeICalText(e.Description) != g.Description {
			n.Description = unescapeICalText(e.Description)
			n.ForceSendFields = append(n.ForceSendFields, "Description")
			changed = true
		}
		if unescapeICalText(e.Location) != g.Location {
			n.Location = unescapeICalText(e.Location)
			n.ForceSendFields = append(n.ForceSendFields, "Location")
			changed = true
		}
		if cfg.ColorID != g.ColorId {
			n.ColorId = cfg.ColorID
			changed = true
		}
		if g.Status != "confirmed" {
			n.Status = "confirmed"
			changed = true
		}

		if cfg.Reminder != 0 {
			//reminder configured
			if g.Reminders == nil || len(g.Reminders.Overrides) == 0 || g.Reminders.Overrides[0].Minutes != int64(cfg.Reminder) || g.Reminders.UseDefault {
				n.Reminders = getReminderForConfig(cfg)
				changed = true
			}
		} else {
			//reminder not configured
			if g.Reminders != nil {
				n.Reminders = &calendar.EventReminders{
					ForceSendFields: []string{"Overrides"},
				}
				changed = true
			}
		}

		if changed {
			update = append(update, n)
		}

	}

	del := make([]string, 0, len(ids))
	for _, e := range ids {
		if e.Status != "cancelled" {
			t, err := time.Parse(time.RFC3339, e.Start.DateTime)
			if err != nil {
				slog.Error("Unable to parse canceled date time", "datetime", e.Start.DateTime, "error", err)
				panic(fmt.Sprintf("Unable to parse canceled date time %s: %v", e.Start.DateTime, err))
			}
			if time.Now().Before(t) {
				del = append(del, e.Id)
			} else {
				slog.Info("Not deleting event because it already started", "summary", e.Summary)
			}
		}
	}
	return create, update, del

}

func iCalToGEvent(cfg Config, e gocal.Event) *calendar.Event {
	allDay := isAllDayEvent(e)
	return &calendar.Event{
		Summary:     unescapeICalText(e.Summary),
		Location:    unescapeICalText(e.Location),
		Description: unescapeICalText(e.Description),
		Start:       getEventTime(*e.Start, allDay),
		End:         getEventTime(*e.End, allDay),
		ICalUID:     getIDForEvent(cfg, e),
		ColorId:     cfg.ColorID,
		ExtendedProperties: &calendar.EventExtendedProperties{
			Private: map[string]string{"url": fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.URL)))},
		},
		Reminders: getReminderForConfig(cfg),
	}
}

func getReminderForConfig(cfg Config) *calendar.EventReminders {
	if cfg.Reminder == 0 { //zero value means not specified
		return nil
	}

	return &calendar.EventReminders{
		Overrides: []*calendar.EventReminder{
			{
				Method:  "popup",
				Minutes: int64(cfg.Reminder),
			},
		},
		ForceSendFields: []string{
			"UseDefault",
		},
	}
}

func getIDForEvent(cfg Config, e gocal.Event) string {
	switch cfg.IDFormat {
	case "url":
		return fmt.Sprintf("%x", sha256.Sum256([]byte(e.URL)))
	case "":
		return e.Uid
	default:
		slog.Error("unknown id format", "format", cfg.IDFormat)
		panic(fmt.Sprintf("unknown id format %s", cfg.IDFormat))
	}
}

func isAllDayEvent(e gocal.Event) bool {
	return e.RawStart.Params["VALUE"] == "DATE" || e.RawEnd.Params["VALUE"] == "DATE"
}

func compareDateTime(x, y time.Time) bool {
	return x.Truncate(time.Second).Equal(y.Truncate(time.Second))
}

func getEventTime(t time.Time, allDay bool) *calendar.EventDateTime {
	if allDay {
		return &calendar.EventDateTime{
			Date:       t.Format("2006-01-02"),
			NullFields: []string{"DateTime"},
		}
	}

	return &calendar.EventDateTime{
		DateTime:   t.Truncate(time.Second).Format(time.RFC3339),
		NullFields: []string{"Date"},
	}
}

func parseGCalTime(t *calendar.EventDateTime) time.Time {
	if t.Date != "" {
		out, err := time.Parse("2006-01-02", t.Date)
		if err != nil {
			slog.Error("Unable to parse date", "date", t.Date, "error", err)
			panic(fmt.Sprintf("Unable to parse date %s: %v", t.Date, err))
		}

		return out
	}

	out, err := time.Parse(time.RFC3339, t.DateTime)
	if err != nil {
		slog.Error("Unable to parse date time", "datetime", t.DateTime, "error", err)
		panic(fmt.Sprintf("Unable to parse date time %s: %v", t.DateTime, err))
	}

	return out
}

func main() {
	client := gcal.NewClient()
	for _, cfg := range getConfig() {
		slog.Info("Starting on calendar", "url", cfg.URL)
		c, u, d := diffEvents(cfg, parseICal(cfg.URL), client.GetEventsForAttribute(map[string]string{"url": fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.URL)))}))
		slog.Info("Processing results", "url", cfg.URL, "created", len(c), "updated", len(u), "deleted", len(d))

		for _, e := range c {
			if err := client.CreateEvent(e); err != nil {
				var gErr *googleapi.Error
				if errors.As(err, &gErr) && gErr.Code == http.StatusConflict {
					slog.Warn("Event already existed", "summary", e.Summary, "error", gErr)
					continue
				}
				slog.Error("failed to create event", "error", err)
				panic(fmt.Sprintf("failed to create event: %v", err))
			}
		}

		for _, e := range u {
			if err := client.UpdateEvent(e); err != nil {
				slog.Error("failed to update event", "error", err)
				panic(fmt.Sprintf("failed to update event: %v", err))
			}
		}
		for _, id := range d {
			if err := client.DeleteEvent(id); err != nil {
				slog.Error("failed to delete event", "error", err)
				panic(fmt.Sprintf("failed to delete event: %v", err))
			}
		}

		slog.Info("finished with calendar")

	}
}

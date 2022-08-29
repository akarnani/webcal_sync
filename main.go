package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

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

	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read calendar body")
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

	seenIds := make(map[string]interface{})

	for _, e := range up {
		if (*e.Start).Before(time.Now()) {
			continue
		}

		i := getIDForEvent(cfg, e)

		if _, ok := seenIds[i]; ok {
			log.Printf("ID %s is a duplicate, not processing", i)
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

		if e.Summary != g.Summary {
			n.Summary = e.Summary
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
		if e.Description != g.Description {
			n.Description = e.Description
			n.ForceSendFields = append(n.ForceSendFields, "Description")
			changed = true
		}
		if e.Location != g.Location {
			n.Location = e.Location
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

		if changed {
			update = append(update, n)
		}

	}

	del := make([]string, 0, len(ids))
	for _, e := range ids {
		if e.Status != "cancelled" {
			t, err := time.Parse(time.RFC3339, e.Start.DateTime)
			if err != nil {
				log.Fatalf("Unable to parse canceled date time %s: %v", e.Start.DateTime, err)
			}
			if time.Now().Before(t) {
				del = append(del, e.Id)
			} else {
				log.Printf("Not deleting event %s because it already started", e.Summary)
			}
		}
	}
	return create, update, del

}

func iCalToGEvent(cfg Config, e gocal.Event) *calendar.Event {
	allDay := isAllDayEvent(e)
	return &calendar.Event{
		Summary:     jsonEscape(e.Summary),
		Location:    jsonEscape(e.Location),
		Description: jsonEscape(e.Description),
		Start:       getEventTime(*e.Start, allDay),
		End:         getEventTime(*e.End, allDay),
		ICalUID:     getIDForEvent(cfg, e),
		ColorId:     cfg.ColorID,
		ExtendedProperties: &calendar.EventExtendedProperties{
			Private: map[string]string{"url": fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.URL)))},
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
		log.Panicf("unknown id format %s", cfg.IDFormat)
	}

	//can't be reached due to default's Panicf
	return ""
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
			log.Fatalf("Unable to parse date %s: %v", t.Date, err)
		}

		return out
	}

	out, err := time.Parse(time.RFC3339, t.DateTime)
	if err != nil {
		log.Fatalf("Unable to parse date time %s: %v", t.DateTime, err)
	}

	return out
}

func main() {
	client := gcal.NewClient()
	for _, cfg := range getConfig() {
		log.Printf("Starting on calendar %s", cfg.URL)
		c, u, d := diffEvents(cfg, parseICal(cfg.URL), client.GetEventsForAttribute(map[string]string{"url": fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.URL)))}))
		log.Println(cfg.URL, len(c), len(u), len(d))

		for _, e := range c {
			if err := client.CreateEvent(e); err != nil {
				var gErr *googleapi.Error
				if errors.As(err, &gErr) && gErr.Code == http.StatusConflict {
					log.Printf("Event already existed: %v, %v", e, gErr)
					continue
				}
				log.Fatalf("failed to create event: %v", err)
			}
		}

		for _, e := range u {
			if err := client.UpdateEvent(e); err != nil {
				log.Fatalf("failed to update event: %v", err)
			}
		}
		for _, id := range d {
			if err := client.DeleteEvent(id); err != nil {
				log.Fatalf("failed to update event: %v", err)
			}
		}

		log.Printf("finished with calendar")

	}
}

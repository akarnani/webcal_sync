package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/akarnani/webcal_sync/gcal"
	"github.com/apognu/gocal"
	"google.golang.org/api/calendar/v3"
)

func parseICal(url string) []gocal.Event {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	c := gocal.NewParser(resp.Body)
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

	for _, e := range up {
		if (*e.Start).Before(time.Now()) {
			continue
		}

		i := getIDForEvent(cfg, e)
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
		t, err := time.Parse(time.RFC3339, g.Start.DateTime)
		if err != nil {
			log.Fatalf("Unable to parse date time %s: %v", g.Start.DateTime, err)
		}
		if !t.Equal(*e.Start) {
			n.Start = &calendar.EventDateTime{DateTime: e.Start.Format(time.RFC3339)}
			changed = true
		}
		t, err = time.Parse(time.RFC3339, g.End.DateTime)
		if err != nil {
			log.Fatalf("Unable to parse date time %s: %v", g.End.DateTime, err)
		}
		if !t.Equal(*e.End) {
			n.End = &calendar.EventDateTime{DateTime: e.End.Format(time.RFC3339)}
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
			del = append(del, e.Id)
		}
	}
	return create, update, del

}

func iCalToGEvent(cfg Config, e gocal.Event) *calendar.Event {
	return &calendar.Event{
		Summary:     e.Summary,
		Location:    e.Location,
		Description: e.Description,
		ICalUID:     getIDForEvent(cfg, e),
		ColorId:     cfg.ColorID,
		Start: &calendar.EventDateTime{
			DateTime: e.Start.Format(time.RFC3339),
		},
		End: &calendar.EventDateTime{
			DateTime: e.End.Format(time.RFC3339),
		},
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
		panic(fmt.Sprintf("unknown id format %s", cfg.IDFormat))
	}
}

func main() {
	client := gcal.NewClient()
	for _, cfg := range getConfig() {
		c, u, d := diffEvents(cfg, parseICal(cfg.URL), client.GetEventsForAttribute(map[string]string{"url": fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.URL)))}))
		fmt.Println(cfg.URL, len(c), len(u), len(d))

		for _, e := range c {
			if err := client.CreateEvent(e); err != nil {
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

	}
}

package gcal

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Client struct {
	svc *calendar.Service
}

func NewClient() *Client {
	ctx := context.Background()
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarEventsScope, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	return &Client{svc: srv}
}

func (c *Client) GetEventsForAttribute(attrs map[string]string) []*calendar.Event {

	var events []*calendar.Event

	r := c.svc.Events.List("primary").TimeMin(time.Now().Format(time.RFC3339)).
		ShowDeleted(true)
	for k, v := range attrs {
		r = r.PrivateExtendedProperty(fmt.Sprintf("%s=%s", k, v))
	}

	err := r.Pages(context.Background(), func(e *calendar.Events) error {
		events = append(events, e.Items...)
		return nil
	})

	if err != nil {
		log.Fatalf("Unable to list events: %v", err)
	}

	return events

}

func (c *Client) CreateEvent(e *calendar.Event) error {
	_, err := c.svc.Events.Insert("primary", e).Do()
	return err

}

func (c *Client) UpdateEvent(e *calendar.Event) error {
	_, err := c.svc.Events.Patch("primary", e.Id, e).Do()
	return err
}

func (c *Client) DeleteEvent(id string) error {
	return c.svc.Events.Delete("primary", id).Do()
}

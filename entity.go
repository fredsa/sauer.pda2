package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
)

// https://cloud.google.com/go/docs/reference/cloud.google.com/go/datastore/latest
type Entity struct {
	Key *datastore.Key `forkind:"hidden" datastore:"__key__"`

	// Person kind.
	Category    string `forkind:"Person" datastore:"category,omitempty,noindex"`
	SendCard    bool   `forkind:"Person" datastore:"send_card,omitempty,noindex"`
	Title       string `forkind:"Person" datastore:"title,omitempty,noindex"`
	MailingName string `forkind:"Person" datastore:"mailing_name,omitempty,noindex"`
	FirstName   string `forkind:"Person" datastore:"first_name,omitempty,noindex"`
	LastName    string `forkind:"Person" datastore:"last_name,omitempty,noindex"`
	CompanyName string `forkind:"Person" datastore:"company_name,omitempty,noindex"`

	// Address kind.
	AddressType   string `forkind:"Address" datastore:"address_type,omitempty,noindex"`
	AddressLine1  string `forkind:"Address" datastore:"address_line1,omitempty,noindex"`
	AddressLine2  string `forkind:"Address" datastore:"address_line2,omitempty,noindex"`
	City          string `forkind:"Address" datastore:"city,omitempty,noindex"`
	StateProvince string `forkind:"Address" datastore:"state_province,omitempty,noindex"`
	PostalCode    string `forkind:"Address" datastore:"postal_code,omitempty,noindex"`
	Country       string `forkind:"Address" datastore:"country,omitempty,noindex"`

	// TODO: delete after purging from datastore.
	Directions string `forkind:"hidden" datastore:"directions,omitempty,noindex" form:"textarea"`

	// Contact kind.
	ContactMethod string `forkind:"Contact" datastore:"contact_method,omitempty,noindex"`
	ContactType   string `forkind:"Contact" datastore:"contact_type,omitempty,noindex"`
	ContactText   string `forkind:"Contact" datastore:"contact_text,omitempty,noindex"`

	// Calendar kind.
	FirstOccurrence time.Time `forkind:"Calendar" datastore:"first_occurrence,omitempty,noindex"`
	Frequency       string    `forkind:"Calendar" datastore:"frequency,omitempty,noindex"`
	Occasion        string    `forkind:"Calendar" datastore:"occasion,omitempty,noindex"`

	// Common fields.
	Comments string   `forkind:"" datastore:"comments,omitempty,noindex" form:"textarea"`
	Enabled  bool     `forkind:"" datastore:"enabled,noindex"` // Required.
	Words    []string `forkind:"hidden" datastore:"words,omitempty,noindex"`
}

func (entity *Entity) maybeKey() string {
	if entity.Key == nil {
		return ""
	} else {
		return entity.Key.Encode()
	}
}

func requestToEntity(r *http.Request, client *datastore.Client) (entity *Entity, err error) {
	// kind := r.URL.Query().Get("kind")
	key := r.URL.Query().Get("key")
	dbkey, err := datastore.DecodeKey(key)
	var e Entity
	err = client.Get(ctx, dbkey, &e)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get %s: %v", dbkey.Kind, err))
	}
	return &e, nil
}

func requestToRootEntity(r *http.Request, client *datastore.Client) (entity *Entity, err error) {
	e, err := requestToEntity(r, client)
	if err != nil {
		return nil, err
	}

	if e.Key.Kind == "Person" {
		return e, nil
	} else {
		var person Entity
		err = client.Get(ctx, e.Key.Parent, &person)
		if err != nil {
			log.Fatalf("Failed to get parent for %s: %v", e.Key.Kind, err)
		}
		return &person, nil
	}
}

func (entity *Entity) enabledText() string {
	return enabledText(entity.Enabled)
}

func (entity *Entity) editUrl() string {
	// Include origin for a fully qualified URL.
	return fmt.Sprintf("%s/?action=edit&kind=%s&key=%s",
		defaultVersionOrigin,
		entity.Key.Kind,
		entity.Key.Encode(),
	)
}

func (person *Entity) displayName() string {
	t := ""
	if person.MailingName != "" {
		t += fmt.Sprintf("[%s] ", person.MailingName)
	}
	if person.CompanyName != "" {
		t += fmt.Sprintf("%s ", person.CompanyName)
	}
	if person.Title != "" {
		t += person.Title + " "
	}
	if person.FirstName != "" {
		t += person.FirstName + " "
	}
	if person.LastName != "" {
		t += person.LastName
	}
	return t
}

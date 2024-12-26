package main

import (
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
)

var categories = []string{
	"(Unspecified)",
	"Relatives",
	"Personal",
	"Hotel/Restaurant/Entertainment",
	"Services by Individuals",
	"Companies, Institutions, etc.",
	"Business Relations",
}

type Entity struct {
	Key *datastore.Key `datastore:"__key__"`

	// Common fields.
	Comments string   `datastore:"comments,noindex"`
	Enabled  bool     `datastore:"enabled,noindex"`
	Words    []string `datastore:"words,noindex"`

	// Person kind.
	Category    string `datastore:"category,noindex"`
	SendCard    bool   `datastore:"send_card,noindex"`
	Title       string `datastore:"title,noindex"`
	MailingName string `datastore:"mailing_name,noindex"`
	FirstName   string `datastore:"first_name,noindex"`
	LastName    string `datastore:"last_name,noindex"`
	CompanyName string `datastore:"company_name,noindex"`

	// Address kind.
	AddressType   string `datastore:"address_type,noindex"`
	AddressLine1  string `datastore:"address_line1,noindex"`
	AddressLine2  string `datastore:"address_line2,noindex"`
	City          string `datastore:"city,noindex"`
	StateProvince string `datastore:"state_province,noindex"`
	PostalCode    string `datastore:"postal_code,noindex"`
	Country       string `datastore:"country,noindex"`
	Directions    string `datastore:"directions,noindex"`

	// Contact kind.
	ContactMethod string `datastore:"contact_method,noindex"`
	ContactType   string `datastore:"contact_type,noindex"`
	ContactText   string `datastore:"contact_text,noindex"`

	// Calendar kind.
	FirstOccurrence time.Time `datastore:"first_occurrence,noindex"`
	Frequency       string    `datastore:"frequency,noindex"`
	Occasion        string    `datastore:"occasion,noindex"`
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

func (person *Entity) enabledText() string {
	return enabledText(person.Enabled)
}

func (person *Entity) editUrl() string {
	// Include origin for a fully qualified URL.
	return fmt.Sprintf("%s/?action=edit&kind=%s&key=%s",
		defaultVersionOrigin,
		person.Key.Kind,
		person.Key.Encode(),
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

func renderPersonView(w io.Writer, client *datastore.Client, person *Entity) {
	name := html.EscapeString(person.displayName())
	comments := html.EscapeString(person.Comments)
	fmt.Fprintf(w, `
		<hr>
		<a href="%s" class="edit-link">Edit</a>
		<span class="thing">%s</span> <span class="tag">(%s) [%s]</span><br>
		<div class="comments">%s</div>
		<div class="indent">
	`, person.editUrl(),
		name,
		person.Category,
		person.enabledText(),
		comments)

	var contacts []Entity
	query := datastore.NewQuery("Contact").Ancestor(person.Key)
	_, err := client.GetAll(ctx, query, &contacts)
	if err != nil {
		log.Fatalf("Failed to get all: %v", err)
	}

	for _, contact := range contacts {
		renderContactView(w, &contact)
	}

	var addresses []Entity
	query = datastore.NewQuery("Address").Ancestor(person.Key)
	_, err = client.GetAll(ctx, query, &addresses)
	if err != nil {
		log.Fatalf("Failed to get all: %v", err)
	}

	for _, address := range addresses {
		renderAddressView(w, &address)
	}

	var events []Entity
	query = datastore.NewQuery("Calendar").Ancestor(person.Key)
	_, err = client.GetAll(ctx, query, &events)
	if err != nil {
		log.Fatalf("Failed to get all: %v", err)
	}

	for _, event := range events {
		renderCalendarView(w, &event)
	}

	fmt.Fprintf(w, `
		</div>`)
}

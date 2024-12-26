package main

import (
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"

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

type Person struct {
	Key *datastore.Key `datastore:"__key__"`

	Category    string `datastore:"category,noindex"`
	SendCard    bool   `datastore:"send_card,noindex"`
	Title       string `datastore:"title,noindex"`
	MailingName string `datastore:"mailing_name,noindex"`
	FirstName   string `datastore:"first_name,noindex"`
	LastName    string `datastore:"last_name,noindex"`
	CompanyName string `datastore:"company_name,noindex"`

	Comments string   `datastore:"comments,noindex"`
	Enabled  bool     `datastore:"enabled,noindex"`
	Words    []string `datastore:"words,noindex"`
}

func requestToPerson(r *http.Request, client *datastore.Client) (person *Person, err error) {
	// kind := r.URL.Query().Get("kind")
	key := r.URL.Query().Get("key")
	dbkey, err := datastore.DecodeKey(key)
	var p Person
	err = client.Get(ctx, dbkey, &p)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to get person: %v", err))
	}
	return &p, nil
}

func (person *Person) enabledText() string {
	return enabledText(person.Enabled)
}

func (person *Person) editUrl() string {
	// Include origin for a fully qualified URL.
	return fmt.Sprintf("%s/?action=edit&kind=%s&key=%s",
		defaultVersionOrigin,
		person.Key.Kind,
		person.Key.Encode(),
	)
}

func (p *Person) displayName() string {
	t := ""
	if p.MailingName != "" {
		t += fmt.Sprintf("[%s] ", p.MailingName)
	}
	if p.CompanyName != "" {
		t += fmt.Sprintf("%s ", p.CompanyName)
	}
	if p.Title != "" {
		t += p.Title + " "
	}
	if p.FirstName != "" {
		t += p.FirstName + " "
	}
	if p.LastName != "" {
		t += p.LastName
	}
	return t
}

func renderPersonView(w io.Writer, client *datastore.Client, person *Person) {
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

	var contacts []Contact
	query := datastore.NewQuery("Contact").Ancestor(person.Key)
	_, err := client.GetAll(ctx, query, &contacts)
	if err != nil {
		log.Fatalf("Failed to get all: %v", err)
	}

	for _, contact := range contacts {
		renderContactView(w, &contact)
	}

	var addresses []Address
	query = datastore.NewQuery("Address").Ancestor(person.Key)
	_, err = client.GetAll(ctx, query, &addresses)
	if err != nil {
		log.Fatalf("Failed to get all: %v", err)
	}

	for _, address := range addresses {
		renderAddressView(w, &address)
	}

	var events []Calendar
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

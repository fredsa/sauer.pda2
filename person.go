package main

import (
	"errors"
	"fmt"
	"html"
	"io"

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

func renderPersonView(w io.Writer, client *datastore.Client, person *Entity) error {
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
		return errors.New(fmt.Sprintf("Failed to get all contacts: %v", err))
	}

	for _, contact := range contacts {
		renderContactView(w, &contact)
	}

	var addresses []Entity
	query = datastore.NewQuery("Address").Ancestor(person.Key)
	_, err = client.GetAll(ctx, query, &addresses)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to get all addresses: %v", err))
	}

	for _, address := range addresses {
		renderAddressView(w, &address)
	}

	var events []Entity
	query = datastore.NewQuery("Calendar").Ancestor(person.Key)
	_, err = client.GetAll(ctx, query, &events)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to get all calendar events: %v", err))
	}

	for _, event := range events {
		renderCalendarView(w, &event)
	}

	fmt.Fprintf(w, `
		</div>`)

	return nil
}

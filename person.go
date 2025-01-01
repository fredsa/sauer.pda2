package main

import (
	"context"
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

func renderPersonView(w io.Writer, ctx context.Context, client *datastore.Client, person *Entity) error {
	var children []Entity
	query := datastore.NewQuery("").Ancestor(person.Key)
	// query = query.FilterField("__key__", ">", person.Key)
	_, err := client.GetAll(ctx, query, &children)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to get all entities: %v", err))
	}

	for _, child := range children {
		switch child.Key.Kind {
		case "Person":
			name := html.EscapeString(person.displayName())
			comments := html.EscapeString(person.Comments)
			fmt.Fprintf(w, `
				<hr>
				<a href="%s" class="edit-link">Edit</a>
				<span class="thing">%s</span> <span class="tag">(%s) [%s]</span><br>
				<div class="comments">%s</div>
				<div class="indent">
			`, person.editURL(),
				name,
				person.Category,
				person.enabledText(),
				comments)
		case "Contact":
			renderContactView(w, &child)
		case "Address":
			renderAddressView(w, &child)
		case "Calendar":
			renderCalendarView(w, &child)
		default:
			return errors.New(fmt.Sprintf("Unknown kind: %s", child.Key.Kind))
		}
	}

	fmt.Fprintf(w, `
		</div>`)

	return nil
}

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"

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

func renderPersonView(ctx context.Context, client *datastore.Client, person *Entity) (string, error) {
	var buffer bytes.Buffer

	var children []Entity
	query := datastore.NewQuery("").Ancestor(person.Key)
	// query = query.FilterField("__key__", ">", person.Key)
	_, err := client.GetAll(ctx, query, &children)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Failed to get all entities: %v", err))
	}

	for _, child := range children {
		switch child.Key.Kind {
		case "Person":
			name := html.EscapeString(person.displayName())
			comments := html.EscapeString(person.Comments)
			buffer.WriteString(fmt.Sprintf(`
				<hr>
				<div class="%s">
					<a href="%s" class="edit-link">Edit</a>
					<span class="thing">%s</span> <span class="tag">(%s) [%s] %s</span><br>
					<div class="comments">%s</div>
					<div class="indent">
			`,
				person.enabledClass(),
				person.editURL(),
				name,
				person.Category,
				person.enabledText(),
				person.sendCardText(),
				comments))
		case "Contact":
			buffer.WriteString(contactView(&child))
		case "Address":
			buffer.WriteString(addressView(&child))
		case "Calendar":
			buffer.WriteString(calendarView(&child))
		default:
			return "", errors.New(fmt.Sprintf("Unknown kind: %s", child.Key.Kind))
		}
	}

	buffer.WriteString(fmt.Sprintf(`
			</div>
		</div>
	`))

	return buffer.String(), nil
}

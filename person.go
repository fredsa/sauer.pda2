package main

import (
	"fmt"
	"html"
	"io"
	"log"

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

func renderPersonForm(w io.Writer, client *datastore.Client, person *Entity) {
	fmt.Fprintf(w, `
		<hr>
		<form name="personform" method="post" action=".">
			<input type="hidden" name="action" value="edit">
			<input type="hidden" name="kind" value="%s">
			<input type="hidden" name="key" value="%s">
			<table>
	`, person.Key.Kind, person.maybeKey())

	formFields(w, person)
	// props = Person.properties()
	// self.formFields(person)
	fmt.Fprintf(w, `<tr><td></td><td><input type="submit" name="updated" value="Save Changes" style="margin-top: 1em;"></td></tr>`)
	propname := "category"
	fmt.Fprintf(w, `
			</table>
		</form>
		<script>
			document.personform.%s.focus();
		</script>
		<hr>
`, propname)
	if person.maybeKey() != "" {
		fmt.Fprintf(w, `
			<a href="?action=create&kind=Contact&parent_key=%s">[+Contact]</a>
			&nbsp;
			<a href="?action=create&kind=Address&parent_key=%s">[+Address]</a>
			&nbsp;
			<a href="?action=create&kind=Calendar&parent_key=%s">[+Calendar]</a>
	  	`, person.Key.Encode(), person.Key.Encode(), person.Key.Encode())
	}
}

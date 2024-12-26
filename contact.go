package main

import (
	"fmt"
	"html"
	"io"
	"strings"

	"cloud.google.com/go/datastore"
)

type Contact struct {
	Key *datastore.Key `datastore:"__key__"`

	ContactMethod string `datastore:"contact_method,noindex"`
	ContactType   string `datastore:"contact_type,noindex"`
	ContactText   string `datastore:"contact_text,noindex"`

	Comments string   `datastore:"comments,noindex"`
	Enabled  bool     `datastore:"enabled,noindex"`
	Words    []string `datastore:"words,noindex"`
}

func (contact *Contact) enabledText() string {
	return enabledText(contact.Enabled)
}

func (contact *Contact) editUrl() string {
	// Include origin for a fully qualified URL.
	return fmt.Sprintf("%s/?action=edit&kind=%s&key=%s",
		defaultVersionOrigin,
		contact.Key.Kind,
		contact.Key.Encode(),
	)
}

func renderContactView(w io.Writer, contact *Contact) {
	text := html.EscapeString(contact.ContactText)
	if strings.HasPrefix(text, "http") {
		text = fmt.Sprintf(`
			<a href="%s" target="_blank">%s</a>
		`, text, text)
	}

	clazz := ""
	if !contact.Enabled {
		clazz = "disabled"
	}
	fmt.Fprintf(w, `
		<div class="%s">
		<a href="%s" class="edit-link">Edit</a>
		<span class="thing %s">%s</span>
	`, clazz,
		contact.editUrl(),
		contact.Key.Kind,
		text)

	fmt.Fprintf(w, `
		<span class="tag">(%s %s) [%s]</span><br>
		<div class="comments">%s</div>
		</div>
	`, contact.ContactMethod,
		contact.ContactType,
		contact.enabledText(),
		html.EscapeString(contact.Comments),
	)
}

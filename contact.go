package main

import (
	"fmt"
	"html"
	"io"
	"strings"
)

func renderContactView(w io.Writer, contact *Entity) error {
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

	return nil
}

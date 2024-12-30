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

func renderContactForm(w io.Writer, contact *Entity) error {
	fmt.Fprintf(w, `
		<hr>
		<form name="contactform" method="post" action=".">
		<input type="hidden" name="action" value="edit">
		<input type="hidden" name="kind" value="%s">
		<input type="hidden" name="key" value="%s">
		<input type="hidden" name="parent_key" value="%s">
		<table>
	`, contact.Key.Kind, contact.maybeKey(), contact.Key.Parent.Encode())

	formFields(w, contact)
	fmt.Fprintf(w, `<tr><td></td><td><input type="submit" name="updated" value="Save" style="margin-top: 1em;"></td></tr>`)
	fmt.Fprintf(w, `
		</table>
		</form>
		<hr>
	`)

	return nil
}

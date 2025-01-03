package main

import (
	"bytes"
	"fmt"
	"html"
	"strings"
)

func contactView(contact *Entity) string {
	var buffer bytes.Buffer

	text := html.EscapeString(contact.ContactText)
	if strings.HasPrefix(text, "http") {
		text = fmt.Sprintf(`
			<a href="%s" target="_blank">%s</a>
		`, text, text)
	}

	buffer.WriteString(fmt.Sprintf(`
		<div class="%s">
			<a href="%s" class="edit-link">Edit</a>
			<span class="thing %s">%s</span>
	`,
		contact.enabledClass(),
		contact.editURL(),
		contact.Key.Kind,
		text))

	buffer.WriteString(fmt.Sprintf(`
			<span class="tag">(%s %s) [%s]</span><br>
			<div class="comments">%s</div>
		</div>
	`, contact.ContactMethod,
		contact.ContactType,
		contact.enabledText(),
		html.EscapeString(contact.Comments),
	))

	return buffer.String()
}

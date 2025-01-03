package main

import (
	"bytes"
	"fmt"

	"golang.org/x/net/html"
)

func calendarView(calendar *Entity) string {
	var buffer bytes.Buffer

	formattedDate := calendar.FirstOccurrence.Format("2006-01-02")
	buffer.WriteString(fmt.Sprintf(`
		<div class="%s">
		<a href="%s" class="edit-link">Edit</a>
		<span class="thing %s">%s</span> <span class="tag">(%s %s) [%s]</span><br>
		<div class="comments">%s</div>
		</div>
	`,
		calendar.enabledClass(),
		calendar.editURL(),
		calendar.Key.Kind,
		formattedDate,
		calendar.Frequency,
		html.EscapeString(calendar.Occasion),
		calendar.enabledText(),
		html.EscapeString(calendar.Comments),
	))

	return buffer.String()
}

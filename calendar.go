package main

import (
	"fmt"
	"io"

	"golang.org/x/net/html"
)

func renderCalendarView(w io.Writer, calendar *Entity) error {
	clazz := ""
	if !calendar.Enabled {
		clazz = "disabled"
	}

	formattedDate := calendar.FirstOccurrence.Format("2006-01-02")
	fmt.Fprintf(w, `
		<div class="%s">
		<a href="%s" class="edit-link">Edit</a>
		<span class="thing %s">%s</span> <span class="tag">(%s %s) [%s]</span><br>
		<div class="comments">%s</div>
		</div>
	`,
		clazz,
		calendar.editUrl(),
		calendar.Key.Kind,
		formattedDate,
		calendar.Frequency,
		html.EscapeString(calendar.Occasion),
		calendar.enabledText(),
		html.EscapeString(calendar.Comments),
	)

	return nil
}

package main

import (
	"fmt"
	"io"

	"golang.org/x/net/html"
)

func renderCalendarView(w io.Writer, calendar *Entity) {
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
}

func renderCalendarForm(w io.Writer, calendar *Entity) {
	fmt.Fprintf(w, `
		<hr>
		<form name="calendarform" method="post" action=".">
		<input type="hidden" name="action" value="edit">
		<input type="hidden" name="kind" value="%s">
		<input type="hidden" name="modified" value="true">
		<input type="hidden" name="key" value="%s">
		<input type="hidden" name="parent_key" value="%s">
		<table>
	`, calendar.Key.Kind, calendar.maybeKey(), calendar.Key.Parent.Encode())

	formFields(w, calendar)
	fmt.Fprintf(w, `<tr><td></td><td><input type="submit" name="updated" value="Save Changes" style="margin-top: 1em;"></td></tr>`)
	fmt.Fprintf(w, `
		</table>
		</form>
		<hr>
	`)
}

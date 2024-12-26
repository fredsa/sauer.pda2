package main

import (
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/datastore"
	"golang.org/x/net/html"
)

type Calendar struct {
	Key *datastore.Key `datastore:"__key__"`

	FirstOccurrence time.Time `datastore:"first_occurrence,noindex"`
	Frequency       string    `datastore:"frequency,noindex"`
	Occasion        string    `datastore:"occasion,noindex"`

	Comments string   `datastore:"comments,noindex"`
	Enabled  bool     `datastore:"enabled,noindex"`
	Words    []string `datastore:"words,noindex"`
}

func (calendar *Calendar) enabledText() string {
	return enabledText(calendar.Enabled)
}

func (calendar *Calendar) editUrl() string {
	// Include origin for a fully qualified URL.
	return fmt.Sprintf("%s/?action=edit&kind=%s&key=%s",
		defaultVersionOrigin,
		calendar.Key.Kind,
		calendar.Key.Encode(),
	)
}

func renderCalendarView(w io.Writer, calendar *Calendar) {
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

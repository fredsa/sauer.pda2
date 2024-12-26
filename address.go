package main

import (
	"fmt"
	"html"
	"io"
	"strings"

	"cloud.google.com/go/datastore"
)

type Address struct {
	Key *datastore.Key `datastore:"__key__"`

	AddressType   string `datastore:"address_type,noindex"`
	AddressLine1  string `datastore:"address_line1,noindex"`
	AddressLine2  string `datastore:"address_line2,noindex"`
	City          string `datastore:"city,noindex"`
	StateProvince string `datastore:"state_province,noindex"`
	PostalCode    string `datastore:"postal_code,noindex"`
	Country       string `datastore:"country,noindex"`
	Directions    string `datastore:"directions,noindex"`

	Comments string   `datastore:"comments,noindex"`
	Enabled  bool     `datastore:"enabled,noindex"`
	Words    []string `datastore:"words,noindex"`
}

func (address *Address) enabledText() string {
	return enabledText(address.Enabled)
}

func (address *Address) editUrl() string {
	// Include origin for a fully qualified URL.
	return fmt.Sprintf("%s/?action=edit&kind=%s&key=%s",
		defaultVersionOrigin,
		address.Key.Kind,
		address.Key.Encode(),
	)
}

func (address *Address) snippet() string {
	s := strings.Join([]string{
		address.AddressLine1,
		address.AddressLine2,
		address.City,
		address.StateProvince,
		address.PostalCode,
		address.Country},
		" / ")
	return strings.ReplaceAll(s, "/  /", "/")
}

func renderAddressView(w io.Writer, address *Address) {
	qlocation := strings.ReplaceAll(address.snippet(), " / ", " ")
	mapsURL := "https://maps.google.com/?q=" + qlocation

	clazz := ""
	if !address.Enabled {
		clazz = "disabled"
	}
	fmt.Fprintf(w, `
		<div class="%s">
		<a href="%s" class="edit-link">Edit</a>

		<span class="thing %s">%s</span>
		<a href="%s" target="_blank">[Google Maps]</a>&nbsp;&nbsp;<a href="%s" target="_blank">[directions]</a>
		<span class="tag" target="_blank">(%s) [%s]</span><br>

		<div class="comments">%s</div>
		</div>
	`,
		clazz,
		address.editUrl(),

		address.Key.Kind,
		html.EscapeString(address.snippet()),
		mapsURL,
		address.AddressType,
		address.enabledText(),

		html.EscapeString(address.Directions),
		html.EscapeString(address.Comments),
	)
}

package main

import (
	"fmt"
	"html"
	"io"
	"strings"
)

func (address *Entity) snippet() string {
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

func renderAddressView(w io.Writer, address *Entity) {
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
			<a href="%s" target="_blank">[Google Maps]</a></a>
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

		html.EscapeString(address.Comments),
	)
}

func renderAddressForm(w io.Writer, address *Entity) {
	fmt.Fprintf(w, `
		<hr>
		<form name="addressform" method="post" action=".">
		<input type="hidden" name="action" value="edit">
		<input type="hidden" name="kind" value="%s">
		<input type="hidden" name="key" value="%s">
		<input type="hidden" name="parent_key" value="%s">
		<table>
	`, address.Key.Kind, address.maybeKey(), address.Key.Parent.Encode())

	formFields(w, address)
	fmt.Fprintf(w, `<tr><td></td><td><input type="submit" name="updated" value="Save Changes" style="margin-top: 1em;"></td></tr>`)
	fmt.Fprintf(w, `
		</table>
		</form>
		<hr>
	`)
}

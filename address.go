package main

import (
	"bytes"
	"fmt"
	"html"
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

func addressView(address *Entity) string {
	var buffer bytes.Buffer

	qlocation := strings.ReplaceAll(address.snippet(), " / ", " ")
	mapsURL := "https://maps.google.com/?q=" + qlocation

	buffer.WriteString(fmt.Sprintf(`
		<div class="%s">
			<a href="%s" class="edit-link">Edit</a>

			<span class="thing %s">%s</span>
			<a href="%s" target="_blank">[Google Maps]</a></a>
			<span class="tag" target="_blank">(%s) [%s]</span><br>

			<div class="comments">%s</div>
		</div>
	`,
		address.enabledClass(),
		address.editURL(),

		address.Key.Kind,
		html.EscapeString(address.snippet()),
		mapsURL,
		address.AddressType,
		address.enabledText(),

		html.EscapeString(address.Comments),
	))

	return buffer.String()
}

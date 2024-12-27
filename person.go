package main

import (
	"fmt"
	"html"
	"io"
	"log"
	"reflect"
	"time"

	"cloud.google.com/go/datastore"
)

var categories = []string{
	"(Unspecified)",
	"Relatives",
	"Personal",
	"Hotel/Restaurant/Entertainment",
	"Services by Individuals",
	"Companies, Institutions, etc.",
	"Business Relations",
}

func renderPersonView(w io.Writer, client *datastore.Client, person *Entity) {
	name := html.EscapeString(person.displayName())
	comments := html.EscapeString(person.Comments)
	fmt.Fprintf(w, `
		<hr>
		<a href="%s" class="edit-link">Edit</a>
		<span class="thing">%s</span> <span class="tag">(%s) [%s]</span><br>
		<div class="comments">%s</div>
		<div class="indent">
	`, person.editUrl(),
		name,
		person.Category,
		person.enabledText(),
		comments)

	var contacts []Entity
	query := datastore.NewQuery("Contact").Ancestor(person.Key)
	_, err := client.GetAll(ctx, query, &contacts)
	if err != nil {
		log.Fatalf("Failed to get all: %v", err)
	}

	for _, contact := range contacts {
		renderContactView(w, &contact)
	}

	var addresses []Entity
	query = datastore.NewQuery("Address").Ancestor(person.Key)
	_, err = client.GetAll(ctx, query, &addresses)
	if err != nil {
		log.Fatalf("Failed to get all: %v", err)
	}

	for _, address := range addresses {
		renderAddressView(w, &address)
	}

	var events []Entity
	query = datastore.NewQuery("Calendar").Ancestor(person.Key)
	_, err = client.GetAll(ctx, query, &events)
	if err != nil {
		log.Fatalf("Failed to get all: %v", err)
	}

	for _, event := range events {
		renderCalendarView(w, &event)
	}

	fmt.Fprintf(w, `
		</div>`)
}

func formFields(w io.Writer, entity *Entity) {
	t := reflect.TypeOf(entity).Elem()
	v := reflect.ValueOf(entity).Elem()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		forkind := field.Tag.Get("forkind")
		if forkind != "" && forkind != entity.Key.Kind {
			// fmt.Fprintf(w, `<tr><td style="vertical-align: top; text-align: right;">SKIP:::  forkind=%s!=%s</td><td>%s=%s</td></tr>`, forkind, entity.Key.Kind, field.Name, value)
			continue
		}

		label := field.Name
		html := "?"
		color := "pink"
		if field.Tag.Get("form") == "textarea" {
			html = fmt.Sprintf(`<textarea name="%s" style="width: 50em; height: 20em; font-family: monospace;">%s</textarea>`, field.Name, value)
		} else if field.Type.Kind() == reflect.String {
			html = fmt.Sprintf(`<input type="text" style="width: 50em;" name="%s" value="%s">`, field.Name, value)
		} else if field.Type.Kind() == reflect.Bool {
			checked := ""
			if value.Bool() {
				checked = "checked"
			}
			html = fmt.Sprintf(`<input type="checkbox" name="%s" %s> %s`, field.Name, checked, label)
			label = ""
		} else if field.Type == reflect.TypeOf(time.Time{}) {
			datevalue := value.Interface().(time.Time)
			date := ""
			if !datevalue.IsZero() {
				date = datevalue.Format("2006-01-02")
			}
			html = fmt.Sprintf(`<input type="text" style="width: 8em;" name="%s" value="%s">`, field.Name, date)
		} else {
			html = fmt.Sprintf("<div>%v %v=%v</div>", field.Type, label, value)
		}

		if field.Name == "words" {
			color = "red"
		} else {
			color = "blue"
		}
		fmt.Fprintf(w, `<tr><td style="vertical-align: top; text-align: right; color: %s;">%s</td><td>%s</td></tr>`, color, label, html)

	}

	//	  if isinstance(prop, SelectableStringProperty):
	//		values = prop.choices
	//		html = `<select name="%s" size="%s">` % (propname, len(values))
	//		for v in values:
	//		  selected = "selected" if value == v else ""
	//		  html += `<option %s value="%s">%s</option>` % (selected, v, v)
	//		html += `</select>`
	//	  elif isinstance(prop, db.StringListProperty):
	//		#html = `<textarea name="%s" style="width: 50em; height: 4em; color: gray;">%s</textarea>` % (propname, ", ".join(value))
	//		html = `<code style="color:#ddd;">%s</code>` % " ".join(value)
	//	  else:
	//		html = `<span style="color:red;">** Unknown property type '%s' for '%s' **</span>` % (prop.__class__.__name__, propname)
}

func renderPersonForm(w io.Writer, client *datastore.Client, person *Entity) {
	fmt.Fprintf(w, `
	<hr>
	<form name="personform" method="post" action=".">
	<input type="hidden" name="action" value="edit">
	<input type="hidden" name="kind" value="%s">
	<input type="hidden" name="modified" value="true">
	<input type="hidden" name="key" value="%s">
	<table>
	`, person.Key.Kind, person.maybeKey())

	formFields(w, person)
	// props = Person.properties()
	// self.formFields(person)
	fmt.Fprintf(w, `<tr><td></td><td><input type="submit" name="updated" value="Save Changes" style="margin-top: 1em;"></td></tr>`)
	propname := "category"
	fmt.Fprintf(w, `
		</table>
		</form>
		<script>
		document.personform.%s.focus();
		</script>
		<hr>
`, propname)
	if person.maybeKey() != "" {
		fmt.Fprintf(w, `
			<a href="?action=create&kind=Contact&parent_key=%s">[+Contact]</a>
			&nbsp;
			<a href="?action=create&kind=Address&parent_key=%s">[+Address]</a>
			&nbsp;
			<a href="?action=create&kind=Calendar&parent_key=%s">[+Calendar]</a>
	  	`, person.Key.Encode(), person.Key.Encode(), person.Key.Encode())
	}
}

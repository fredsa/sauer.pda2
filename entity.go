package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
)

var kinds = []string{"Person", "Address", "Contact", "Calendar"}

// https://cloud.google.com/go/docs/reference/cloud.google.com/go/datastore/latest
type Entity struct {
	Key *datastore.Key `forkind:"hidden" datastore:"__key__"`

	// Person kind.
	Category    string `forkind:"Person" datastore:"category,omitempty" form:"select"`
	SendCard    bool   `forkind:"Person" datastore:"send_card,omitempty" default:"false"` // Default false.
	Title       string `forkind:"Person" datastore:"title,omitempty"`
	MailingName string `forkind:"Person" datastore:"mailing_name,omitempty"`
	FirstName   string `forkind:"Person" datastore:"first_name,omitempty"`
	LastName    string `forkind:"Person" datastore:"last_name,omitempty"`
	CompanyName string `forkind:"Person" datastore:"company_name,omitempty"`

	// Address kind.
	AddressType   string `forkind:"Address" datastore:"address_type,omitempty" form:"select"`
	AddressLine1  string `forkind:"Address" datastore:"address_line1,omitempty"`
	AddressLine2  string `forkind:"Address" datastore:"address_line2,omitempty"`
	City          string `forkind:"Address" datastore:"city,omitempty"`
	StateProvince string `forkind:"Address" datastore:"state_province,omitempty"`
	PostalCode    string `forkind:"Address" datastore:"postal_code,omitempty"`
	Country       string `forkind:"Address" datastore:"country,omitempty"`

	// Contact kind.
	ContactMethod string `forkind:"Contact" datastore:"contact_method,omitempty" form:"select"`
	ContactType   string `forkind:"Contact" datastore:"contact_type,omitempty" form:"select"`
	ContactText   string `forkind:"Contact" datastore:"contact_text,omitempty"`

	// Calendar kind.
	FirstOccurrence time.Time `forkind:"Calendar" datastore:"first_occurrence,omitempty" hint:"YYYY-MM-DD"`
	Frequency       string    `forkind:"Calendar" datastore:"frequency,omitempty" form:"select"`
	Occasion        string    `forkind:"Calendar" datastore:"occasion,omitempty"`

	// Common fields.
	Comments string   `forkind:"" datastore:"comments,omitempty,noindex" form:"textarea"` // Not indexed.
	Enabled  bool     `forkind:"" datastore:"enabled" default:"true"`                     // Default true.
	Words    []string `forkind:"hidden" datastore:"words,omitempty" json:"-"`
}

var choices = map[string][]string{
	"Category": {
		"(Unspecified)",
		"Relatives",
		"Personal",
		"Hospitality",  //"Hotel/Restaurant/Entertainment",
		"Freelance",    //"Services by Individuals",
		"Company",      //"Companies, Institutions, etc.",
		"Professional", //"Business Relations",
	},
	"AddressType": {
		"(Unspecified)",
		"Home",
		"Business",
	},
	"ContactMethod": {
		"(Unspecified)",
		"Personal",
		"Business",
	},
	"ContactType": {
		"(Unspecified)",
		"Voice",
		"Data",
		"Email",
		"Mobile",
		"URL",
		"Facsimile",
	},
	"Frequency": {
		"Annual",
	},
}

func requestToEntity(r *http.Request, ctx context.Context, client *datastore.Client) (entity *Entity, err error) {
	key := getValue(r, "key")
	dbkey, err := datastore.DecodeKey(key)

	if r.Method == "POST" {
		e := &Entity{}

		t := reflect.TypeOf(e).Elem()
		v := reflect.ValueOf(e).Elem()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			value := v.Field(i)

			v := r.Form.Get(field.Name)
			if field.Name == "Key" {
				value.Set(reflect.ValueOf(dbkey))
			} else if field.Tag.Get("forkind") == "hidden" {
				// Skip.
				continue
			} else if field.Type.Kind() == reflect.Bool {
				value.SetBool(v != "")
			} else if field.Type == reflect.TypeOf(time.Time{}) {
				t := time.Time{}
				if v != "" {
					t, err = time.Parse("2006-01-02", v)
					if err != nil {
						return nil, errors.New(fmt.Sprintf("Failed to parse date %q: %v", v, err))
					}
				}
				// log.Printf("DATE: %s == %v", field.Name, t)
				value.Set(reflect.ValueOf(t))
			} else if field.Tag.Get("form") == "select" {
				value.SetString(v)
			} else {
				value.SetString(v)
			}
		}

		return e, nil
	} else {
		var e Entity
		err = client.Get(ctx, dbkey, &e)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to get %s: %v", dbkey, err))
		}

		return &e, nil
	}
}

func (entity *Entity) words() []string {
	// Map prevents duplicate results.
	results := make(map[string]struct{})
	t := reflect.TypeOf(entity).Elem()
	v := reflect.ValueOf(entity).Elem()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)
		if value.String() == "" {
			// Skip.
			continue
		} else if field.Tag.Get("forkind") == "hidden" {
			// Skip.
			continue
		} else if field.Type.Kind() == reflect.Bool {
			// Only act on true as unset fields will appear to be false.
			if value.Bool() {
				word := fmt.Sprintf("%s=%v", field.Name, value.Bool())
				// log.Printf("BOOL: %s == %v", field.Name, value)
				results[word] = struct{}{}
			}
		} else if field.Type == reflect.TypeOf(time.Time{}) {
			datevalue := value.Interface().(time.Time)
			if !datevalue.IsZero() {
				word := datevalue.Format("2006-01-02")
				// log.Printf("DATE: %s == %v", field.Name, word)
				results[word] = struct{}{}
			}
		} else if field.Tag.Get("form") == "select" {
			if value.String() != "" {
				word := fmt.Sprintf("%s=%s", field.Name, strings.Trim(value.String(), "()"))
				// log.Printf("SELECT: %s", word)
				results[word] = struct{}{}
			}
		} else {
			newwords := WORDS_RE.Split(value.String(), -1)
			for _, word := range newwords {
				// log.Printf("%s: %q", field.Name, word)
				results[word] = struct{}{}
				if strings.Contains(word, "=") {
					subwords := strings.Split(word, "=")
					for _, sw := range subwords {
						// log.Printf("%s: %q", field.Name, sw)
						results[sw] = struct{}{}
					}
				}
			}
		}
	}

	words := make([]string, 0, len(results))
	for word := range results {
		words = append(words, strings.ToLower(word))
	}
	return words
}

func (entity *Entity) fix() {
	// After other fixes, lastly.
	entity.Words = entity.words()
}

func (entity *Entity) save(ctx context.Context, client *datastore.Client) (*datastore.Key, error) {
	key, err := client.Put(ctx, entity.Key, entity)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to put entity: %v", err))
	}

	return key, nil
}

func (entity *Entity) enabledClass() string {
	if entity.Enabled {
		return ""
	} else {
		return "disabled"
	}
}

func (entity *Entity) enabledText() string {
	if entity.Enabled {
		return "Enabled"
	} else {
		return "DISABLED"
	}
}

func (entity *Entity) sendCardText() string {
	if entity.SendCard {
		return "[SendCard]"
	} else {
		return ""
	}
}

func (entity *Entity) actionURL(action string) string {
	// Include origin for a fully qualified URL.
	return fmt.Sprintf("%s/?action=%s&key=%s",
		defaultVersionOrigin(),
		action,
		entity.Key.Encode(),
	)
}

func (entity *Entity) viewURL() string {
	return entity.actionURL("view")
}

func (entity *Entity) editURL() string {
	return entity.actionURL("edit")
}

func (person *Entity) displayName() string {
	t := ""
	if person.MailingName != "" {
		t += fmt.Sprintf("[%s] ", person.MailingName)
	}
	if person.CompanyName != "" {
		t += fmt.Sprintf("%s ", person.CompanyName)
	}
	if person.Title != "" {
		t += person.Title + " "
	}
	if person.FirstName != "" {
		t += person.FirstName + " "
	}
	if person.LastName != "" {
		t += person.LastName
	}
	return strings.TrimSpace(t)
}

func form(ctx context.Context, entity *Entity) string {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf(`
		<hr>
		<form name="myform" method="post" action=".">
			<input type="hidden" name="action" value="edit">
			<table>
	`))
	buffer.WriteString(formFields(ctx, entity))
	buffer.WriteString(fmt.Sprintf(`
				<tr><td></td><td><input type="submit" name="updated" value="Save" style="margin-top: 1em;"></td></tr>
	`))
	buffer.WriteString(fmt.Sprintf(`
			</table>
		</form>
		<hr>
	`))

	if !entity.Key.Incomplete() && entity.Key.Kind == "Person" {
		for _, kind := range kinds {
			if kind != entity.Key.Kind {
				buffer.WriteString(createEntityLink(kind, entity.Key))
			}
		}
	}

	return buffer.String()
}

func createEntityLink(kind string, parentKey *datastore.Key) string {
	childkey := datastore.IDKey(kind, 0, parentKey)
	return fmt.Sprintf(`<a href="?action=create&key=%s">[+%s]</a>&nbsp;`, childkey.Encode(), kind)
}

func keyLiteral(key *datastore.Key) string {
	t := ""
	for key != nil {
		if t != "" {
			t = ", " + t
		}
		if key.Incomplete() {
			t = "<incomplete>" + t
		}
		if key.Name != "" {
			t = fmt.Sprintf("%s, '%s'", key.Kind, key.Name) + t
		} else {
			t = fmt.Sprintf("%s, %d", key.Kind, key.ID) + t
		}
		key = key.Parent
	}
	return fmt.Sprintf("Key(%s)", t)
}

func formFields(ctx context.Context, entity *Entity) string {
	var buffer bytes.Buffer

	t := reflect.TypeOf(entity).Elem()
	v := reflect.ValueOf(entity).Elem()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)
		label := field.Name
		html := "?"
		color := "blue"
		if field.Name == "Words" {
			color = "red"
		}

		forkind := field.Tag.Get("forkind")
		hint := field.Tag.Get("hint")
		if field.Name == "Key" {
			if !isAdmin(ctx) {
				continue
			}
			color = "gray"
			html = fmt.Sprintf(`
					<input type="hidden" name="key" value="%s">
					<code>%s<br>%s<br>%s<code>
				`,
				entity.Key.Encode(),
				value,
				keyLiteral(entity.Key),
				entity.Key.Encode(),
			)
		} else if forkind == "hidden" {
			if !isAdmin(ctx) {
				continue
			}
			color = "gray"
			html = fmt.Sprintf(`<code>%q</code>`, value)
		} else if forkind != "" && forkind != entity.Key.Kind {
			continue
			// color = "purple"
			// html = fmt.Sprintf(`[SKIP] forkind=%s!=%s %s=%s`, forkind, entity.Key.Kind, field.Name, value)
		} else if field.Tag.Get("form") == "textarea" {
			html = fmt.Sprintf(`<textarea name="%s">%s</textarea>`, field.Name, value)
		} else if field.Tag.Get("form") == "select" {
			values := choices[field.Name]
			html = fmt.Sprintf(`<select name="%s" size="%d">`, field.Name, len(values))
			for i, v := range values {
				selected := ""
				if value.String() == v {
					selected = "selected"
				} else if value.String() == "" && i == 0 {
					selected = "selected"
				}
				html += fmt.Sprintf(`<option %s value="%s">%s</option>`, selected, v, v)
			}
			html += `</select>`
		} else if field.Type.Kind() == reflect.String {
			html = fmt.Sprintf(`<input type="text" name="%s" value="%s" placeholder="%s">`, field.Name, value, hint)
		} else if field.Type.Kind() == reflect.Bool {
			val := false
			if !entity.Key.Incomplete() {
				val = value.Bool()
			} else {
				defval := field.Tag.Get("default")
				v, err := strconv.ParseBool(defval)
				if err != nil {
					log.Fatalf("Failed to parse bool %q: %v", defval, err)
				}
				val = v
			}

			checked := ""
			if val {
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
			html = fmt.Sprintf(`<input type="text" style="width: 8em;" name="%s" value="%s" placeholder="%s">`, field.Name, date, hint)
		} else {
			html = fmt.Sprintf("<div>%v %v=%v</div>", field.Type, label, value)
		}

		buffer.WriteString(fmt.Sprintf(`<tr style="color: %s;"><td style="vertical-align: top; text-align: right;">%s</td><td>%s</td></tr>`, color, label, html))
	}

	return buffer.String()
}

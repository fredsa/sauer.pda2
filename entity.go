package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
)

var choices = map[string][]string{
	"Category": {
		"(Unspecified)",
		"Relatives",
		"Personal",
		"Hotel/Restaurant/Entertainment",
		"Services by Individuals",
		"Companies, Institutions, etc.",
		"Business Relations",
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

var kinds = []string{"Person", "Address", "Contact", "Calendar"}

// https://cloud.google.com/go/docs/reference/cloud.google.com/go/datastore/latest
type Entity struct {
	Key *datastore.Key `forkind:"hidden" datastore:"__key__"`

	// Person kind.
	Category    string `forkind:"Person" datastore:"category,omitempty,noindex" form:"select"`
	SendCard    bool   `forkind:"Person" datastore:"send_card,omitempty,noindex" default:"false"` // Default false.
	Title       string `forkind:"Person" datastore:"title,omitempty,noindex"`
	MailingName string `forkind:"Person" datastore:"mailing_name,omitempty,noindex"`
	FirstName   string `forkind:"Person" datastore:"first_name,omitempty,noindex"`
	LastName    string `forkind:"Person" datastore:"last_name,omitempty,noindex"`
	CompanyName string `forkind:"Person" datastore:"company_name,omitempty,noindex"`

	// Address kind.
	AddressType   string `forkind:"Address" datastore:"address_type,omitempty,noindex" form:"select"`
	AddressLine1  string `forkind:"Address" datastore:"address_line1,omitempty,noindex"`
	AddressLine2  string `forkind:"Address" datastore:"address_line2,omitempty,noindex"`
	City          string `forkind:"Address" datastore:"city,omitempty,noindex"`
	StateProvince string `forkind:"Address" datastore:"state_province,omitempty,noindex"`
	PostalCode    string `forkind:"Address" datastore:"postal_code,omitempty,noindex"`
	Country       string `forkind:"Address" datastore:"country,omitempty,noindex"`

	// TODO: delete after purging from datastore.
	Directions string `forkind:"hidden" datastore:"directions,omitempty,noindex" form:"textarea"`

	// Contact kind.
	ContactMethod string `forkind:"Contact" datastore:"contact_method,omitempty,noindex" form:"select"`
	ContactType   string `forkind:"Contact" datastore:"contact_type,omitempty,noindex" form:"select"`
	ContactText   string `forkind:"Contact" datastore:"contact_text,omitempty,noindex"`

	// Calendar kind.
	FirstOccurrence time.Time `forkind:"Calendar" datastore:"first_occurrence,omitempty,noindex"`
	Frequency       string    `forkind:"Calendar" datastore:"frequency,omitempty,noindex" form:"select"`
	Occasion        string    `forkind:"Calendar" datastore:"occasion,omitempty,noindex"`

	// Common fields.
	Comments string   `forkind:"" datastore:"comments,omitempty,noindex" form:"textarea"`
	Enabled  bool     `forkind:"" datastore:"enabled" default:"true"` // Required. Indexed. Default true.
	Words    []string `forkind:"hidden" datastore:"words,omitempty"`  // Indexed.
}

func (entity *Entity) maybeKey() string {
	if entity.Key == nil {
		return ""
	} else {
		return entity.Key.Encode()
	}
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
		err := e.fixAndSave(ctx, client)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to save %v: %v", e, err))
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
	words := []string{}
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
			if value.Bool() {
				word := field.Name
				// log.Printf("BOOL: %s == %v", field.Name, value)
				words = append(words, word)
			}
		} else if field.Type == reflect.TypeOf(time.Time{}) {
			datevalue := value.Interface().(time.Time)
			if !datevalue.IsZero() {
				word := datevalue.Format("2006-01-02")
				// log.Printf("DATE: %s == %v", field.Name, word)
				words = append(words, word)
			}
		} else if field.Tag.Get("form") == "select" {
			if value.String() != "" {
				word := fmt.Sprintf("%s=%s", field.Name, value.String())
				// log.Printf("SELECT: %s", word)
				words = append(words, word)
			}
		} else {
			re := regexp.MustCompile(`\W+`)
			newwords := re.Split(value.String(), -1)
			// log.Printf("%s: %q", field.Name, newwords)
			words = append(words, newwords...)
		}
	}

	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return words
}

func (entity *Entity) fixAndSave(ctx context.Context, client *datastore.Client) error {
	entity.Words = entity.words()
	key, err := client.Put(ctx, entity.Key, entity)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to put entity: %v", err))
	}
	entity.Key = key

	return nil
}

func (entity *Entity) enabledText() string {
	return enabledText(entity.Enabled)
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
	return t
}

func renderForm(w io.Writer, ctx context.Context, entity *Entity) {
	fmt.Fprintf(w, `
		<hr>
		<form name="myform" method="post" action=".">
			<input type="hidden" name="action" value="edit">
			<table>
	`)
	renderFormFields(w, ctx, entity)
	fmt.Fprintf(w, `<tr><td></td><td><input type="submit" name="updated" value="Save" style="margin-top: 1em;"></td></tr>`)
	fmt.Fprintf(w, `
			</table>
		</form>
		<hr>
	`)

	if !entity.Key.Incomplete() && entity.Key.Kind == "Person" {
		for _, kind := range kinds {
			if kind != entity.Key.Kind {
				renderCreateEntity(w, kind, entity.Key)
			}
		}
	}
}

func renderCreateEntity(w io.Writer, kind string, parentKey *datastore.Key) {
	childkey := datastore.IDKey(kind, 0, parentKey)
	fmt.Fprintf(w, `<a href="?action=create&key=%s">[+%s]</a>&nbsp;`, childkey.Encode(), kind)
}

func renderFormFields(w io.Writer, ctx context.Context, entity *Entity) {
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
		if field.Name == "Key" {
			if !isAdmin(ctx) {
				continue
			}
			color = "gray"
			html = fmt.Sprintf(`
					<input type="hidden" name="key" value="%s">
					<code>%s<br>%s<code>
				`, entity.Key.Encode(), value, entity.Key.Encode())
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
			for _, v := range values {
				selected := ""
				if value.String() == v {
					selected = "selected"
				}
				html += fmt.Sprintf(`<option %s value="%s">%s</option>`, selected, v, v)
			}
			html += `</select>`
		} else if field.Type.Kind() == reflect.String {
			html = fmt.Sprintf(`<input type="text" name="%s" value="%s">`, field.Name, value)
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
			html = fmt.Sprintf(`<input type="text" style="width: 8em;" name="%s" value="%s">`, field.Name, date)
		} else {
			html = fmt.Sprintf("<div>%v %v=%v</div>", field.Type, label, value)
		}

		fmt.Fprintf(w, `<tr style="color: %s;"><td style="vertical-align: top; text-align: right;">%s</td><td>%s</td></tr>`, color, label, html)
	}
}

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/appengine/v2"
	"google.golang.org/appengine/v2/user"
)

// https://cloud.google.com/appengine/docs/standard/go/runtime#environment_variables
const PORT = "PORT"
const GOOGLE_CLOUD_PROJECT = "GOOGLE_CLOUD_PROJECT"
const GAE_APPLICATION = "GAE_APPLICATION" // App id, with prefix.
const GAE_ENV = "GAE_ENV"                 // `standard` in production.
const GAE_RUNTIME = "GAE_RUNTIME"         // Runtime in `app.yaml`.
const GAE_VERSION = "GAE_VERSION"         // App version.

var ADMINS_FREDSA = []string{"fredsa@gmail.com", "fred@allen-sauer.com"}
var ctx context.Context
var projectID string
var isDev = false

var defaultVersionOrigin = "unset-default-version-origin"

func main() {
	projectID = os.Getenv(GOOGLE_CLOUD_PROJECT)
	isDev = os.Getenv(GAE_APPLICATION) == ""
	port := os.Getenv(PORT)

	if isDev {
		port = "4200"
		defaultVersionOrigin = "http://localhost:" + port
		_ = os.Setenv(GAE_APPLICATION, "my-app-id")
		_ = os.Setenv(GAE_RUNTIME, "go123456")
		_ = os.Setenv(GAE_VERSION, "my-version")
		_ = os.Setenv(GAE_ENV, "standard")
	} else {
		defaultVersionOrigin = "https://" + appengine.DefaultVersionHostname(ctx)
	}

	ctx = context.Background()

	http.HandleFunc("/", indexHandler)

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to listen and serve: %v", err)
	}

	// doit(os.Stdout)
}

func enabledText(enabled bool) string {
	if enabled {
		return "enabled"
	} else {
		return "DISABLED"
	}
}

func renderView(w io.Writer, client *datastore.Client, entity *Entity) error {
	switch entity.Key.Kind {
	case "Person":
		return renderPersonView(w, client, entity)
	case "Contact":
		return renderContactView(w, entity)
	case "Address":
		return renderAddressView(w, entity)
	case "Calendar":
		return renderCalendarView(w, entity)
	default:
		return errors.New(fmt.Sprintf("Unknown kind: %s", entity.Key.Kind))
	}
}

func renderForm(w io.Writer, client *datastore.Client, entity *Entity) error {
	switch entity.Key.Kind {
	case "Person":
		return renderPersonForm(w, client, entity)
	case "Contact":
		return renderContactForm(w, entity)
	case "Address":
		return renderAddressForm(w, entity)
	case "Calendar":
		return renderCalendarForm(w, entity)
	default:
		return errors.New(fmt.Sprintf("Unknown kind: %s", entity.Key.Kind))
	}
}

func getValue(r *http.Request, name string) string {
	value := r.URL.Query().Get(name)
	if value == "" {
		value = r.Form.Get(name)
	}
	return value
}

func intersection(a, b []*datastore.Key) []*datastore.Key {
	result := make([]*datastore.Key, 0)
	for k := range a {
		if slices.ContainsFunc(b, func(v *datastore.Key) bool { return v.Equal(a[k]) }) {
			result = append(result, a[k])
		}
	}
	return result
}

func tasknotifyHandler(w http.ResponseWriter, r *http.Request, client *datastore.Client) error {
	// loc, err := time.LoadLocation("America/Los_Angeles")
	// if err != nil {
	// 	log.Fatalf("Failed to load time location: %v", err)
	// }
	nowmmdd := time.Now().Format("01-02")
	nowmmdd = "01-15"
	log.Printf("NOW = %v", nowmmdd)

	query := datastore.NewQuery("Calendar")
	// query = query.FilterEntity(datastore.PropertyFilter{FieldName: "enabled", Operator: "=", Value: true})
	var entities []Entity
	_, err := client.GetAll(ctx, query, &entities)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to fetch calendar entries: %v", err))
	}

	fmt.Fprintf(w, "Found %d calendar entries", len(entities))
	for _, e := range entities {
		if !e.Enabled {
			continue
		}
		mmdd := e.FirstOccurrence.Format("01-02")
		if mmdd == nowmmdd {
			fmt.Fprintf(w, "MATCH %s %s %v", mmdd, nowmmdd, e.Occasion)
		} else {
			// fmt.Fprintf(w,"no match %s %s %v", mmdd, nowmmdd, e.Occasion)
		}
	}

	return nil

	//   now_mm_dd = now.strftime("%m/%d")
	//   log += "Searching for calendar entities for %s ...\n" % now_mm_dd

	//   for calendar in Calendar.all():
	//     if not calendar.enabled:
	//       continue
	//     when = calendar.first_occurrence
	//     if when.strftime("%m/%d") == now_mm_dd:
	//       log += "%s\n" % calendar.viewUrl()
	//       taskqueue.add(url='/task/mail', params={'key': calendar.key()})
	//   log += "Done"
	//   log_and_mail()
}

func searchHandler(w http.ResponseWriter, r *http.Request, client *datastore.Client, u *user.User, q string) error {
	keys := []*datastore.Key{}
	q = strings.TrimSpace(strings.ToLower(q))
	qlist := regexp.MustCompile(`\s+`).Split(q, -1)
	for i, qword := range qlist {
		wordkeys := make([]*datastore.Key, 0)
		for _, kind := range kinds {
			query := datastore.NewQuery(kind)
			query = query.FilterEntity(datastore.PropertyFilter{FieldName: "words", Operator: ">=", Value: qword})
			query = query.FilterEntity(datastore.PropertyFilter{FieldName: "words", Operator: "<=", Value: qword + "~"})
			query = query.KeysOnly()
			ks, err := client.GetAll(ctx, query, Entity{})
			if err != nil {
				return errors.New(fmt.Sprintf("Failed to fetch keys for kind %s, word %s: %v", kind, qword, err))
			}
			log.Printf("kind=%s, qword=%s, ks=%q", kind, qword, ks)
			wordkeys = append(wordkeys, ks...)
		}

		// Convert keys to root `Person` key.
		for j, wk := range wordkeys {
			if wordkeys[j].Parent != nil {
				wordkeys[j] = wk.Parent
			}
		}

		if i == 0 {
			keys = wordkeys
		} else {
			keys = intersection(keys, wordkeys)
		}
	}

	var entities = make([]Entity, len(keys))
	err := client.GetMulti(ctx, keys, entities)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to fetch entities with keys %v: %v", keys, err))
	}

	renderPremable(w, u, q)
	fmt.Fprintf(w, "<div>%d result(s)</div>", len(entities))
	for _, entity := range entities {
		renderView(w, client, &entity)
	}
	renderPostamble(w, u, q)

	return nil
}

func mainPageHandler(w http.ResponseWriter, r *http.Request, client *datastore.Client) error {
	u := user.Current(ctx)
	// log.Printf("user: %v\n", u)
	if u == nil {
		// dest := r.URL.String()
		// url, err := user.LoginURL(ctx, dest)
		// if err != nil {
		// 	log.Fatalf("Failed to generate login URL: %v\n", err)
		// }
		// http.Redirect(w, r, url, http.StatusFound)
		// return
		u = &user.User{
			Email: "someone@gmail.com",
			Admin: true,
		}
	}

	q := getValue(r, "q")
	action := getValue(r, "action")
	kind := getValue(r, "kind")

	// fmt.Fprintf(w, "<div>q=%v</div>", q)
	// fmt.Fprintf(w, "<div>action=%v</div>", action)
	// fmt.Fprintf(w, "<div>kind=%v</div>", kind)

	// TODO Fix multi word search.
	// TODO Search results should all be Person kind.
	if q != "" {
		searchHandler(w, r, client, u, q)
	} else {
		switch action {
		case "create":
			entity := Entity{
				Key: &datastore.Key{
					Kind: kind,
				},
			}
			renderPremable(w, u, q)
			renderForm(w, client, &entity)
			renderPostamble(w, u, q)
		case "view":
			entity, err := requestToRootEntity(r, client)
			if err != nil {
				return errors.New(fmt.Sprintf("Unable to convert request to person: %v", err))
			}
			renderPremable(w, u, q)
			renderPersonView(w, client, entity)
			renderPostamble(w, u, q)
		case "edit":
			entity, err := requestToEntity(r, client)
			if err != nil {
				return errors.New(fmt.Sprintf("Unable to convert request to entity: %v", err))
			}

			renderPremable(w, u, q)
			if r.Method == "POST" {
				renderView(w, client, entity)
			} else {
				renderForm(w, client, entity)
			}
			renderPostamble(w, u, q)
		case "fix":
			// count = 0
			// query = db.Query(keys_only == True)
			// for key := range query {
			// 	count += 1
			// 	if key.kind().startswith('_') {
			// 		continue
			// 	}
			// 	//       taskqueue.add(url='/', params={'fix': key})
			// 	log.Printf("%s: %s", count, key)
			// }
			// fmt.Fprintf(w, `DONE<br>`)
		}
	}

	return nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create client: %v", err), http.StatusInternalServerError)
	}
	defer client.Close()

	// Runs daily from `cron.yaml`, or manually from admin link.
	if r.URL.Path == "/task/notify" {
		err = tasknotifyHandler(w, r, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to handle task %q: %v", r.URL.Path, err), http.StatusInternalServerError)
		}
	}

	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusInternalServerError)
			return
		}
	}

	err = mainPageHandler(w, r, client)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to render main page: %v", err), http.StatusInternalServerError)
	}
}

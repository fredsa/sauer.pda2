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
	"google.golang.org/appengine/v2/mail"
	"google.golang.org/appengine/v2/user"
)

// https://cloud.google.com/appengine/docs/standard/go/runtime#environment_variables
const PORT = "PORT"
const GOOGLE_CLOUD_PROJECT = "GOOGLE_CLOUD_PROJECT" // The Google Cloud project ID associated with your application.
const GAE_APPLICATION = "GAE_APPLICATION"           // App id, with prefix.
const GAE_ENV = "GAE_ENV"                           // `standard` in production.
const GAE_RUNTIME = "GAE_RUNTIME"                   // Runtime in `app.yaml`.
const GAE_VERSION = "GAE_VERSION"                   // App version.
const DUMMY_APP_ID = "my-app-id"

var ADMINS_FREDSA = []string{"fredsa@gmail.com", "fred@allen-sauer.com"}

func init() {
	// Register handlers in init() per `appengine.Main()` documentation.
	http.HandleFunc("/", indexHandler)
}

func main() {
	if isDev() {
		_ = os.Setenv(GAE_APPLICATION, DUMMY_APP_ID)
		_ = os.Setenv(GAE_RUNTIME, "go123456")
		_ = os.Setenv(GAE_VERSION, "my-version")
		_ = os.Setenv(GAE_ENV, "standard")
		_ = os.Setenv(PORT, "4200")
		log.Printf("appengine.Main() will listen: %s", defaultVersionOrigin())
	}

	// Standard App Engine APIs require `appengine.Main` to have been called.
	appengine.Main()
}

func defaultVersionOrigin() string {
	if isDev() {
		return "http://localhost:" + os.Getenv(PORT)
	} else {
		return fmt.Sprintf("https://%s.appspot.com", projectID())
	}
}

func isDev() bool {
	appid := os.Getenv(GAE_APPLICATION)
	return appid == "" || appid == DUMMY_APP_ID
}

func projectID() string {
	return os.Getenv(GOOGLE_CLOUD_PROJECT)
}

func consoleURL() string {
	return fmt.Sprintf(`https://console.cloud.google.com/appengine?project=%s`, projectID())
}

func isAdmin(ctx context.Context) bool {
	log.Printf("user.IsAdmin(ctx)=%v", user.IsAdmin(ctx))
	return isDev() || user.IsAdmin(ctx)
}

func enabledText(enabled bool) string {
	if enabled {
		return "enabled"
	} else {
		return "DISABLED"
	}
}

func renderView(w io.Writer, ctx context.Context, client *datastore.Client, entity *Entity) error {
	switch entity.Key.Kind {
	case "Person":
		return renderPersonView(w, ctx, client, entity)
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

func tasknotifyHandler(w http.ResponseWriter, ctx context.Context, client *datastore.Client) error {
	// https://cloud.google.com/appengine/docs/standard/services/mail?tab=go#who_can_send_mail
	// - The Gmail or Google Workspace Account of the user who is currently signed in
	// - Any email address of the form anything@[MY_PROJECT_ID].appspotmail.com or anything@[MY_PROJECT_NUMBER].appspotmail.com
	// - Any email address listed in the Google Cloud console under Email API Authorized Senders:
	//   https://console.cloud.google.com/appengine/settings/emailsenders?project=sauer-pda
	sender := fmt.Sprintf("pda@%s.appspotmail.com", projectID())

	// emailTo = []string{"Fred and/or Amber Sauer <sauer@allen-sauer.com>"}
	emailTo := []string{"Fred Sauer <fredsa@gmail.com>"}

	// loc, err := time.LoadLocation("America/Los_Angeles")
	// if err != nil {
	// 	log.Fatalf("Failed to load time location: %v", err)
	// }
	nowmmdd := time.Now().Format("01-02")
	query := datastore.NewQuery("Calendar")
	query = query.FilterEntity(datastore.PropertyFilter{FieldName: "enabled", Operator: "=", Value: true})
	var events []Entity
	_, err := client.GetAll(ctx, query, &events)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to fetch calendar entries: %v", err))
	}

	fmt.Fprintf(w, "Comparing %d enabled calendar entries ragainst today's date: %v\n", len(events), nowmmdd)
	for _, event := range events {
		if !event.Enabled {
			continue
		}

		mmdd := event.FirstOccurrence.Format("01-02")
		if mmdd == nowmmdd {
			person := &Entity{}
			err = client.Get(ctx, event.Key.Parent, person)

			event := fmt.Sprintf("%s %s %s", event.FirstOccurrence.Format("2006-01-02"), event.Occasion, event.Comments)
			body := fmt.Sprintf("%s\n\n%s\n", person.displayName(), person.viewURL())
			subject := fmt.Sprintf("%s %s", projectID(), event)
			fmt.Fprintf(w, "\n>>> MATCH %v\n", event)

			msg := &mail.Message{
				Sender:  sender,
				To:      emailTo,
				Subject: subject,
				Body:    body,
			}
			mail.Send(ctx, msg)
			log.Printf("Sent email for event: %q\n", event)
			log.Printf("- Sender: %s", sender)
			log.Printf("- To: %s", emailTo)
			log.Printf("- Subject: %s", subject)
			log.Printf("- Body: %s", body)
		}
	}

	return nil
}

func searchHandler(w http.ResponseWriter, ctx context.Context, client *datastore.Client, q string) error {
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
			// log.Printf("Found kind=%s, qword=%s, ks=%q", kind, qword, ks)
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

	renderPremable(w, ctx, q)
	fmt.Fprintf(w, "<div>%d result(s)</div>", len(entities))
	for _, entity := range entities {
		renderView(w, ctx, client, &entity)
	}
	renderPostamble(ctx, w)

	return nil
}

func mainPageHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, client *datastore.Client) error {
	q := getValue(r, "q")
	action := getValue(r, "action")
	key := getValue(r, "key")

	if q != "" {
		searchHandler(w, ctx, client, q)
	} else {
		switch action {
		case "create":
			dbkey, err := datastore.DecodeKey(key)
			if err != nil {
				return errors.New(fmt.Sprintf("Failed to decode key %q: %v", key, err))
			}
			entity := Entity{
				// Key: &datastore.Key{
				// Kind: kind,
				// },
				Key: dbkey,
			}
			renderPremable(w, ctx, q)
			renderForm(w, ctx, &entity)
			renderPostamble(ctx, w)
		case "view":
			entity, err := requestToEntity(r, ctx, client)
			if err != nil {
				return errors.New(fmt.Sprintf("Unable to convert request to person: %v", err))
			}
			renderPremable(w, ctx, q)
			renderPersonView(w, ctx, client, entity)
			renderPostamble(ctx, w)
		case "edit":
			entity, err := requestToEntity(r, ctx, client)
			if err != nil {
				return errors.New(fmt.Sprintf("Unable to convert request to entity: %v", err))
			}

			if r.Method == "POST" {
				key := entity.Key
				if entity.Key.Parent != nil {
					key = entity.Key.Parent
				}
				http.Redirect(w, r, fmt.Sprintf("/?action=view&key=%s", key.Encode()), http.StatusFound)
				return nil
			} else {
				renderPremable(w, ctx, q)
				renderForm(w, ctx, entity)
				renderPostamble(ctx, w)
			}
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
		default:
			renderPremable(w, ctx, q)
			renderPostamble(ctx, w)
		}
	}

	return nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// App Engine context for the in-flight HTTP request.
	ctx := appengine.NewContext(r)

	client, err := datastore.NewClient(ctx, projectID())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create client: %v", err), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	// Runs daily from `cron.yaml`, or manually from admin link.
	if r.URL.Path == "/task/notify" {
		err = tasknotifyHandler(w, ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to handle task %q: %v", r.URL.Path, err), http.StatusInternalServerError)
			return
		}
		return
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

	err = mainPageHandler(w, r, ctx, client)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to render main page: %v", err), http.StatusInternalServerError)
	}
}

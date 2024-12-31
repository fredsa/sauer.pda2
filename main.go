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

var ADMINS_FREDSA = []string{"fredsa@gmail.com", "fred@allen-sauer.com"}
var projectID string
var isDev = false
var sender string
var emailTo []string

var defaultVersionOrigin = "unset-default-version-origin"

func main() {
	http.HandleFunc("/", indexHandler)

	projectID = os.Getenv(GOOGLE_CLOUD_PROJECT)
	isDev = os.Getenv(GAE_APPLICATION) == ""
	port := os.Getenv(PORT)

	// https://cloud.google.com/appengine/docs/standard/services/mail?tab=go#who_can_send_mail
	// - The Gmail or Google Workspace Account of the user who is currently signed in
	// - Any email address of the form anything@[MY_PROJECT_ID].appspotmail.com or anything@[MY_PROJECT_NUMBER].appspotmail.com
	// - Any email address listed in the Google Cloud console under Email API Authorized Senders:
	//   https://console.cloud.google.com/appengine/settings/emailsenders?project=sauer-pda
	sender = fmt.Sprintf("pda@%s.appspotmail.com", projectID)
	// emailTo = []string{"Fred and/or Amber Sauer <sauer@allen-sauer.com>"}
	emailTo = []string{"Fred Sauer <fredsa@gmail.com>"}

	if isDev {
		port = "4200"
		defaultVersionOrigin = "http://localhost:" + port
		_ = os.Setenv(GAE_APPLICATION, "my-app-id")
		_ = os.Setenv(GAE_RUNTIME, "go123456")
		_ = os.Setenv(GAE_VERSION, "my-version")
		_ = os.Setenv(GAE_ENV, "standard")
		emailTo = []string{"Fred Sauer <fredsa@gmail.com>"}

		log.Printf("Listening on port %s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("Failed to listen and serve: %v", err)
		}
	} else {
		defaultVersionOrigin = fmt.Sprintf("https://%s.appspot.com", projectID)
		appengine.Main()
	}

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

	fmt.Fprintf(w, "Comparing %d enabled calendar entries against today's date: %v\n", len(events), nowmmdd)
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
			subject := fmt.Sprintf("%s %s", projectID, event)
			fmt.Fprintf(w, "MATCH %s %s %v\n", mmdd, nowmmdd, event)

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

func searchHandler(w http.ResponseWriter, ctx context.Context, client *datastore.Client, u *user.User, q string) error {
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

	renderPremable(w, u, q)
	fmt.Fprintf(w, "<div>%d result(s)</div>", len(entities))
	for _, entity := range entities {
		renderView(w, ctx, client, &entity)
	}
	renderPostamble(w, u)

	return nil
}

func mainPageHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, client *datastore.Client) error {
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
	// kind := getValue(r, "kind")
	key := getValue(r, "key")

	// fmt.Fprintf(w, "<div>q=%v</div>", q)
	// fmt.Fprintf(w, "<div>action=%v</div>", action)
	// fmt.Fprintf(w, "<div>kind=%v</div>", kind)

	// TODO Fix multi word search.
	// TODO Search results should all be Person kind.
	if q != "" {
		searchHandler(w, ctx, client, u, q)
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
			renderPremable(w, u, q)
			renderForm(w, &entity)
			renderPostamble(w, u)
		case "view":
			// TODO Here, or elsewhere, make this view the root entity.
			entity, err := requestToEntity(r, ctx, client)
			if err != nil {
				return errors.New(fmt.Sprintf("Unable to convert request to person: %v", err))
			}
			renderPremable(w, u, q)
			renderPersonView(w, ctx, client, entity)
			renderPostamble(w, u)
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
				renderPremable(w, u, q)
				renderForm(w, entity)
				renderPostamble(w, u)
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
			renderPremable(w, u, q)
			renderPostamble(w, u)
		}
	}

	return nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	rctx := r.Context()
	log.Printf("request ctx=%p %v", &rctx, rctx)

	// Context for the in-flight HTTP request.
	ctx := appengine.NewContext(r)
	log.Printf("appengine.NewContext=%p %v", &ctx, ctx)

	bgctx := context.Background()
	log.Printf("context.Background()=%p %v", &bgctx, bgctx)

	log.Printf("appengine.AppID(ctx)=%v", appengine.AppID(ctx))
	log.Printf("appengine.AppID(rctx)=%v", appengine.AppID(rctx))
	log.Printf("appengine.AppID(bgctx)=%v", appengine.AppID(bgctx))

	log.Printf("appengine.IsAppEngine()=%v", appengine.IsAppEngine())
	log.Printf("appengine.IsDevAppServer()=%v", appengine.IsDevAppServer())
	log.Printf("appengine.IsFlex()=%v", appengine.IsFlex())
	log.Printf("appengine.IsSecondGen()=%v", appengine.IsSecondGen())
	if !isDev {
		log.Printf("appengine.IsStandard()=%v", appengine.IsStandard()) // Requires http://metadata/computeMetadata/v1/instance/attributes/gae_backend_instance
		log.Printf("appengine.InstanceID()=%v", appengine.InstanceID()) // Requires http://metadata/computeMetadata/v1/instance/attributes/gae_backend_instance
	}

	log.Printf("user.Current(ctx)=%v", user.Current(ctx))
	log.Printf("user.Current(rctx)=%v", user.Current(rctx))
	log.Printf("user.Current(bgctx)=%v", user.Current(bgctx))

	log.Printf("user.IsAdmin(ctx)=%v", user.IsAdmin(ctx))
	log.Printf("user.IsAdmin(rctx)=%v", user.IsAdmin(rctx))
	log.Printf("user.IsAdmin(bgctx)=%v", user.IsAdmin(bgctx))

	if !isDev {
		u, e := user.LoginURL(ctx, "/foo") // Requires http://appengine.googleapis.internal:10001/rpc_http
		log.Printf("user.LoginURL(ctx, /foo)=%v e=%v", u, e)

		u, e = user.LoginURL(rctx, "/foo")
		log.Printf("user.LoginURL(rctx, /foo)=%v e=%v", u, e)

		u, e = user.LoginURL(bgctx, "/foo")
		log.Printf("user.LoginURL(bgctx, /foo)=%v e=%v", u, e)
	}

	client, err := datastore.NewClient(ctx, projectID)
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

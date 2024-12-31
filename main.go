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
	"google.golang.org/appengine/user"
	"google.golang.org/appengine/v2"
	"google.golang.org/appengine/v2/mail"
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

func init() {
	projectID = os.Getenv(GOOGLE_CLOUD_PROJECT)
	isDev = os.Getenv(GAE_APPLICATION) == ""

	if isDev {
		_ = os.Setenv(PORT, "4200")
		port := os.Getenv(PORT)
		_ = os.Setenv(GAE_APPLICATION, "my-app-id")
		_ = os.Setenv(GAE_RUNTIME, "go123456")
		_ = os.Setenv(GAE_VERSION, "my-version")
		_ = os.Setenv(GAE_ENV, "standard")
		emailTo = []string{"Fred Sauer <fredsa@gmail.com>"}
		defaultVersionOrigin = "http://localhost:" + port
	} else {
		defaultVersionOrigin = fmt.Sprintf("https://%s.appspot.com", projectID)
	}

	// Register handlers in init() per `appengine.Main()` documentation.
	http.HandleFunc("/", indexHandler)
}

func main() {
	if isDev {
		log.Printf("appengine.Main() will listen: %s", defaultVersionOrigin)
	}

	// Standard App Engine APIs require `appengine.Main` to have been called.
	appengine.Main()
}

func isAdmin(ctx context.Context) bool {
	log.Printf("user.IsAdmin(ctx)=%v", user.IsAdmin(ctx))
	return isDev || user.IsAdmin(ctx)
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
	sender = fmt.Sprintf("pda@%s.appspotmail.com", projectID)

	// emailTo = []string{"Fred and/or Amber Sauer <sauer@allen-sauer.com>"}
	emailTo = []string{"Fred Sauer <fredsa@gmail.com>"}

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
			subject := fmt.Sprintf("%s %s", projectID, event)
			fmt.Fprintf(w, "MATCH %v\n", event)

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
	// kind := getValue(r, "kind")
	key := getValue(r, "key")

	// fmt.Fprintf(w, "<div>q=%v</div>", q)
	// fmt.Fprintf(w, "<div>action=%v</div>", action)
	// fmt.Fprintf(w, "<div>kind=%v</div>", kind)

	// TODO Fix multi word search.
	// TODO Search results should all be Person kind.
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
			// TODO Here, or elsewhere, make this view the root entity.
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

	log.Printf(">>>>>>>>>> indexHandler: ctx=%v", ctx)

	// Locally, nil
	// Server, nil
	log.Printf("indexHandler: user.Current(ctx)=%v", user.Current(ctx))

	// Locally, nil
	// Server, nil
	log.Printf("indexHandler: user.Current(ctx)=%q", user.Current(ctx))

	// Locally, nil
	// Server, nil
	log.Printf("indexHandler: user.Current(ctx)=%s", user.Current(ctx))

	// Locally, false
	// Server, false
	log.Printf("indexHandler: user.IsAdmin(ctx)=%v", user.IsAdmin(ctx))

	if !isDev {
		// Locally, err `service bridge HTTP failed: Post "http://appengine.googleapis.internal:10001/rpc_http": dial tcp: lookup appengine.googleapis.internal: no such host`
		// Server, err `API error 2 (user: NOT_ALLOWED)`
		loginURL, err := user.LoginURL(ctx, "foo")
		log.Printf("indexHandler: user.LoginURL(ctx)=%v err=%v", loginURL, err)

		// Locally, err `service bridge HTTP failed: Post "http://appengine.googleapis.internal:10001/rpc_http": dial tcp: lookup appengine.googleapis.internal: no such host`
		// Server, err `API error 2 (user: NOT_ALLOWED)`
		logoutURL, err := user.LoginURL(ctx, "foo")
		log.Printf("indexHandler: user.LogoutURL(ctx)=%v err=%v", logoutURL, err)
	}

	// Locally, value `os.Getenv("GAE_APPLICATION")`
	// Server, `sauer-pda-dev`
	log.Printf("indexHandler: appengine.AppID(ctx)=%v", appengine.AppID(ctx))

	if !isDev {
		// Locally, `` and logs `Get "http://metadata/computeMetadata/v1/instance/zone": dial tcp: lookup metadata: no such host`
		// Server, `us-west1-1`
		log.Printf("indexHandler: appengine.Datacenter(ctx)=%v", appengine.Datacenter(ctx))
	}

	// Locally, ``
	// Server, `sauer-pda-dev.uw.r.appspot.com`
	log.Printf("indexHandler: appengine.DefaultVersionHostname(ctx)=%v", appengine.DefaultVersionHostname(ctx))

	if !isDev {
		// Locally, panics `http: panic serving [::1]:64902: Metadata fetch failed for 'instance/attributes/gae_backend_instance': Get "http://metadata/computeMetadata/v1/instance/attributes/gae_backend_instance": dial tcp: lookup metadata: no such host`
		// Server, `0066d924808f85e59480f4f834d89809739e28d68d0471e54a81ecfdd776e886ec44b9294369e983466180a7ef8a9dd24967183bc382a400c8a9d3f8f483ef2fac91a655321eeb3743b798cf97ca`
		log.Printf("indexHandler: appengine.InstanceID()=%v", appengine.InstanceID())
	}

	// Locally, true
	// Server, true
	log.Printf("indexHandler: appengine.IsAppEngine()=%v", appengine.IsAppEngine())

	// Locally, false
	// Server, false
	log.Printf("indexHandler: appengine.IsDevAppServer()=%v", appengine.IsDevAppServer())

	// Locally, false
	// Server, false
	log.Printf("indexHandler: appengine.IsFlex()=%v", appengine.IsFlex())

	// Locally, true
	// Server, true
	log.Printf("indexHandler: appengine.IsSecondGen()=%v", appengine.IsSecondGen())

	// Locally, true
	// Server, true
	log.Printf("indexHandler: appengine.IsStandard()=%v", appengine.IsStandard())

	if !isDev {
		// Locally, panics `http: panic serving [::1]:65140: Metadata fetch failed for 'instance/attributes/gae_backend_name': Get "http://metadata/computeMetadata/v1/instance/attributes/gae_backend_name": dial tcp: lookup metadata: no such host`
		// Server, `default`
		log.Printf("indexHandler: appengine.ModuleName(ctx)=%v", appengine.ModuleName(ctx))
	}

	// Locally, ``
	// Server, `677455b700ff0cb62a9b86bdbb00017a75777e73617565722d7064612d6465760001323032343132333174313232383135000100`
	log.Printf("indexHandler: appengine.RequestID(ctx)=%v", appengine.RequestID(ctx))

	// Locally, `standard`
	// Server, `standard`
	log.Printf("indexHandler: appengine.ServerSoftware()=%v", appengine.ServerSoftware())

	if !isDev {
		// Locally, err `service bridge HTTP failed: Post "http://appengine.googleapis.internal:10001/rpc_http": dial tcp: lookup appengine.googleapis.internal: no such host`
		// Server, `sauer-pda-dev@appspot.gserviceaccount.com`
		serviceAccount, err := appengine.ServiceAccount(ctx)
		log.Printf("indexHandler: appengine.ServiceAccount(ctx)=%v err=%v", serviceAccount, err)
	}

	if !isDev {
		// Locally, panics `http: panic serving [::1]:49339: Metadata fetch failed for 'instance/attributes/gae_backend_version': Get "http://metadata/computeMetadata/v1/instance/attributes/gae_backend_version": dial tcp: lookup metadata: no such host`
		// Server, `20241231t122815.465917320654064374`
		log.Printf("indexHandler: appengine.VersionID(ctx)=%v", appengine.VersionID(ctx))
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

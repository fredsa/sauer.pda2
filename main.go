package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

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
		log.Fatal(err)
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

func renderView(w io.Writer, client *datastore.Client, entity *Entity) {
	switch entity.Key.Kind {
	case "Person":
		renderPersonView(w, client, entity)
	case "Contact":
		renderContactView(w, entity)
	case "Address":
		renderAddressView(w, entity)
	case "Calendar":
		renderCalendarView(w, entity)
	default:
		log.Fatalf("Unknown kind: %s", entity.Key.Kind)
	}
}

func renderForm(w io.Writer, client *datastore.Client, entity *Entity) {
	switch entity.Key.Kind {
	case "Person":
		renderPersonForm(w, client, entity)
	case "Contact":
		renderContactForm(w, entity)
	case "Address":
		renderAddressForm(w, entity)
	case "Calendar":
		renderCalendarForm(w, entity)
	default:
		log.Fatalf("Unknown kind: %s", entity.Key.Kind)
	}
}

func getValue(r *http.Request, name string) string {
	value := r.URL.Query().Get(name)
	if value == "" {
		value = r.Form.Get(name)
	}
	return value
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			// log.Fatalf("Error parsing form: %v", err)
			return
		}
	}

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
	modified := getValue(r, "modified") == "true"

	fmt.Fprintf(w, "<div>q=%v</div>", q)
	fmt.Fprintf(w, "<div>action=%v</div>", action)
	fmt.Fprintf(w, "<div>kind=%v</div>", kind)
	fmt.Fprintf(w, "<div>modified=%v</div>", modified)
	renderPremable(w, u, q)

	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	if q != "" {
		keys := []*datastore.Key{}
		q = strings.TrimSpace(strings.ToLower(q))
		qlist := regexp.MustCompile(`\s+`).Split(q, -1)
		for _, qword := range qlist {
			for _, kind := range kinds {
				query := datastore.NewQuery(kind)
				query = query.FilterEntity(datastore.PropertyFilter{FieldName: "words", Operator: ">=", Value: qword})
				query = query.FilterEntity(datastore.PropertyFilter{FieldName: "words", Operator: "<=", Value: qword + "~"})
				query = query.KeysOnly()
				ks, err := client.GetAll(ctx, query, Entity{})
				if err != nil {
					log.Fatalf("Failed to fetch keys for kind %s, word %s: %v", kind, qword, err)
				}
				log.Printf("kind=%s, qword=%s, ks=%q", kind, qword, ks)
				keys = append(keys, ks...)
			}
		}

		// TODO Filter out duplicate keys.
		var entities = make([]Entity, len(keys))
		err = client.GetMulti(ctx, keys, entities)
		if err != nil {
			log.Fatalf("Failed to fetch entities with keys %v: %v", keys, err)
		}
		for _, entity := range entities {
			renderView(w, client, &entity)
		}
	} else {
		switch action {
		case "create":
			entity := Entity{
				Key: &datastore.Key{
					Kind: kind,
				},
			}
			renderForm(w, client, &entity)
		case "view":
			entity, err := requestToRootEntity(r, client)
			if err != nil {
				log.Fatalf("Unable to convert request to person: %v", err)
			}
			renderPersonView(w, client, entity)
		case "edit":
			entity, err := requestToEntity(r, client)
			if err != nil {
				log.Fatalf("Unable to convert request to entity: %v", err)
			}

			if r.Method == "POST" {
				entity.fixAndSave(client)
			}

			if modified {
				renderView(w, client, entity)
			} else {
				renderForm(w, client, entity)
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
		}
	}

	renderPostamble(w, u, q)
}

// func doit(w io.Writer) {
// 	client, err := datastore.NewClient(ctx, projectID)
// 	if err != nil {
// 		log.Fatalf("Failed to create client: %v", err)
// 	}
// 	defer client.Close()

// 	query := datastore.NewQuery("Person").Limit(15)

// 	var people []Thing
// 	_, err = client.GetAll(ctx, query, &people)
// 	if err != nil {
// 		log.Fatalf("Failed to get all: %v", err)
// 	}

// 	for i, p := range people {
// 		name := fmt.Sprintf("%v %v %v", p.CompanyName, p.FirstName, p.LastName)
// 		name = strings.TrimSpace(name)
// 		fmt.Fprintf(w, "%3d: %v %v\n", i, p.FirstOccurrence, name)
// 	}

// 	fmt.Fprint(w, "Done.")
// }

package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"

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

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
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

	q := r.URL.Query().Get("q")
	action := r.URL.Query().Get("action")
	kind := r.URL.Query().Get("kind")
	modified := r.URL.Query().Get("modified") == "true"

	renderPremable(w, u, q)

	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// k := datastore.NameKey("Foo", "fookey", nil)
	// foo := &Entity{
	// 	City:     "foocity",
	// 	Comments: "foocomments" + strings.Repeat("x", 10000),
	// }
	// client.Put(ctx, k, foo)

	// k := datastore.IDKey("Person", 1001, nil)
	// var e Entity
	// client.Get(ctx, k, &e)
	// log.Printf("e.CompanyName=%v", e.CompanyName)
	// log.Printf("e.Comments=%v", e.Comments)

	// k = datastore.IDKey("Foo", 1001, nil)
	// client.Put(ctx, k, &e)
	// e = Entity{}
	// client.Get(ctx, k, &e)
	// log.Printf("e.CompanyName=%v", e.CompanyName)
	// log.Printf("e.Comments=%v", e.Comments)

	if q != "" {
		//   qlist = re.split('\W+', q.lower())
		//   if '' in qlist:
		//     qlist.remove('')
		//   results = None
		//   for qword in qlist:
		//     word_results = set([])
		//     for kind in [Person, Address, Calendar, Contact]:
		//       query = db.Query(kind, keys_only=True)
		//       query.filter("words >=", qword)
		//       query.filter("words <=", qword + "~")
		//       word_results = word_results | set([x.parent() or x for x in query])
		//       #self.response.out.write("word_results = %s<br><br>" % word_results)
		//     if results is None:
		//       #self.response.out.write("results is None<br>")
		//       results = word_results
		//     else:
		//       results = results & word_results
		//     #self.response.out.write("results = %s<br><br>" % results
		//     self.response.out.write("%s result(s) matching <code>%s</code><br>" % (len(word_results), qword))

		//   keys = list(results)
		//   if (len(qlist) > 1):
		//     self.response.out.write("===> %s result(s) matching <code>%s</code><br>" % (len(keys), " ".join(qlist)))
		//   while (keys):
		//     # Max 30 keys allow in IN clause
		//     somekeys = keys[:30]
		//     keys = keys[30:]
		//     #self.response.out.write("somekeys = %s<br><br>" % somekeys)
		//     query = db.Query(Person)
		//     query.filter("__key__ IN", somekeys)
		//     s = set(query)
		//     #self.response.out.write("s = %s<br><br>" % s)
		//     for person in sorted(s, key=Thing.key):
		//       #self.response.out.write("person = %s<br><br>" % person)
		//       self.personView(person)
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

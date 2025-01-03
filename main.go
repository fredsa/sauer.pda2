package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/appengine/v2"
	"google.golang.org/appengine/v2/mail"
	"google.golang.org/appengine/v2/taskqueue"
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

func datastoreURL() string {
	return fmt.Sprintf(`https://console.cloud.google.com/datastore/databases/-default-?project=%s`, projectID())
}

func isAdmin(ctx context.Context) bool {
	return isDev() || user.IsAdmin(ctx)
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

func mailmergeHandler(w http.ResponseWriter, ctx context.Context, client *datastore.Client) error {
	lineFormat := "%q,%q,%q,%q,%q\n"
	fmt.Fprintf(w, lineFormat, "Name", "AddressLine1", "AddressLine2", "AddressLine3", "AddressLine4")

	query := datastore.NewQuery("Person")
	query = query.FilterField("send_card", "=", true)
	query = query.FilterField("enabled", "=", true)
	var people []Entity
	_, err := client.GetAll(ctx, query, &people)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to fetch people: %v", err))
	}

	for _, person := range people {
		name := person.MailingName
		if name == "" {
			name = person.displayName()
		}

		aquery := datastore.NewQuery("Address").Ancestor(person.Key).Limit(2)
		aquery = aquery.FilterField("__key__", ">", person.Key)
		aquery = aquery.FilterField("enabled", "=", true)
		var addresses []Entity
		_, err = client.GetAll(ctx, aquery, &addresses)
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to fetch addresses: %v", err))
		}

		if len(addresses) == 0 {
			fmt.Fprintf(w, lineFormat, name,
				"___________",
				"___________",
				"___________",
				"___________",
			)
		}

		for _, a := range addresses {
			// United States
			line3 := a.City + ", " + a.StateProvince + " " + a.PostalCode
			switch a.Country {
			case "The Netherlands":
				line3 = a.PostalCode + "  " + a.City
				if a.StateProvince != "" {
					line3 = line3 + ", " + a.StateProvince
				}
			case "Portugal":
				line3 = a.PostalCode + "  " + a.City
				if a.StateProvince != "" {
					line3 = line3 + ", " + a.StateProvince
				}
			case "Canada":
				line3 = a.City + " " + a.StateProvince + "  " + a.PostalCode
			}
			fmt.Fprintf(w, lineFormat, name,
				a.AddressLine1,
				a.AddressLine2,
				strings.TrimSpace(line3),
				a.Country,
			)
		}
	}

	return nil
}

func tasknotifyHandler(w http.ResponseWriter, ctx context.Context, client *datastore.Client) error {
	// https://cloud.google.com/appengine/docs/standard/services/mail?tab=go#who_can_send_mail
	// - The Gmail or Google Workspace Account of the user who is currently signed in
	// - Any email address of the form anything@[MY_PROJECT_ID].appspotmail.com or anything@[MY_PROJECT_NUMBER].appspotmail.com
	// - Any email address listed in the Google Cloud console under Email API Authorized Senders:
	sender := fmt.Sprintf("pda@%s.appspotmail.com", projectID())

	emailTo := []string{"sauer@allen-sauer.com"}

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
		renderPersonView(w, ctx, client, &entity)
	}
	renderPostamble(ctx, w)

	return nil
}

func addTask(w http.ResponseWriter, ctx context.Context, task *taskqueue.Task) error {
	if isDev() {
		fmt.Fprintf(w, "*** dev mode *** Not adding task: %s", task.Path)
		return nil
	} else {
		task, err := taskqueue.Add(ctx, task, "")
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to add task %v: %v", task.Path, err))
		}

		fmt.Fprintf(w, "Added task: %s", task.Path)
		return nil
	}
}

func addTasks(w http.ResponseWriter, ctx context.Context, tasks []*taskqueue.Task) error {
	if isDev() {
		fmt.Fprintf(w, "*** dev mode *** Not adding %d tasks", len(tasks))
		return nil
	} else {
		tasks, err := taskqueue.AddMulti(ctx, tasks, "")
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to add %v tasks: %v", len(tasks), err))
		}

		fmt.Fprintf(w, "Added %d tasks", len(tasks))
		return nil
	}
}

func fixPersonHandler(w http.ResponseWriter, ctx context.Context, client *datastore.Client, key string) error {
	dbkey, err := datastore.DecodeKey(key)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode person key %q: %v", key, err), http.StatusBadRequest)
	}

	// Results include ancestor Person and all descendents.
	query := datastore.NewQuery("").Ancestor(dbkey)
	var entities []Entity
	_, err = client.GetAll(ctx, query, &entities)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to fetch all to be fixed entities: %v", err))
	}

	fmt.Fprintf(w, "Fixing %d entities:\n", len(entities))
	keys := make([]*datastore.Key, len(entities))
	for i, e := range entities {
		keys[i] = e.Key
		fmt.Fprintf(w, "%4d: %v\n", i+1, e.Key)
		before := fmt.Sprintf("%v", e)
		e.fix()

		after := fmt.Sprintf("%v", e)
		if before == after {
			fmt.Fprintf(w, "Same")
		} else {
			fmt.Fprintf(w, "Before: %v\n", before)
			fmt.Fprintf(w, "After : %v\n", after)
		}
		fmt.Fprint(w, strings.Repeat("\n", 10))

		entities[i] = e
	}
	keys, err = client.PutMulti(ctx, keys, entities)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to put %d fixed entities: %v", len(entities), err))
	}

	fmt.Fprintf(w, "Done")
	return nil
}

func fixAllHandler(w http.ResponseWriter, ctx context.Context, client *datastore.Client, next string) error {
	// https://cloud.google.com/appengine/docs/standard/quotas#Task_Queue
	MAX_TASKS_PER_BATCH := 100

	query := datastore.NewQuery("Person")
	query = query.Limit(MAX_TASKS_PER_BATCH)

	if next != "" {
		key, err := datastore.DecodeKey(next)
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to decode person key %q: %v", next, err))
		}
		query = query.FilterField("__key__", ">", key)
	}

	var people []Entity
	_, err := client.GetAll(ctx, query, &people)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to fetch person entities: %v", err))
	}

	if len(people) == MAX_TASKS_PER_BATCH {
		next := people[MAX_TASKS_PER_BATCH-1].Key
		path, err := url.JoinPath("/task/fix", "all", next.Encode())
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to join path: %v", err))
		}
		fmt.Fprintf(w, "Adding continuation task: %v\n", path)
		task := taskqueue.NewPOSTTask(path, nil)
		err = addTask(w, ctx, task)
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to add continuation task with key %v: %v", next, err))
		}
	}

	fmt.Fprintf(w, "\n\nCreating %d tasks:\n", len(people))
	tasks := make([]*taskqueue.Task, len(people))
	for i, person := range people {
		path, err := url.JoinPath("/task/fix", "Person", person.Key.Encode())
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to join person path: %v", err))
		}
		tasks[i] = taskqueue.NewPOSTTask(path, nil)
		fmt.Fprintf(w, "%4d: %v => %v  %v\n", i+1, tasks[i].Path, person.Key, person.displayName())
	}
	err = addTasks(w, ctx, tasks)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to add tasks: %v", err))
	}

	return nil
}

func fixHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, client *datastore.Client) error {
	// Expect one of:
	// - `/task/fix/all/`         => []string{ "" "task" "fix" "all" "" }
	// - `/task/fix/all/<key>`    => []string{ "" "task" "fix" "Person" "<key>" }
	// - `/task/fix/Person/<key>` => []string{ "" "task" "fix" "Person" "<key>" }
	segments := strings.Split(r.URL.Path, "/")

	if len(segments) != 5 {
		return errors.New(fmt.Sprintf("Invalid segments: %q", segments))
	}

	switch segments[3] {
	case "Person":
		return fixPersonHandler(w, ctx, client, segments[4])
	case "all":
		return fixAllHandler(w, ctx, client, segments[4])
	default:
		return errors.New(fmt.Sprintf("Unhandled path segment %s for path: %s", segments[1], r.URL.Path))
	}
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
				dbkey, err := entity.save(ctx, client)
				if err != nil {
					return errors.New(fmt.Sprintf("Unable to save entity: %v", err))
				}
				entity.Key = dbkey

				if dbkey.Parent != nil {
					dbkey = dbkey.Parent
				}
				http.Redirect(w, r, fmt.Sprintf("/?action=view&key=%s", dbkey.Encode()), http.StatusFound)
				return nil
			} else {
				renderPremable(w, ctx, q)
				renderForm(w, ctx, entity)
				renderPostamble(ctx, w)
			}
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

	if r.URL.Path == "/mailmerge" {
		err = mailmergeHandler(w, ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to json export %q: %v", r.URL.Path, err), http.StatusInternalServerError)
			return
		}
		return
	}

	// Runs daily from `cron.yaml`, or manually from admin link.
	if r.URL.Path == "/task/notify" {
		err = tasknotifyHandler(w, ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to handle task %q: %v", r.URL.Path, err), http.StatusInternalServerError)
			return
		}
		return
	}

	// Fix entities.
	if strings.HasPrefix(r.URL.Path, "/task/fix") {
		err = fixHandler(w, r, ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed fix handler: %v", err), http.StatusInternalServerError)
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

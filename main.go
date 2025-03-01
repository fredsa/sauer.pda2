package main

import (
	"bytes"
	"context"
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

var ADMINS_FREDSA = []string{"fredsa@gmail.com"}
var WORDS_RE = regexp.MustCompile(`[^\w=]+`)

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
	return isDev() || slices.Contains(ADMINS_FREDSA, user.Current(ctx).Email)
}

func getValue(r *http.Request, name string) string {
	value := r.URL.Query().Get(name)
	if value == "" {
		value = r.Form.Get(name)
	}
	return value
}

func mailmergeHandler(ctx context.Context, client *datastore.Client) (string, error) {
	var buffer bytes.Buffer

	lineFormat := "%q,%q,%q,%q,%q\n"
	buffer.WriteString(fmt.Sprintf(lineFormat, "Name", "AddressLine1", "AddressLine2", "AddressLine3", "AddressLine4"))

	query := datastore.NewQuery("Person")
	query = query.FilterField("send_card", "=", true)
	query = query.FilterField("enabled", "=", true)
	var people []Entity
	_, err := client.GetAll(ctx, query, &people)
	if err != nil {
		return "", fmt.Errorf("failed to fetch people: %v", err)
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
			return "", fmt.Errorf("failed to fetch addresses: %v", err)
		}

		if len(addresses) == 0 {
			buffer.WriteString(fmt.Sprintf(lineFormat, name,
				"___________",
				"___________",
				"___________",
				"___________",
			))
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
			buffer.WriteString(fmt.Sprintf(lineFormat, name,
				a.AddressLine1,
				a.AddressLine2,
				strings.TrimSpace(line3),
				a.Country,
			))
		}
	}

	return buffer.String(), nil
}

func tasknotifyHandler(ctx context.Context, client *datastore.Client) (string, error) {
	var buffer bytes.Buffer

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
		return "", fmt.Errorf("failed to fetch calendar entries: %v", err)
	}

	buffer.WriteString(fmt.Sprintf("Comparing %d enabled calendar entries ragainst today's date: %v\n", len(events), nowmmdd))
	for _, event := range events {
		if !event.Enabled {
			continue
		}

		mmdd := event.FirstOccurrence.Format("01-02")
		if mmdd == nowmmdd {
			person := &Entity{}
			err = client.Get(ctx, event.Key.Parent, person)
			if err != nil {
				return "", fmt.Errorf("failed to get person for event: %v", err)
			}

			event := fmt.Sprintf("%s %s %s", event.FirstOccurrence.Format("2006-01-02"), event.Occasion, event.Comments)
			body := fmt.Sprintf("%s\n\n%s\n", person.displayName(), person.viewURL())
			subject := fmt.Sprintf("%s %s", projectID(), event)
			buffer.WriteString(fmt.Sprintf("\n>>> MATCH %v\n", event))

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

	return buffer.String(), nil
}

func wordSearch(ctx context.Context, client *datastore.Client, words []string) ([]*datastore.Key, error) {
	// Map prevents duplicate results.
	keymap := make(map[string]*datastore.Key)
	for _, word := range words {
		// Kindless queries aren't supported with property filters.
		for _, kind := range kinds {
			query := datastore.NewQuery(kind)
			query = query.FilterEntity(datastore.PropertyFilter{FieldName: "words", Operator: ">=", Value: word})
			query = query.FilterEntity(datastore.PropertyFilter{FieldName: "words", Operator: "<=", Value: word + "~"})
			query = query.KeysOnly()
			keys, err := client.GetAll(ctx, query, Entity{})
			if err != nil {
				return nil, fmt.Errorf("failed to fetch keys for kind %s, word %s: %v", kind, word, err)
			}
			// log.Printf("word=%s kind=%s => keys=%q", word, kind, keys)

			for _, key := range keys {
				// Convert keys to root `Person` key.
				for key.Parent != nil {
					key = key.Parent
				}
				keymap[key.Encode()] = key
			}
		}
	}

	uniquekeys := make([]*datastore.Key, 0, len(keymap))
	for _, k := range keymap {
		uniquekeys = append(uniquekeys, k)
	}

	return uniquekeys, nil
}

func removeEmtpy(words []string) []string {
	var result []string
	for _, str := range words {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}

func searchHandler(ctx context.Context, client *datastore.Client, q string) (string, error) {
	var buffer bytes.Buffer

	q = strings.TrimSpace(strings.ToLower(q))
	words := WORDS_RE.Split(q, -1)
	words = removeEmtpy(words)

	keys, err := wordSearch(ctx, client, words)
	if err != nil {
		return "", fmt.Errorf("failed to get people keys from query: %v", err)
	}

	var entities = make([]Entity, len(keys))
	err = client.GetMulti(ctx, keys, entities)
	if err != nil {
		if merr, ok := err.(datastore.MultiError); ok {
			for i, err := range merr {
				if err != nil {
					return "", fmt.Errorf("key[%d]=%q => err=%v", i, keys[i], err)
				}
			}
		}
		return "", fmt.Errorf("failed to fetch entities with keys %q: %v", keys, err)
	}

	buffer.WriteString(preamble(ctx, q))
	buffer.WriteString(fmt.Sprintf("<div>%d result(s) for: %q</div>", len(entities), words))
	for _, entity := range entities {
		resp, err := renderPersonView(ctx, client, &entity)
		if err != nil {
			return "", fmt.Errorf("failed to render person view: %v", err)
		}
		buffer.WriteString(resp)
	}
	buffer.WriteString(postamble(ctx))

	return buffer.String(), nil
}

func addTask(ctx context.Context, task *taskqueue.Task) (string, error) {
	if isDev() {
		return fmt.Sprintf("*** dev mode *** Not adding task: %s", task.Path), nil
	} else {
		task, err := taskqueue.Add(ctx, task, "")
		if err != nil {
			return "", fmt.Errorf("failed to add task %v: %v", task.Path, err)
		}

		return fmt.Sprintf("Added task: %s", task.Path), nil
	}
}

func addTasks(ctx context.Context, tasks []*taskqueue.Task) (string, error) {
	if isDev() {
		return fmt.Sprintf("*** dev mode *** Not adding %d tasks", len(tasks)), nil
	} else {
		tasks, err := taskqueue.AddMulti(ctx, tasks, "")
		if err != nil {
			return "", fmt.Errorf("failed to add %v tasks: %v", len(tasks), err)
		}

		return fmt.Sprintf("Added %d tasks", len(tasks)), nil
	}
}

func fixPersonHandler(ctx context.Context, client *datastore.Client, key string) (string, error) {
	var buffer bytes.Buffer

	dbkey, err := datastore.DecodeKey(key)
	if err != nil {
		return "", fmt.Errorf("failed to decode person key %q: %v", key, err)
	}

	// Results include ancestor Person and all descendents.
	query := datastore.NewQuery("").Ancestor(dbkey)
	var entities []Entity
	_, err = client.GetAll(ctx, query, &entities)
	if err != nil {
		return "", fmt.Errorf("failed to fetch all to be fixed entities: %v", err)
	}

	buffer.WriteString(fmt.Sprintf("Fixing %d entities:\n", len(entities)))
	keys := make([]*datastore.Key, len(entities))
	for i, e := range entities {
		keys[i] = e.Key
		buffer.WriteString(fmt.Sprintf("%4d: %v\n", i+1, e.Key))
		before := fmt.Sprintf("%v", e)
		e.fix()

		after := fmt.Sprintf("%v", e)
		if before == after {
			buffer.WriteString("Same")
		} else {
			buffer.WriteString(fmt.Sprintf("Before: %v\n", before))
			buffer.WriteString(fmt.Sprintf("After : %v\n", after))
		}
		buffer.WriteString(fmt.Sprint(strings.Repeat("\n", 10)))

		entities[i] = e
	}
	_, err = client.PutMulti(ctx, keys, entities)
	if err != nil {
		return "", fmt.Errorf("failed to put %d fixed entities: %v", len(entities), err)
	}

	buffer.WriteString("Done")
	return buffer.String(), nil
}

func fixAllHandler(ctx context.Context, client *datastore.Client, next string) (string, error) {
	var buffer bytes.Buffer

	// https://cloud.google.com/appengine/docs/standard/quotas#Task_Queue
	MAX_TASKS_PER_BATCH := 100

	query := datastore.NewQuery("Person")
	query = query.Limit(MAX_TASKS_PER_BATCH)

	if next != "" {
		key, err := datastore.DecodeKey(next)
		if err != nil {
			return "", fmt.Errorf("failed to decode person key %q: %v", next, err)
		}
		query = query.FilterField("__key__", ">", key)
	}

	var people []Entity
	_, err := client.GetAll(ctx, query, &people)
	if err != nil {
		return "", fmt.Errorf("failed to fetch person entities: %v", err)
	}

	if len(people) == MAX_TASKS_PER_BATCH {
		next := people[MAX_TASKS_PER_BATCH-1].Key
		path, err := url.JoinPath("/task/fix", "all", next.Encode())
		if err != nil {
			return "", fmt.Errorf("failed to join path: %v", err)
		}
		buffer.WriteString(fmt.Sprintf("Adding continuation task: %v\n", path))
		task := taskqueue.NewPOSTTask(path, nil)
		resp, err := addTask(ctx, task)
		if err != nil {
			return "", fmt.Errorf("failed to add continuation task with key %v: %v", next, err)
		}
		buffer.WriteString(resp)
	}

	buffer.WriteString(fmt.Sprintf("\n\nCreating %d tasks:\n", len(people)))
	tasks := make([]*taskqueue.Task, len(people))
	for i, person := range people {
		path, err := url.JoinPath("/task/fix", "Person", person.Key.Encode())
		if err != nil {
			return "", fmt.Errorf("failed to join person path: %v", err)
		}
		tasks[i] = taskqueue.NewPOSTTask(path, nil)
		buffer.WriteString(fmt.Sprintf("%4d: %v => %v  %v\n", i+1, tasks[i].Path, person.Key, person.displayName()))
	}
	resp, err := addTasks(ctx, tasks)
	if err != nil {
		return "", fmt.Errorf("failed to add tasks: %v", err)
	}
	buffer.WriteString(resp)

	return buffer.String(), nil
}

func fixHandler(r *http.Request, ctx context.Context, client *datastore.Client) (string, error) {
	// Expect one of:
	// - `/task/fix/all/`         => []string{ "" "task" "fix" "all" "" }
	// - `/task/fix/all/<key>`    => []string{ "" "task" "fix" "Person" "<key>" }
	// - `/task/fix/Person/<key>` => []string{ "" "task" "fix" "Person" "<key>" }
	segments := strings.Split(r.URL.Path, "/")

	if len(segments) != 5 {
		return "", fmt.Errorf("invalid segments: %q", segments)
	}

	switch segments[3] {
	case "Person":
		return fixPersonHandler(ctx, client, segments[4])
	case "all":
		return fixAllHandler(ctx, client, segments[4])
	default:
		return "", fmt.Errorf("unhandled path segment %s for path: %s", segments[1], r.URL.Path)
	}
}

func viewEntity(ctx context.Context, client *datastore.Client, entity *Entity) (string, error) {
	var buffer bytes.Buffer

	buffer.WriteString(preamble(ctx, ""))
	personview, err := renderPersonView(ctx, client, entity)
	if err != nil {
		return "", fmt.Errorf("failed to render person view: %v", err)
	}
	buffer.WriteString(personview)
	buffer.WriteString(postamble(ctx))

	return buffer.String(), nil
}

func editEntity(ctx context.Context, entity *Entity) string {
	var buffer bytes.Buffer

	buffer.WriteString(preamble(ctx, ""))
	buffer.WriteString(form(ctx, entity))
	buffer.WriteString(postamble(ctx))

	return buffer.String()
}

func mainPageHandler(r *http.Request, ctx context.Context, client *datastore.Client) (string, error) {
	var buffer bytes.Buffer

	q := getValue(r, "q")
	action := getValue(r, "action")
	key := getValue(r, "key")

	if q != "" {
		resp, err := searchHandler(ctx, client, q)
		if err != nil {
			return "", fmt.Errorf("failed to search: %v", err)
		}
		buffer.WriteString(resp)
	} else {
		switch action {
		case "create":
			dbkey, err := datastore.DecodeKey(key)
			if err != nil {
				return "", fmt.Errorf("failed to decode key %q: %v", key, err)
			}
			entity := Entity{
				// Key: &datastore.Key{
				// Kind: kind,
				// },
				Key: dbkey,
			}
			buffer.WriteString(editEntity(ctx, &entity))
		case "view":
			entity, err := requestToEntity(r, ctx, client)
			if err != nil {
				return "", fmt.Errorf("unable to convert request to person: %v", err)
			}
			buffer.WriteString(preamble(ctx, q))
			personview, err := renderPersonView(ctx, client, entity)
			if err != nil {
				return "", fmt.Errorf("failed to render person view: %v", err)
			}
			buffer.WriteString(personview)
			buffer.WriteString(postamble(ctx))
		case "edit":
			entity, err := requestToEntity(r, ctx, client)
			if err != nil {
				return "", fmt.Errorf("unable to convert request to entity: %v", err)
			}

			if r.Method == "POST" {
				entity.fix()
				dbkey, err := entity.save(ctx, client)
				if err != nil {
					return "", fmt.Errorf("unable to save entity: %v", err)
				}
				entity.Key = dbkey

				// Always display root entity.
				if dbkey.Parent != nil {
					dbkey = dbkey.Parent
					err = client.Get(ctx, dbkey, entity)
					if err != nil {
						return "", fmt.Errorf("failed to get parent entity: %v", err)
					}
				}
				resp, err := viewEntity(ctx, client, entity)
				if err != nil {
					return "", fmt.Errorf("failed to view entity: %v", err)
				}
				buffer.WriteString(resp)
			} else {
				buffer.WriteString(editEntity(ctx, entity))
			}
		default:
			buffer.WriteString(preamble(ctx, q))
			buffer.WriteString(postamble(ctx))
		}
	}

	return buffer.String(), nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// App Engine context for the in-flight HTTP request.
	ctx := appengine.NewContext(r)

	client, err := datastore.NewClient(ctx, projectID())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create client: %v", err), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	if r.URL.Path == "/mailmerge" {
		resp, err := mailmergeHandler(ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to json export %q: %v", r.URL.Path, err), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, resp)
		return
	}

	// Runs daily from `cron.yaml`, or manually from admin link.
	if r.URL.Path == "/task/notify" {
		resp, err := tasknotifyHandler(ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to handle task %q: %v", r.URL.Path, err), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, resp)
		return
	}

	// Fix entities.
	if strings.HasPrefix(r.URL.Path, "/task/fix") {
		resp, err := fixHandler(r, ctx, client)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed fix handler: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, resp)
		return
	}

	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to parse form: %v", err), http.StatusInternalServerError)
			return
		}
	}

	resp, err := mainPageHandler(r, ctx, client)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to render main page: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, resp)
}

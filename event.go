package orgreminders

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"
	"bytes"
	"html/template"
	"time"
)

type Event struct {
	Orgs         []string
	Created      time.Time
	Saved        time.Time
	Due          time.Time
	DueFormatted string
	Title        string
	EmailMessage template.HTML
	TextMessage  string
	Submitter    user.User
	Email        bool
	Text         bool
	Reminders    Schedule
}

func NewEvent() Event {
	event := Event{
		Created: time.Now().UTC(),
	}

	return event
}

func GetAllEvents(c appengine.Context, active bool) map[string]Event {
	var dbResults []Event
	utcLoc, _ := time.LoadLocation("UTC")
	var today = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, utcLoc)
	mapResults := make(map[string]Event)
	q := datastore.NewQuery("Event")

	if active {
		q = q.Filter("Due >= ", today)
	}

	keys, err := q.GetAll(c, &dbResults)
	if err != nil {
		c.Infof("GetEvents DB lookup error: %v", err)
	}

	for indx, event := range dbResults {
		mapResults[keys[indx].Encode()] = event
	}

	return mapResults
}

func GetEventByKey(c appengine.Context, key string) (bool, Event) {
	var result = new(Event)
	var okay = false

	keyObj, decerr := datastore.DecodeKey(key)
	if decerr != nil {
		c.Infof("Invalid event key specified")
		return okay, *result
	}

	// Attempt a DB retrieve
	err := datastore.Get(c, keyObj, result)
	if err != nil {
		c.Infof("GetEventByKey DB lookup error: %v", err)
	} else {
		okay = true
	}

	return okay, *result
}

// Save an event to the database
func (e Event) Save(c appengine.Context) (bool, string) {
	var result bool

	e.Saved = time.Now().UTC()
	key := datastore.NewIncompleteKey(c, "Event", nil)
	keyNew, err := datastore.Put(c, key, &e)
	if err != nil {
		result = false
	}

	return result, keyNew.Encode()
}

func (e Event) Update(c appengine.Context, key string) bool {
	var result bool

	keyObj, decerr := datastore.DecodeKey(key)
	if decerr != nil {
		c.Infof("Invalid key specified")
		return result
	}

	e.Saved = time.Now().UTC()
	_, err := datastore.Put(c, keyObj, &e)
	if err != nil {
		result = false
	}

	return result
}

func (e Event) Notify(c appengine.Context, now bool) (sent bool) {
	// Loop through organizations for the event and send out notifications
	for _, orgname := range e.Orgs {
		var notify bool
		// Lookup organization
		o, oerr := GetOrganizationByName(c, orgname)
		if oerr != nil {
			c.Infof("Notify: Error looking up org: %s. Skipping notifications for this organization.", orgname)
			continue
		}

		// Analyze the active event and see if we need to send out a notification
		location, _ := time.LoadLocation(o.TimeZone)
		checkTime := time.Now().In(location)

		// If we are overdue, don't notify
		if e.Due.In(location).Unix() < time.Now().In(location).Unix() {
			continue
		}

		if now {
			notify = true
		} else {
			// Cycle through event reminder times and notify (or not)
			var times = e.Reminders.Times(e.Due.In(location))
			for _, ttime := range times {
				if checkTime.Truncate(time.Minute) == ttime.Truncate(time.Minute) {
					notify = true
					break
				}
			}
		}

		// Trigger notification
		if notify {
			c.Infof("Event notification triggered")
			if e.Email {
				sent = SendOrgMessage(c, o, e, "email")
			}
			if e.Text {
				sent = SendOrgMessage(c, o, e, "text")
			}
		}
	}

	return
}

func (e Event) GetHTMLView(c appengine.Context, key string) string {
	buffer := new(bytes.Buffer)
	var tmpltxt = `<label>Event Title: </label><a href="https://orgreminders.appspot.com/editevent?id=` + key + `">{{.Title}}</a>
	<br>
	<label>When Due: </label>{{.DueFormatted}}
	<br>
	<label>Organization(s): </label>{{range .Orgs}}{{.}},{{end}}
	<br>
	<label>Email enabled: </label>{{.Email}}
	<br>
	<label>Text Enabled: </label>{{.Text}}
	<br>
	<label>Email Message: </label><br><div class="msgbody">{{.EmailMessage}}</div>
	<br>
	<label>Text Message: </label><br><div class="msgbody"><pre>{{.TextMessage}}</pre></div>
	<br>`

	template, terr := template.New("foo").Parse(tmpltxt)

	if terr != nil {
		c.Infof("error parsing event html template: %v", terr)
		return ""
	}

	template.Execute(buffer, e)
	return buffer.String()
}

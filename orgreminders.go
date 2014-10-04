package orgreminders

import (
	"appengine"
	"appengine/datastore"
	"appengine/mail"
	"appengine/user"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

// App-global variables
var Templates *template.Template
var Duration_Day = 24 * time.Hour
var Duration_Week = 7 * Duration_Day
var TemplateFiles = []string{
	"tmpl/header.html",
	"tmpl/css.html",
	"tmpl/home.html",
	"tmpl/save.html",
	"tmpl/new-event.html",
	"tmpl/new-org.html",
	"tmpl/editorg.html",
	"tmpl/editevent.html",
	"tmpl/events.html",
	"tmpl/organizations.html",
	"tmpl/error.html",
	"tmpl/cron.html",
	"tmpl/new-member.html",
	"tmpl/members.html",
	"tmpl/editmember.html",
}

type Page struct {
	Error          string
	Events         map[string]Event
	Keys           []*datastore.Key
	Event2Edit     Event
	Organizations  map[string]Organization
	Org2Edit       Organization
	Org2EditKey    string
	Location       time.Location
	AllowNewOrg    bool
	SuperUser      bool
	LoggedIn       bool
	UserEmail      string
	Orgs           []string
	Members        map[string]Member
	SavedEvent     bool
	SavedOrg       bool
	SavedMember    bool
	Member2Edit    Member
	Member2EditKey string
	ScheduleHTML   map[string][]string
}

func NewPage(u *User) (*Page, error) {
	var result = Page{}

	if u.Meta != nil {
		result.LoggedIn = true
		result.AllowNewOrg = true
		result.UserEmail = u.Meta.Email

		if u.SuperUser {
			result.SuperUser = true
		}
	}

	return &result, nil
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := Templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func init() {
	templTest, err := template.ParseFiles(TemplateFiles...)
	if err != nil {
		log.Println("Some (or all) of the required templates are missing, exiting: ", err.Error())
		return
	}

	Templates = templTest

	http.HandleFunc("/", DefaultHandler)
	http.HandleFunc("/newevent", NewEventHandler)
	http.HandleFunc("/neworg", NewOrgHandler)
	http.HandleFunc("/events", EventsHandler)
	http.HandleFunc("/organizations", OrgsHandler)
	http.HandleFunc("/saveevent", EventSaveHandler)
	http.HandleFunc("/saveorg", OrgSaveHandler)
	http.HandleFunc("/editorg", OrgEditHandler)
	http.HandleFunc("/editevent", EventEditHandler)
	http.HandleFunc("/cron", CronHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/newmember", NewMemberHandler)
	http.HandleFunc("/savemember", MemberSaveHandler)
	http.HandleFunc("/members", MembersHandler)
	http.HandleFunc("/editmember", MemberEditHandler)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	url, _ := user.LogoutURL(c, "/")
	http.Redirect(w, r, url, http.StatusFound)
}

func DefaultHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)

	title := "home"
	renderTemplate(w, title, p)
}

func NewOrgHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)

	title := "new-org"
	renderTemplate(w, title, p)
}

func NewEventHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)

	title := "new-event"
	for _, org := range u.Orgs {
		p.Orgs = append(p.Orgs, org.Name)
	}

	sort.Strings(p.Orgs)
	renderTemplate(w, title, p)
}

func EventSaveHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)
	c := appengine.NewContext(r)

	r.ParseForm()

	event := NewEvent()
	event.Title = r.PostFormValue("title")
	event.EmailMessage = template.HTML(r.PostFormValue("emailmessage"))
	event.TextMessage = r.PostFormValue("textmessage")
	event.Submitter = *u.Meta
	event.Orgs = r.PostForm["orgs"]

	if len(event.Orgs) < 1 {
		p.Error = "You must choose an organization."
		renderTemplate(w, "error", p)
		return
	}

	if r.PostFormValue("sendemail") == "on" {
		event.Email = true
	}

	if r.PostFormValue("sendtext") == "on" {
		event.Text = true
	}

	// save reminder schedule
	var remqtys = r.PostForm["remqty[]"]
	var remtyps = r.PostForm["remtyp[]"]
	for remkey, remval := range remqtys {
		var entry = fmt.Sprintf("%s%s", remval, remtyps[remkey])
		event.Reminders.Add(entry)
	}

	o, err := GetOrganizationByName(c, event.Orgs[0])

	if err != nil {
		c.Infof("Error: %s", err.Error())
		p.Error = err.Error()
		renderTemplate(w, "error", p)
		return
	}

	location, _ := time.LoadLocation(o.TimeZone)
	const longForm = "01/02/2006 3:04pm"
	t, timeerr := time.ParseInLocation(longForm, r.PostFormValue("due"), location)
	if timeerr != nil {
		http.Error(w, "Invalid time string", http.StatusInternalServerError)
		return
	}

	event.Due = t

	event.Key = r.PostFormValue("key")
	var subject = "Event Saved: "
	if event.Key == "" {
		_, event.Key = event.Save(c)
	} else {
		event.Update(c)
		subject = "Event Updated: "
	}

	if r.PostFormValue("oncreate") == "on" {
		event.Notify(c, true)
	}

	event.DueFormatted = event.Due.In(location).Format("01/02/2006 3:04pm")
	AdminNotify(c, u.Meta.Email, subject+event.Title, "The following event was just saved: <br><br>"+event.GetHTMLView(c))

	p.Event2Edit = event
	p.SavedEvent = true

	renderTemplate(w, "save", p)
}

func EventEditHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)
	c := appengine.NewContext(r)
	var ok bool

	ok, p.Event2Edit = GetEventByKey(c, r.FormValue("id"))

	if ok {
		org, _ := GetOrganizationByName(c, p.Event2Edit.Orgs[0])
		location, _ := time.LoadLocation(org.TimeZone)
		p.Event2Edit.DueFormatted = p.Event2Edit.Due.In(location).Format("01/02/2006 3:04pm")

		uorgs := GetOrganizationsByUser(c, u.Meta.Email)
		for _, uorg := range uorgs {
			missing := true
			for _, porg := range p.Event2Edit.Orgs {
				if uorg.Name == porg {
					missing = false
					break
				}
			}

			if missing == true {
				p.Orgs = append(p.Orgs, uorg.Name)
			}
		}

		// Extract usable event reminder list
		p.ScheduleHTML = p.Event2Edit.Reminders.HTML()

		sort.Strings(p.Orgs)
		renderTemplate(w, "editevent", p)
	} else {
		p.Error = "Event not found."
		renderTemplate(w, "error", p)
	}
}

func OrgSaveHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)
	c := appengine.NewContext(r)

	org := NewOrganization()
	org.Name = r.PostFormValue("name")
	org.Description = r.PostFormValue("description")
	org.Active = true
	org.Expires = time.Now().UTC().Add(Duration_Week)
	org.Administrator = strings.Split(r.PostFormValue("admin"), "\r\n")
	org.TimeZone = r.PostFormValue("timezone")

	key := r.PostFormValue("key")
	if key == "" {
		c.Infof("saving org")
		org.Save(c)
	} else {
		c.Infof("updating org")
		org.Update(c, key)
	}

	p.SavedOrg = true
	p.Org2Edit = org
	p.Org2EditKey = key
	renderTemplate(w, "save", p)
}

func OrgEditHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)
	c := appengine.NewContext(r)

	p.Org2EditKey = r.FormValue("id")
	p.Org2Edit = GetOrganizationByKey(c, p.Org2EditKey)

	renderTemplate(w, "editorg", p)
}

func EventsHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)
	p.Events = make(map[string]Event)
	c := appengine.NewContext(r)

	for _, org := range u.Orgs {
		events := org.GetEvents(c, true)
		location, _ := time.LoadLocation(org.TimeZone)

		for indx, event := range events {
			event.Due = event.Due.In(location)
			event.DueFormatted = event.Due.Format("01/02/2006 3:04pm")
			p.Events[indx] = event
		}
	}

	renderTemplate(w, "events", p)
}

func OrgsHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)
	c := appengine.NewContext(r)
	mapResults := make(map[string]Organization)

	for indx, org := range u.Orgs {
		org.Members = org.GetMembers(c)
		mapResults[indx] = org
	}
	p.Organizations = mapResults

	renderTemplate(w, "organizations", p)
}

func MembersHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)
	p.Members = make(map[string]Member)
	c := appengine.NewContext(r)

	for _, org := range u.Orgs {
		members := org.GetMembers(c)

		for indx, member := range members {
			p.Members[indx] = member
		}
	}

	renderTemplate(w, "members", p)
}

func NewMemberHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)

	title := "new-member"
	for _, org := range u.Orgs {
		p.Orgs = append(p.Orgs, org.Name)
	}

	sort.Strings(p.Orgs)
	renderTemplate(w, title, p)
}

func MemberEditHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)
	c := appengine.NewContext(r)
	var ok bool

	p.Member2EditKey = r.FormValue("id")
	ok, p.Member2Edit = GetMemberByKey(c, p.Member2EditKey)

	// Protect web users
	if ok && p.Member2Edit.WebUser {
		if u.Meta.Email != p.Member2Edit.Email && u.SuperUser == false {
			ok = false
		}
	}

	if ok {
		uorgs := GetOrganizationsByUser(c, u.Meta.Email)
		for _, uorg := range uorgs {
			missing := true
			for _, porg := range p.Member2Edit.Orgs {
				if uorg.Name == porg {
					missing = false
					break
				}
			}

			if missing == true {
				p.Orgs = append(p.Orgs, uorg.Name)
			}
		}

		sort.Strings(p.Orgs)
		renderTemplate(w, "editmember", p)
	} else {
		p.Error = "Member not found or access denied."
		renderTemplate(w, "error", p)
	}
}

func MemberSaveHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)
	c := appengine.NewContext(r)

	r.ParseForm()

	member := Member{}
	member.Name = r.PostFormValue("name")
	member.Email = r.PostFormValue("email")
	member.Cell = r.PostFormValue("cell")
	member.Carrier = r.PostFormValue("carrier")
	member.TextAddr = GenTextAddr(member.Cell, member.Carrier)
	member.Orgs = r.PostForm["orgs"]

	if r.PostFormValue("emailon") == "on" {
		member.EmailOn = true
	}

	if r.PostFormValue("texton") == "on" {
		member.TextOn = true
	}

	if u.SuperUser && r.PostFormValue("webuser") == "on" {
		member.WebUser = true
	}

	// Must have or don't save
	if len(r.PostForm["orgs"]) <= 0 && member.WebUser == false {
		p.Error = "Cannot save without an organization."
		renderTemplate(w, "error", p)
		return
	}

	key := r.PostFormValue("key")
	if key == "" {
		c.Infof("saving member")
		_, key = member.Save(c)
	} else {
		c.Infof("updating member")
		member.Update(c, key)
	}

	p.Member2Edit = member
	p.Member2EditKey = key
	p.SavedMember = true
	renderTemplate(w, "save", p)
}

func AdminNotify(c appengine.Context, creator string, subject string, message string) {
	var appid = appengine.AppID(c)
	msg := &mail.Message{
		Sender:   "orgreminders@" + appid + ".appspotmail.com",
		Subject:  subject,
		HTMLBody: message,
		To:       []string{creator},
	}

	c.Infof("notify (%s): %v", subject, creator)

	if err := mail.Send(c, msg); err != nil {
		c.Errorf("Couldn't send email: %v", err)
	}
}

func SendOrgMessage(c appengine.Context, o Organization, e Event, t string) (result bool) {
	var appid = appengine.AppID(c)
	var senderUserName = strings.Replace(o.Name, " ", "_", -1)
	var sender = fmt.Sprintf("%s Reminders <%s@%s.appspotmail.com", o.Name, senderUserName, appid)
	members := o.GetMembers(c)
	recipients := []string{}

	for _, m := range members {
		if t == "email" && m.EmailOn {
			recipients = append(recipients, m.Email)
		} else if t == "text" && m.TextOn {
			recipients = append(recipients, m.TextAddr)
		}
	}

	if len(recipients) == 0 {
		c.Infof("No recipients, not sending reminder (" + t + ")")
		result = true
		return
	}

	// get rid of duplicate recipients
	recipients = removeDuplicates(recipients)

	msg := &mail.Message{
		Sender:   sender,
		Bcc:      recipients,
		Subject:  e.Title,
		Body:     e.TextMessage,
		HTMLBody: string(e.EmailMessage),
	}

	c.Infof("notify (%s): %v", e.Title, recipients)
	if err := mail.Send(c, msg); err != nil {
		c.Errorf("Couldn't send email: %v", err)
	} else {
		result = true
	}

	return
}

func CronHandler(w http.ResponseWriter, r *http.Request) {
	u := UserLookup(w, r)
	p, _ := NewPage(&u)
	p.Events = make(map[string]Event)
	c := appengine.NewContext(r)

	events := GetAllEvents(c, true) // active only
	//c.Infof("# events to check for cron: %v", len(events))
	for key, event := range events {
		//c.Infof("checking event: %s", event.Title)
		res := event.Notify(c, false)
		if res {
			org, _ := GetOrganizationByName(c, event.Orgs[0])
			location, _ := time.LoadLocation(org.TimeZone)
			event.Due = event.Due.In(location)
			event.DueFormatted = event.Due.Format("01/02/2006 3:04pm")
			p.Events[key] = event
		}
	}

	renderTemplate(w, "cron", p)
}

// from: https://groups.google.com/d/msg/golang-nuts/-pqkICuokio/KqJ0091EzVcJ
func removeDuplicates(a []string) []string {
	result := []string{}
	seen := map[string]string{}
	for _, val := range a {
		if _, ok := seen[val]; !ok {
			result = append(result, val)
			seen[val] = val
		}
	}
	return result
}

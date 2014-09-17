package orgreminders

import (
	"appengine"
	"appengine/datastore"
	"errors"
	"sort"
	"time"
)

type Organization struct {
	Name          string
	Description   string
	Saved         time.Time
	Created       time.Time
	Active        bool
	Expires       time.Time
	TimeZone      string
	Administrator []string
	Members       map[string]Member `datastore:"-"`
}

type Organizations []Organization

func NewOrganization() Organization {
	org := Organization{
		Created: time.Now().UTC(),
	}

	return org
}

func GetAllOrganizations(c appengine.Context) (dbResults []Organization, err error) {
	q := datastore.NewQuery("Organization")
	_, err = q.GetAll(c, &dbResults)

	if err != nil {
		c.Infof("GetAllOrganizations DB lookup error: %v", err)
		err = errors.New("DB lookup error")
	}

	return dbResults, err
}

// Retrieve an organization from the database
func GetOrganizationByName(c appengine.Context, n string) (result Organization, err error) {
	// Attempt a DB retrieve
	var dbResults []Organization

	q := datastore.NewQuery("Organization").Filter("Name = ", n)
	_, err = q.GetAll(c, &dbResults)
	if err != nil {
		c.Infof("GetOrganizationByName DB lookup error: %v", err)
		err = errors.New("DB lookup error")
	}

	if len(dbResults) > 1 {
		c.Infof("Multiple organizations match search criteria (name=%s), returning first one.", n)
	} else if len(dbResults) == 0 {
		err = errors.New("No results found")
	}

	if len(dbResults) >= 1 {
		result = dbResults[0]
	}

	return
}

func GetOrganizationByKey(c appengine.Context, key string) Organization {
	var result = new(Organization)
	keyObj, decerr := datastore.DecodeKey(key)
	if decerr != nil {
		c.Infof("Invalid org key specified")
		return *result
	}

	// Attempt a DB retrieve
	err := datastore.Get(c, keyObj, result)

	if err != nil {
		c.Infof("GetOrganizationByKey DB lookup error: %v", err)
	}

	return *result
}

func GetOrganizationsByUser(c appengine.Context, u string) map[string]Organization {
	// Attempt a DB retrieve
	var dbResults []Organization
	mapResults := make(map[string]Organization)

	q := datastore.NewQuery("Organization").Filter("Administrator = ", u)
	keys, err := q.GetAll(c, &dbResults)
	if err != nil {
		c.Infof("GetOrganizationsByUser DB lookup error: %v", err)
	}

	for indx, org := range dbResults {
		mapResults[keys[indx].Encode()] = org
	}

	return mapResults
}

// Save an organization to the database
func (o Organization) Save(c appengine.Context) bool {
	var result bool

	key := datastore.NewIncompleteKey(c, "Organization", nil)

	o.Saved = time.Now().UTC()
	_, err := datastore.Put(c, key, &o)
	if err != nil {
		c.Infof("org.Save error: %v", err)
		result = false
	}

	return result
}

func (o Organization) Update(c appengine.Context, key string) bool {
	var result bool

	keyObj, decerr := datastore.DecodeKey(key)
	if decerr != nil {
		c.Infof("Invalid key specified")
		return result
	}

	o.Saved = time.Now().UTC()
	_, err := datastore.Put(c, keyObj, &o)
	if err != nil {
		result = false
	}

	return result
}

func (o Organization) GetEvents(c appengine.Context, active bool) map[string]Event {
	// Attempt a DB retrieve
	var dbResults []Event
	mapResults := make(map[string]Event)
	q := datastore.NewQuery("Event").Filter("Orgs = ", o.Name)
	utcLoc, _ := time.LoadLocation("UTC")
	var today = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, utcLoc)

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

func (o Organization) GetMembers(c appengine.Context) map[string]Member {
	// Attempt a DB retrieve
	var dbResults Members
	mapResults := make(map[string]Member)
	q := datastore.NewQuery("Member").Filter("Orgs = ", o.Name).Order("Name")

	keys, err := q.GetAll(c, &dbResults)
	if err != nil {
		c.Infof("GetMembers DB lookup error: %v", err)
	}

	// Sort our list of members by Name
	sort.Sort(dbResults)

	for indx, member := range dbResults {
		member.Key = keys[indx].Encode()
		mapResults[member.Name] = member
	}

	return mapResults
}

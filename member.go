package orgreminders

import (
	"appengine"
	"appengine/datastore"
	"errors"
	"regexp"
)

type Member struct {
	Key      string `datastore:"-"`
	Name     string
	Email    string
	Cell     string
	Carrier  string
	TextAddr string
	TextOn   bool
	EmailOn  bool
	Orgs     []string
	WebUser  bool
}

type Members []Member

func (slice Members) Len() int {
	return len(slice)
}

func (slice Members) Less(i, j int) bool {
	return slice[i].Name < slice[j].Name
}

func (slice Members) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func GenTextAddr(cell string, carrier string) (addr string) {
	rp := regexp.MustCompile("[^\\d]")
	number := rp.ReplaceAllString(cell, "")
	var suffix string

	switch carrier {
	case "att":
		suffix = "txt.att.net"
	case "sprint":
		suffix = "messaging.sprintpcs.com"
	case "verizon":
		suffix = "vtext.com"
	case "tmobile":
		suffix = "tmomail.net"
	default:
		return
	}

	addr = number + "@" + suffix
	return
}

func GetMemberByEmail(c appengine.Context, email string) (result Member, err error) {
	var dbResults []Member

	q := datastore.NewQuery("Member").Filter("Email = ", email)
	_, err = q.GetAll(c, &dbResults)
	if err != nil {
		c.Infof("DB lookup error: %v", err)
		err = errors.New("DB lookup error")
	}

	if len(dbResults) > 1 {
		c.Infof("Multiple members match search criteria (name=%s), returning first one.", email)
	} else if len(dbResults) == 0 {
		err = errors.New("No results found")
	}

	if len(dbResults) >= 1 {
		result = dbResults[0]
	}

	return
}

func GetMemberByKey(c appengine.Context, key string) (bool, Member) {
	var result = new(Member)
	var okay = false

	keyObj, decerr := datastore.DecodeKey(key)
	if decerr != nil {
		c.Infof("Invalid org key specified")
		return okay, *result
	}

	// Attempt a DB retrieve
	err := datastore.Get(c, keyObj, result)

	if err != nil {
		c.Infof("GetMemberByKey DB lookup error: %v", err)
	} else {
		okay = true
	}

	return okay, *result
}

func (m Member) Save(c appengine.Context) (bool, string) {
	var result bool

	key := datastore.NewIncompleteKey(c, "Member", nil)
	keyNew, err := datastore.Put(c, key, &m)
	if err != nil {
		result = false
	}

	return result, keyNew.Encode()
}

func (m Member) Update(c appengine.Context, key string) bool {
	var result bool

	keyObj, decerr := datastore.DecodeKey(key)
	if decerr != nil {
		c.Infof("Invalid key specified")
		return result
	}

	_, err := datastore.Put(c, keyObj, &m)
	if err != nil {
		result = false
	}

	return result
}

func GetWebMembers(c appengine.Context) (dbResults []Member, err error) {
	q := datastore.NewQuery("Member").Filter("WebUser = ", true)
	_, err = q.GetAll(c, &dbResults)

	if err != nil {
		c.Infof("GetWebMembers DB lookup error: %v", err)
		err = errors.New("DB lookup error")
	}

	return dbResults, err
}

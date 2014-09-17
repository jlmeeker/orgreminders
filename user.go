package orgreminders

import (
	"appengine"
	"appengine/user"
	"net/http"
)

type User struct {
	Meta      *user.User
	Orgs      map[string]Organization
	SuperUser bool
}

func UserLookup(w http.ResponseWriter, r *http.Request) User {
	c := appengine.NewContext(r)
	u := User{}
	authuser := user.Current(c)
	var allowed bool

	if authuser != nil {
		if authuser.Admin {
			allowed = true
		} else {
			webmembers, err := GetWebMembers(c)
			if err == nil {
				for _, member := range webmembers {
					if authuser.Email == member.Email {
						allowed = true
					}
				}
			}
		}
	}

	if allowed {
		if authuser != nil {
			u.Meta = authuser
			u.Orgs = GetOrganizationsByUser(c, u.Meta.Email)

			if u.Meta.Admin {
				u.SuperUser = true
			}
		}
	} else {
		if authuser != nil {
			url, _ := user.LogoutURL(c, "/")
			http.Redirect(w, r, url, http.StatusFound)
		}
	}

	return u
}

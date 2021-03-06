package controller

import (
	"fmt"
	"net/http"
	"net/url"

	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"
)

func WhoAmIHandler(w http.ResponseWriter, r *http.Request) {
	c, status, err := models.MakeContext(r, w)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	ctl := WhoAmIController{}

	switch c.GetHttpMethod() {
	case "OPTIONS":
		c.RespondWithOptions([]string{"OPTIONS", "GET"})
		return
	case "GET":
		ctl.Read(c)
	default:
		c.RespondWithStatus(http.StatusMethodNotAllowed)
		return
	}
}

type WhoAmIController struct{}

func (wc *WhoAmIController) Read(c *models.Context) {

	if c.Request.Method != "GET" {
		c.RespondWithNotImplemented()
		return
	}

	if c.Auth.UserId < 0 {
		c.RespondWithErrorMessage(
			"Bad access token supplied",
			http.StatusForbidden,
		)
		return
	}

	if c.Auth.UserId == 0 {
		c.RespondWithErrorMessage(
			"You must be authenticated to ask 'who am I?'",
			http.StatusForbidden,
		)
		return
	}

	m, status, err := models.GetProfileSummary(c.Site.Id, c.Auth.ProfileId)
	if err != nil {
		if status == http.StatusNotFound {
			c.RespondWithErrorMessage(
				"You must create a user profile for this site at api/v1/profiles/",
				http.StatusNotFound,
			)
			return
		} else {
			c.RespondWithErrorMessage(
				fmt.Sprintf("Could not retrieve profile: %v", err.Error()),
				http.StatusInternalServerError,
			)
			return
		}
	}

	location := fmt.Sprintf(
		"%s/%d",
		h.ApiTypeProfile,
		m.Id,
	)

	if c.Auth.ProfileId > 0 && c.Auth.Method == "query" {
		u, _ := url.Parse(location)
		qs := u.Query()
		qs.Del("access_token")
		qs.Add("access_token", c.Auth.AccessToken.TokenValue)
		u.RawQuery = qs.Encode()
		location = u.String()
	}

	c.ResponseWriter.Header().Set("Location", location)
	c.RespondWithStatus(307)
}

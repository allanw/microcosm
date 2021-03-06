package controller

import (
	"net/http"

	"github.com/microcosm-cc/microcosm/models"
)

type SearchController struct{}

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	c, status, err := models.MakeContext(r, w)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	ctl := SearchController{}

	switch c.GetHttpMethod() {
	case "OPTIONS":
		c.RespondWithOptions([]string{"OPTIONS", "HEAD", "GET"})
		return
	case "GET":
		ctl.Read(c)
	case "HEAD":
		ctl.Read(c)
	default:
		c.RespondWithStatus(http.StatusMethodNotAllowed)
		return
	}
}

// Returns information about a single site, the one related to this HTTP context
func (ctl *SearchController) Read(c *models.Context) {

	results, status, err := models.Search(
		c.Site.Id,
		*c.Request.URL,
		c.Auth.ProfileId,
	)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	c.ResponseWriter.Header().Set("Cache-Control", `no-cache, max-age=0`)

	c.RespondWithData(results)
}

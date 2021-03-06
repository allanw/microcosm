package controller

import (
	"net/http"
	"strconv"

	"github.com/microcosm-cc/microcosm/models"
)

type SiteHostController struct{}

func SiteHostHandler(w http.ResponseWriter, r *http.Request) {
	c, status, err := models.MakeContext(r, w)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}
	ctl := SiteHostController{}

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

// Read responds with a header and body containing microcosm host for site.
func (ctl *SiteHostController) Read(c *models.Context) {
	host, exists := c.RouteVars["host"]
	if !exists {
		c.RespondWithErrorMessage("No host query specified", http.StatusBadRequest)
		return
	}

	// This could be further optimised by only caching host -> microcosm subdomain.
	site, status, err := models.GetSiteByDomain(host)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}
	microcosmHost := site.SubdomainKey + ".microco.sm"

	contentLen := len(microcosmHost)
	c.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(contentLen))
	c.ResponseWriter.Header().Set("Content-Type", "text/plain")
	c.ResponseWriter.Header().Set("X-Microcosm-Host", microcosmHost)

	// Calling Write automatically sets HTTP status code to 200.
	c.ResponseWriter.Write([]byte(microcosmHost))
}

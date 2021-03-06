package controller

import (
	"net/http"

	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"
)

func UpdatesHandler(w http.ResponseWriter, r *http.Request) {
	c, status, err := models.MakeContext(r, w)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	ctl := UpdatesController{}

	switch c.GetHttpMethod() {
	case "OPTIONS":
		c.RespondWithOptions([]string{"OPTIONS", "HEAD", "GET"})
		return
	case "HEAD":
		ctl.ReadMany(c)
	case "GET":
		ctl.ReadMany(c)
	default:
		c.RespondWithStatus(http.StatusMethodNotAllowed)
		return
	}
}

type UpdatesController struct{}

func (ctl *UpdatesController) ReadMany(c *models.Context) {

	if c.Auth.ProfileId < 1 {
		c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
		return
	}

	limit, offset, status, err := h.GetLimitAndOffset(c.Request.URL.Query())
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	ems, total, pages, status, err := models.GetUpdates(c.Site.Id, c.Auth.ProfileId, limit, offset)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	thisLink := h.GetLinkToThisPage(*c.Request.URL, offset, limit, total)
	m := models.UpdatesType{}
	m.Updates = h.ConstructArray(
		ems,
		h.ApiTypeUpdate,
		total,
		limit,
		offset,
		pages,
		c.Request.URL,
	)
	m.Meta.Links =
		[]h.LinkType{
			h.LinkType{Rel: "self", Href: thisLink.String()},
		}

	c.ResponseWriter.Header().Set("Cache-Control", "no-cache, max-age=0")
	c.RespondWithData(m)
}

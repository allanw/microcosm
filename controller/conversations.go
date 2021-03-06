package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/microcosm-cc/microcosm/audit"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"
)

type ConversationsController struct{}

func ConversationsHandler(w http.ResponseWriter, r *http.Request) {
	c, status, err := models.MakeContext(r, w)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	ctl := ConversationsController{}

	switch c.GetHttpMethod() {
	case "OPTIONS":
		c.RespondWithOptions([]string{"OPTIONS", "GET", "HEAD", "POST"})
		return
	case "GET":
		ctl.ReadMany(c)
	case "HEAD":
		ctl.ReadMany(c)
	case "POST":
		ctl.Create(c)
	default:
		c.RespondWithStatus(http.StatusMethodNotAllowed)
		return
	}
}

// Returns an array of conversations
func (ctl *ConversationsController) ReadMany(c *models.Context) {

	// Start Authorisation
	perms := models.GetPermission(
		models.MakeAuthorisationContext(
			c, 0, h.ItemTypes[h.ItemTypeConversation], 0),
	)
	if !perms.CanRead {
		c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
		return
	}
	// End Authorisation

	// Fetch query string args if any exist
	limit, offset, status, err := h.GetLimitAndOffset(c.Request.URL.Query())
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	ems, total, pages, status, err := models.GetConversations(c.Site.Id, c.Auth.ProfileId, limit, offset)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	// Construct the response
	thisLink := h.GetLinkToThisPage(*c.Request.URL, offset, limit, total)

	m := models.ConversationsType{}
	m.Conversations = h.ConstructArray(
		ems,
		h.ApiTypeConversation,
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
	m.Meta.Permissions = perms

	c.ResponseWriter.Header().Set("Cache-Control", "no-cache, max-age=0")

	c.RespondWithData(m)
}

// Creates a conversations
func (ctl *ConversationsController) Create(c *models.Context) {

	// Validate inputs
	m := models.ConversationType{}
	m.Meta.Flags.Deleted = false
	m.Meta.Flags.Moderated = false
	m.Meta.Flags.Open = true
	m.Meta.Flags.Sticky = false

	err := c.Fill(&m)
	if err != nil {
		c.RespondWithErrorMessage(
			fmt.Sprintf("The post data is invalid: %v", err.Error()),
			http.StatusBadRequest,
		)
		return
	}

	// Start : Authorisation
	perms := models.GetPermission(
		models.MakeAuthorisationContext(
			c, 0, h.ItemTypes[h.ItemTypeMicrocosm], m.MicrocosmId),
	)
	if !perms.CanCreate {
		c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
		return
	}
	// End : Authorisation

	// Populate where applicable from auth and context
	m.Meta.CreatedById = c.Auth.ProfileId
	m.Meta.Created = time.Now()

	status, err := m.Insert(c.Site.Id, c.Auth.ProfileId)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	audit.Create(
		c.Site.Id,
		h.ItemTypes[h.ItemTypeConversation],
		m.Id,
		c.Auth.ProfileId,
		time.Now(),
		c.IP,
	)

	go models.SendUpdatesForNewItemInAMicrocosm(c.Site.Id, m)

	go models.RegisterWatcher(
		c.Auth.ProfileId,
		h.UpdateTypes[h.UpdateTypeNewComment],
		m.Id,
		h.ItemTypes[h.ItemTypeConversation],
		c.Site.Id,
	)

	c.RespondWithSeeOther(
		fmt.Sprintf(
			"%s/%d",
			h.ApiTypeConversation,
			m.Id,
		),
	)
}

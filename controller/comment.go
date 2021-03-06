package controller

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/lib/pq"

	"github.com/microcosm-cc/microcosm/audit"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"
)

type CommentController struct{}

func CommentHandler(w http.ResponseWriter, r *http.Request) {
	c, status, err := models.MakeContext(r, w)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	ctl := CommentController{}

	switch c.GetHttpMethod() {
	case "OPTIONS":
		c.RespondWithOptions([]string{"OPTIONS", "GET", "HEAD", "PUT", "PATCH", "DELETE"})
		return
	case "GET":
		ctl.Read(c)
	case "HEAD":
		ctl.Read(c)
	case "PUT":
		ctl.Update(c)
	case "PATCH":
		ctl.Patch(c)
	case "DELETE":
		ctl.Delete(c)
	default:
		c.RespondWithStatus(http.StatusMethodNotAllowed)
		return
	}
}

// Reads a single comment
func (ctl *CommentController) Read(c *models.Context) {
	_, itemTypeId, itemId, status, err := c.GetItemTypeAndItemId()
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	// Start Authorisation
	perms := models.GetPermission(
		models.MakeAuthorisationContext(
			c, 0, itemTypeId, itemId),
	)
	if !perms.CanRead {
		c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
		return
	}
	// End Authorisation

	limit, _, status, err := h.GetLimitAndOffset(c.Request.URL.Query())
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	m, status, err := models.GetComment(c.Site.Id, itemId, c.Auth.ProfileId, limit)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	m.Meta.Permissions = perms

	if c.Auth.ProfileId > 0 {
		go models.MarkAsRead(m.ItemTypeId, m.ItemId, c.Auth.ProfileId, m.Meta.Created)
	}

	c.ResponseWriter.Header().Set("Cache-Control", `no-cache, max-age=0`)

	c.RespondWithData(m)

}

// Updates (replaces) a single comment
func (ctl *CommentController) Update(c *models.Context) {
	_, itemTypeId, itemId, status, err := c.GetItemTypeAndItemId()
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	// Initialise
	m, status, err := models.GetCommentSummary(c.Site.Id, itemId)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	// Fill from POST data
	err = c.Fill(&m)
	if err != nil {
		c.RespondWithErrorMessage(
			fmt.Sprintf("The post data is invalid: %v", err.Error()),
			http.StatusBadRequest,
		)
		return
	}

	// Start Authorisation
	perms := models.GetPermission(
		models.MakeAuthorisationContext(
			c, 0, itemTypeId, itemId),
	)
	if !perms.CanUpdate {
		c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
		return
	}
	// End Authorisation

	// Populate where applicable from auth and context
	m.Meta.EditedByNullable = sql.NullInt64{Int64: c.Auth.ProfileId, Valid: true}
	m.Meta.EditedNullable = pq.NullTime{Time: time.Now(), Valid: true}

	// Update
	status, err = m.Update(c.Site.Id)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	audit.Replace(
		c.Site.Id,
		h.ItemTypes[h.ItemTypeComment],
		m.Id,
		c.Auth.ProfileId,
		time.Now(),
		c.IP,
	)

	// Respond
	c.RespondWithSeeOther(
		fmt.Sprintf(
			"%s/%d",
			h.ApiTypeComment,
			m.Id,
		),
	)
}

// Partially updates a comment, this is limited to changing the boolean flags only
func (ctl *CommentController) Patch(c *models.Context) {
	_, itemTypeId, itemId, status, err := c.GetItemTypeAndItemId()
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	patches := []h.PatchType{}
	err = c.Fill(&patches)
	if err != nil {
		c.RespondWithErrorMessage(
			fmt.Sprintf("The post data is invalid: %v", err.Error()),
			http.StatusBadRequest,
		)
		return
	}

	status, err = h.TestPatch(patches)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	// Start Authorisation
	ac := models.MakeAuthorisationContext(c, 0, itemTypeId, itemId)
	perms := models.GetPermission(ac)
	if !perms.CanUpdate {
		c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
		return
	}

	// All patches are 'replace'
	for _, patch := range patches {
		status, err := patch.ScanRawValue()
		if !patch.Bool.Valid {
			c.RespondWithErrorDetail(err, status)
			return
		}

		switch patch.Path {
		case "/meta/flags/deleted":
			// Only super users' can undelete, but super users' and owners can delete
			if !patch.Bool.Valid {
				c.RespondWithErrorMessage("/meta/flags/deleted requires a bool value", http.StatusBadRequest)
				return
			}
			if (patch.Bool.Bool == false && !(perms.IsModerator || perms.IsOwner)) || !perms.IsModerator {
				c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
				return
			}
		case "/meta/flags/moderated":
			if !perms.IsModerator {
				c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
				return
			}
		default:
			c.RespondWithErrorMessage("Invalid patch operation path", http.StatusBadRequest)
			return
		}
	}
	// End Authorisation

	m := models.CommentSummaryType{}
	m.Id = itemId
	status, err = m.Patch(c.Site.Id, ac, patches)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	audit.Update(
		c.Site.Id,
		h.ItemTypes[h.ItemTypeComment],
		m.Id,
		c.Auth.ProfileId,
		time.Now(),
		c.IP,
	)

	c.RespondWithOK()
}

// Deletes a single comment
func (ctl *CommentController) Delete(c *models.Context) {
	_, itemTypeId, itemId, status, err := c.GetItemTypeAndItemId()
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	// Start Authorisation
	perms := models.GetPermission(
		models.MakeAuthorisationContext(
			c, 0, itemTypeId, itemId),
	)
	if !perms.CanDelete {
		c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
		return
	}
	// End Authorisation

	// Partially instantiated type for Id passing
	m, status, err := models.GetCommentSummary(c.Site.Id, itemId)
	if err != nil {
		if status == http.StatusNotFound {
			c.RespondWithOK()
			return
		}

		c.RespondWithErrorDetail(err, status)
		return
	}

	// Delete resource
	status, err = m.Delete(c.Site.Id)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	audit.Delete(
		c.Site.Id,
		h.ItemTypes[h.ItemTypeComment],
		m.Id,
		c.Auth.ProfileId,
		time.Now(),
		c.IP,
	)

	c.RespondWithOK()
}

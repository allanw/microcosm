package controller

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/lib/pq"

	"github.com/microcosm-cc/microcosm/audit"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"
)

type AttendeeController struct{}

func AttendeeHandler(w http.ResponseWriter, r *http.Request) {
	c, status, err := models.MakeContext(r, w)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	ctl := AttendeeController{}

	switch c.GetHttpMethod() {
	case "OPTIONS":
		c.RespondWithOptions([]string{"OPTIONS", "HEAD", "GET", "PUT", "DELETE"})
		return
	case "HEAD":
		ctl.Read(c)
	case "GET":
		ctl.Read(c)
	case "PUT":
		ctl.Update(c)
	case "DELETE":
		ctl.Delete(c)
	default:
		c.RespondWithStatus(http.StatusMethodNotAllowed)
		return
	}
}

func (ctl *AttendeeController) Read(c *models.Context) {

	// Verify ID is a positive integer
	eventId, err := strconv.ParseInt(c.RouteVars["event_id"], 10, 64)
	if err != nil {
		c.RespondWithErrorMessage(
			fmt.Sprintf("The supplied event_id ('%s') is not a number.", c.RouteVars["event_id"]),
			http.StatusBadRequest,
		)
		return
	}

	profileId, err := strconv.ParseInt(c.RouteVars["profile_id"], 10, 64)
	if err != nil {
		c.RespondWithErrorMessage(
			fmt.Sprintf("The supplied profile_id ('%s') is not a number.", c.RouteVars["profile_id"]),
			http.StatusBadRequest,
		)
		return
	}

	attendeeId, status, err := models.GetAttendeeId(eventId, profileId)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	// Start Authorisation
	perms := models.GetPermission(
		models.MakeAuthorisationContext(
			c, 0, h.ItemTypes[h.ItemTypeAttendee], attendeeId),
	)
	if !perms.CanRead {
		c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
		return
	}
	// End Authorisation

	// Read Event
	m, status, err := models.GetAttendee(c.Site.Id, attendeeId)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	m.Meta.Permissions = perms

	c.ResponseWriter.Header().Set("Cache-Control", `no-cache, max-age=0`)

	c.RespondWithData(m)
}

func (ctl *AttendeeController) Update(c *models.Context) {

	// Verify ID is a positive integer
	eventId, err := strconv.ParseInt(c.RouteVars["event_id"], 10, 64)
	if err != nil {
		c.RespondWithErrorMessage(
			fmt.Sprintf("The supplied event_id ('%s') is not a number.", c.RouteVars["event_id"]),
			http.StatusBadRequest,
		)
		return
	}

	m := models.AttendeeType{}

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
			c, 0, h.ItemTypes[h.ItemTypeEvent], eventId),
	)
	if !perms.CanUpdate {
		c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
		return
	}

	if perms.IsOwner || perms.IsModerator || perms.IsSiteOwner {
		if m.ProfileId != c.Auth.ProfileId && m.RSVP == "yes" {
			c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
			return
		}
	} else {
		if m.ProfileId != c.Auth.ProfileId {
			c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
			return
		}
	}
	_, status, err := models.GetProfileSummary(c.Site.Id, m.ProfileId)
	if err != nil {
		c.RespondWithErrorMessage(h.NoAuthMessage, status)
		return
	}
	// End Authorisation

	// Populate where applicable from auth and context
	t := time.Now()
	m.EventId = eventId
	m.Meta.CreatedById = c.Auth.ProfileId
	m.Meta.Created = t
	m.Meta.EditedByNullable = sql.NullInt64{Int64: c.Auth.ProfileId, Valid: true}
	m.Meta.EditedNullable = pq.NullTime{Time: t, Valid: true}

	status, err = m.Update(c.Site.Id)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	if m.RSVP == "yes" {
		go models.SendUpdatesForNewAttendeeInAnEvent(c.Site.Id, m)
	}

	audit.Replace(
		c.Site.Id,
		h.ItemTypes[h.ItemTypeAttendee],
		m.Id,
		c.Auth.ProfileId,
		time.Now(),
		c.IP,
	)

	c.RespondWithSeeOther(
		fmt.Sprintf("%s/%d", fmt.Sprintf(h.ApiTypeAttendee, m.EventId), m.ProfileId),
	)
}

func (ctl *AttendeeController) Delete(c *models.Context) {

	// Validate inputs
	eventId, err := strconv.ParseInt(c.RouteVars["event_id"], 10, 64)
	if err != nil {
		c.RespondWithErrorMessage(
			fmt.Sprintf("The supplied event_id ('%s') is not a number.", c.RouteVars["event_id"]),
			http.StatusBadRequest,
		)
		return
	}

	profileId, err := strconv.ParseInt(c.RouteVars["profile_id"], 10, 64)
	if err != nil {
		c.RespondWithErrorMessage(
			fmt.Sprintf("The supplied profile_id ('%s') is not a number.", c.RouteVars["profile_id"]),
			http.StatusBadRequest,
		)
		return
	}

	attendeeId, status, err := models.GetAttendeeId(eventId, profileId)
	if err != nil {
		c.RespondWithOK()
		return
	}

	// Start Authorisation
	perms := models.GetPermission(
		models.MakeAuthorisationContext(
			c, 0, h.ItemTypes[h.ItemTypeAttendee], attendeeId),
	)
	if !perms.CanDelete {
		c.RespondWithErrorMessage(h.NoAuthMessage, http.StatusForbidden)
		return
	}
	// End Authorisation

	m, status, err := models.GetAttendee(c.Site.Id, attendeeId)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	// Delete resource
	status, err = m.Delete(c.Site.Id)
	if err != nil {
		c.RespondWithErrorDetail(err, status)
		return
	}

	audit.Replace(
		c.Site.Id,
		h.ItemTypes[h.ItemTypeAttendee],
		attendeeId,
		c.Auth.ProfileId,
		time.Now(),
		c.IP,
	)

	c.RespondWithOK()
}

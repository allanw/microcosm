package models

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/lib/pq"

	c "github.com/microcosm-cc/microcosm/cache"
	h "github.com/microcosm-cc/microcosm/helpers"
)

const (
	EventStatusProposed  string = "proposed"
	EventStatusUpcoming  string = "upcoming"
	EventStatusPostponed string = "postponed"
	EventStatusCancelled string = "cancelled"
	EventStatusPast      string = "past"
)

type EventsType struct {
	Events h.ArrayType    `json:"events"`
	Meta   h.CoreMetaType `json:"meta"`
}

type EventSummaryType struct {
	ItemSummary

	// Specific to events
	WhenNullable  pq.NullTime    `json:"-"`
	When          string         `json:"when,omitempty"`
	Duration      int64          `json:"duration,omitempty"`
	WhereNullable sql.NullString `json:"-"`
	Where         string         `json:"where,omitempty"`
	Lat           float64        `json:"lat,omitempty"`
	Lon           float64        `json:"lon,omitempty"`
	North         float64        `json:"north,omitempty"`
	East          float64        `json:"east,omitempty"`
	South         float64        `json:"south,omitempty"`
	West          float64        `json:"west,omitempty"`
	Status        string         `json:"status"`
	RSVPLimit     int32          `json:"rsvpLimit"`
	RSVPAttending int32          `json:"rsvpAttend,omitempty"`
	RSVPSpaces    int32          `json:"rsvpSpaces,omitempty"`

	ItemSummaryMeta
}

type EventType struct {
	ItemDetail

	// Specific to events
	WhenNullable  pq.NullTime    `json:"-"`
	When          string         `json:"when,omitempty"`
	Duration      int32          `json:"duration,omitempty"`
	Where         string         `json:"where,omitempty"`
	WhereNullable sql.NullString `json:"-"`
	Lat           float64        `json:"lat,omitempty"`
	Lon           float64        `json:"lon,omitempty"`
	North         float64        `json:"north,omitempty"`
	East          float64        `json:"east,omitempty"`
	South         float64        `json:"south,omitempty"`
	West          float64        `json:"west,omitempty"`
	Status        string         `json:"status"`
	RSVPLimit     int32          `json:"rsvpLimit"`
	RSVPAttending int32          `json:"rsvpAttend,omitempty"`
	RSVPSpaces    int32          `json:"rsvpSpaces,omitempty"`

	ItemDetailCommentsAndMeta
}

func (m *EventType) Validate(
	siteId int64,
	profileId int64,
	exists bool,
) (
	int,
	error,
) {

	m.Title = SanitiseText(m.Title)
	m.Where = SanitiseText(m.Where)
	m.Meta.EditReason = SanitiseText(m.Meta.EditReason)

	// Does the Microcosm specified exist on this site?
	if !exists {
		_, status, err := GetMicrocosmSummary(siteId, m.MicrocosmId, profileId)
		if err != nil {
			glog.Infof(`GetMicrocosmSummary error %+v`, err)
			return status, err
		}
	}

	if exists {
		if m.Meta.EditReason == `` {
			glog.Info(`No edit reason given`)
			return http.StatusBadRequest,
				errors.New("You must provide a reason for the update")
		} else {
			m.Meta.EditReason = ShoutToWhisper(m.Meta.EditReason)
		}
	}

	if m.MicrocosmId <= 0 {
		glog.Infof(`Microcosm ID (%d) <= zero`, m.MicrocosmId)
		return http.StatusBadRequest,
			errors.New("You must specify a Microcosm ID")
	}

	if m.Title == `` {
		glog.Info(`Title is a required field`)
		return http.StatusBadRequest, errors.New("Title is a required field")
	}
	m.Title = ShoutToWhisper(m.Title)

	// Default status is 'upcoming' if not specified
	if strings.Trim(m.When, ` `) == `` {
		m.Status = EventStatusProposed
	} else {
		m.Status = EventStatusUpcoming
	}

	if strings.Trim(m.When, ` `) != `` {
		eventTimestamp, err := time.Parse(time.RFC3339, m.When)
		if err != nil {
			glog.Infof(`time.Parse err for %s, %+v`, m.When, err)
			return http.StatusBadRequest, err
		}

		m.WhenNullable = pq.NullTime{Time: eventTimestamp, Valid: true}
	}

	// If no duration is specified, default to 1 hour.
	// Value is in minutes
	if m.Duration < 0 {
		m.Duration = 60 * 1
	}

	if m.Where != `` {
		m.Where = ShoutToWhisper(m.Where)
		m.WhereNullable = sql.NullString{String: m.Where, Valid: true}
	}

	if m.RSVPLimit < 0 {
		glog.Infof(`RSVPLimit (%d) below zero`, m.RSVPLimit)
		return http.StatusBadRequest,
			errors.New("RSVPLimit must be 0 (unlimited) or greater")
	}

	// If a limit is specified, there are initially the same number of
	// spaces. Otherwise, both will be initialized to zero which
	// indicates that there is no RSVP limit
	m.RSVPSpaces = m.RSVPLimit

	m.Meta.Flags.SetVisible()

	return http.StatusOK, nil
}

func (m *EventType) FetchProfileSummaries(siteId int64) (int, error) {

	profile, status, err := GetProfileSummary(siteId, m.Meta.CreatedById)
	if err != nil {
		return status, err
	}
	m.Meta.CreatedBy = profile

	if m.Meta.EditedByNullable.Valid {
		profile, status, err :=
			GetProfileSummary(siteId, m.Meta.EditedByNullable.Int64)
		if err != nil {
			return status, err
		}
		m.Meta.EditedBy = profile
	}

	return http.StatusOK, nil
}

func (m *EventSummaryType) FetchProfileSummaries(siteId int64) (int, error) {

	profile, status, err := GetProfileSummary(siteId, m.Meta.CreatedById)
	if err != nil {
		glog.Errorf(
			"GetProfileSummary(%d, %d) %+v",
			siteId,
			m.Meta.CreatedById,
			err,
		)
		return status, err
	}
	m.Meta.CreatedBy = profile

	switch m.LastComment.(type) {
	case LastComment:
		lastComment := m.LastComment.(LastComment)

		profile, status, err =
			GetProfileSummary(siteId, lastComment.CreatedById)
		if err != nil {
			glog.Errorf("%+v", lastComment)
			glog.Errorf(
				"GetProfileSummary(%d, %d) %+v",
				siteId,
				lastComment.CreatedById,
				err,
			)
			return status, err
		}

		lastComment.CreatedBy = profile
		m.LastComment = lastComment
	}

	return http.StatusOK, nil
}

func IsAttending(profileId int64, eventId int64) (bool, error) {

	if profileId == 0 || eventId == 0 {
		return false, nil
	}

	var attendeeIds []int64

	key := fmt.Sprintf(mcEventKeys[c.CacheProfileIds], eventId)
	attendeeIds, ok := c.CacheGetInt64Slice(key)

	if !ok {
		db, err := h.GetConnection()
		if err != nil {
			return false, err
		}

		rows, err := db.Query(`
SELECT profile_id
  FROM attendees
 WHERE event_id = $1
   AND state_id = 1`,
			eventId,
		)
		if err != nil {
			return false, err
		}

		for rows.Next() {
			var attendeeId int64
			err = rows.Scan(&attendeeId)
			attendeeIds = append(attendeeIds, attendeeId)
		}

		c.CacheSetInt64Slice(key, attendeeIds, mcTtl)
	}

	for _, Id := range attendeeIds {
		if profileId == Id {
			return true, nil
		}
	}

	return false, nil
}

func (m *EventType) GetAttending(profileId int64) (int, error) {
	if profileId == 0 {
		return http.StatusOK, nil
	}

	attending, err := IsAttending(profileId, m.Id)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	m.Meta.Flags.Attending = attending
	return http.StatusOK, nil
}

func (m *EventSummaryType) GetAttending(profileId int64) (int, error) {
	if profileId == 0 {
		return http.StatusOK, nil
	}

	attending, err := IsAttending(profileId, m.Id)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	m.Meta.Flags.Attending = attending
	return http.StatusOK, nil
}

func (m *EventType) Insert(siteId int64, profileId int64) (int, error) {

	status, err := m.Validate(siteId, profileId, false)
	if err != nil {
		return status, err
	}

	var (
		when  string
		where string
	)

	if m.WhenNullable.Valid {
		when = m.WhenNullable.Time.String()
	}
	if m.WhereNullable.Valid {
		where = m.WhereNullable.String
	}

	dupeKey := "dupe_" + h.Md5sum(
		strconv.FormatInt(m.MicrocosmId, 10)+
			m.Title+
			when+
			where+
			fmt.Sprintf("%b", m.Lat)+
			fmt.Sprintf("%b", m.Lon)+
			fmt.Sprintf("%b", m.North)+
			fmt.Sprintf("%b", m.East)+
			fmt.Sprintf("%b", m.South)+
			fmt.Sprintf("%b", m.West)+
			m.Status+
			fmt.Sprintf("%d", m.RSVPLimit)+
			strconv.FormatInt(m.Meta.CreatedById, 10),
	)

	v, ok := c.CacheGetInt64(dupeKey)
	if ok {
		m.Id = v
		return http.StatusOK, nil
	}

	tx, err := h.GetTransaction()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	var insertId int64

	err = tx.QueryRow(`
INSERT INTO events (
    microcosm_id, title, created, created_by, "when",
    duration, "where", lat, lon, bounds_north,
    bounds_east, bounds_south, bounds_west, status, rsvp_limit,
    rsvp_spaces
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15,
    $16
) RETURNING event_id`,
		m.MicrocosmId,
		m.Title,
		m.Meta.Created,
		m.Meta.CreatedById,
		m.WhenNullable,
		m.Duration,
		m.WhereNullable,
		m.Lat,
		m.Lon,
		m.North,
		m.East,
		m.South,
		m.West,
		m.Status,
		m.RSVPLimit,
		m.RSVPSpaces,
	).Scan(
		&insertId,
	)
	if err != nil {
		glog.Errorf(`Could not create event: %+v`, err)
		return http.StatusInternalServerError,
			fmt.Errorf("Error inserting data and returning ID: %+v", err)
	}
	m.Id = insertId

	err = IncrementMicrocosmItemCount(tx, m.MicrocosmId)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	err = tx.Commit()
	if err != nil {
		glog.Errorf(`Could not commit event transaction: %+v`, err)
		return http.StatusInternalServerError,
			fmt.Errorf("Transaction failed: %v", err.Error())
	}

	// 5 minute dupe check
	c.CacheSetInt64(dupeKey, m.Id, 60*5)

	PurgeCache(h.ItemTypes[h.ItemTypeEvent], m.Id)
	PurgeCache(h.ItemTypes[h.ItemTypeMicrocosm], m.MicrocosmId)

	return http.StatusOK, nil
}

func (m *EventType) Update(siteId int64, profileId int64) (int, error) {

	status, err := m.Validate(siteId, profileId, true)
	if err != nil {
		return status, err
	}

	// Update resource
	tx, err := h.GetTransaction()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
UPDATE events 
   SET microcosm_id = $2
      ,title = $3
      ,edited = $4
      ,edited_by = $5
      ,edit_reason = $6
      ,"when" = $7
      ,duration = $8
      ,"where" = $9
      ,lat = $10
      ,lon = $11
      ,bounds_north = $12
      ,bounds_east = $13
      ,bounds_south = $14
      ,bounds_west = $15
      ,status = $16
      ,rsvp_limit = $17
 WHERE event_id = $1`,

		m.Id,
		m.MicrocosmId,
		m.Title,
		m.Meta.EditedNullable,
		m.Meta.EditedByNullable,

		m.Meta.EditReason,
		m.WhenNullable,
		m.Duration,
		m.WhereNullable,
		m.Lat,

		m.Lon,
		m.North,
		m.East,
		m.South,
		m.West,

		m.Status,
		m.RSVPLimit,
	)
	if err != nil {
		tx.Rollback()
		return http.StatusInternalServerError, errors.New(
			fmt.Sprintf("Update of event failed: %v", err.Error()),
		)
	}

	//Recalculate attendees
	status, err = m.UpdateAttendees(tx)
	if err != nil {
		return status, err
	}

	err = tx.Commit()
	if err != nil {
		return http.StatusInternalServerError, errors.New(
			fmt.Sprintf("Transaction failed: %v", err.Error()),
		)
	}

	PurgeCache(h.ItemTypes[h.ItemTypeEvent], m.Id)
	PurgeCache(h.ItemTypes[h.ItemTypeMicrocosm], m.MicrocosmId)

	return http.StatusOK, nil
}

func (m *EventType) UpdateAttendees(tx *sql.Tx) (int, error) {

	_, err := tx.Exec(`
UPDATE events
   SET rsvp_attending = att.attending
      ,rsvp_spaces = CASE rsvp_limit WHEN 0 THEN 0 ELSE (rsvp_limit - att.attending) END
  FROM (
        SELECT e.event_id
              ,a.state_id
              ,COUNT(a.*) as attending
          FROM events e
               LEFT OUTER JOIN (
                     SELECT *
                       FROM attendees
                      WHERE state_id = 1
               ) a ON e.event_id = a.event_id
         WHERE e.event_id = $1
         GROUP BY e.event_id
                 ,a.state_id
       ) AS att
 WHERE events.event_id = att.event_id`,
		m.Id,
	)
	if err != nil {
		tx.Rollback()
		return http.StatusInternalServerError, errors.New(
			fmt.Sprintf("Update of event attendees failed: %v", err.Error()),
		)
	}

	return http.StatusOK, nil
}

func (m *EventType) Patch(ac AuthContext, patches []h.PatchType) (int, error) {

	// Update resource
	tx, err := h.GetTransaction()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	for _, patch := range patches {

		m.Meta.EditedNullable = pq.NullTime{Time: time.Now(), Valid: true}
		m.Meta.EditedByNullable = sql.NullInt64{Int64: ac.ProfileId, Valid: true}

		var column string
		patch.ScanRawValue()
		switch patch.Path {
		case "/meta/flags/sticky":
			column = "is_sticky"
			m.Meta.Flags.Sticky = patch.Bool.Bool
			m.Meta.EditReason =
				fmt.Sprintf("Set sticky to %t", m.Meta.Flags.Sticky)
		case "/meta/flags/open":
			column = "is_open"
			m.Meta.Flags.Open = patch.Bool.Bool
			m.Meta.EditReason =
				fmt.Sprintf("Set open to %t", m.Meta.Flags.Open)
		case "/meta/flags/deleted":
			column = "is_deleted"
			m.Meta.Flags.Deleted = patch.Bool.Bool
			m.Meta.EditReason =
				fmt.Sprintf("Set delete to %t", m.Meta.Flags.Deleted)
		case "/meta/flags/moderated":
			column = "is_moderated"
			m.Meta.Flags.Moderated = patch.Bool.Bool
			m.Meta.EditReason =
				fmt.Sprintf("Set moderated to %t", m.Meta.Flags.Moderated)
		default:
			return http.StatusBadRequest,
				errors.New("Unsupported path in patch replace operation")
		}

		m.Meta.Flags.SetVisible()
		_, err = tx.Exec(`
UPDATE events
   SET `+column+` = $2
      ,is_visible = $3
      ,edited = $4
      ,edited_by = $5
      ,edit_reason = $6
 WHERE event_id = $1`,
			m.Id,
			patch.Bool.Bool,
			m.Meta.Flags.Visible,
			m.Meta.EditedNullable,
			m.Meta.EditedByNullable,
			m.Meta.EditReason,
		)
		if err != nil {
			return http.StatusInternalServerError, errors.New(
				fmt.Sprintf("Update failed: %v", err.Error()),
			)
		}
	}

	err = tx.Commit()
	if err != nil {
		return http.StatusInternalServerError, errors.New(
			fmt.Sprintf("Transaction failed: %v", err.Error()),
		)
	}

	PurgeCache(h.ItemTypes[h.ItemTypeEvent], m.Id)
	PurgeCache(h.ItemTypes[h.ItemTypeMicrocosm], m.MicrocosmId)

	return http.StatusOK, nil
}

func (m *EventType) Delete() (int, error) {

	// Connect to DB
	tx, err := h.GetTransaction()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
UPDATE events
   SET is_deleted = true
      ,is_visible = false
 WHERE event_id = $1`,
		m.Id,
	)
	if err != nil {
		return http.StatusInternalServerError, errors.New(
			fmt.Sprintf("Delete failed: %v", err.Error()),
		)
	}

	err = DecrementMicrocosmItemCount(tx, m.MicrocosmId)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	err = tx.Commit()
	if err != nil {
		return http.StatusInternalServerError, errors.New(
			fmt.Sprintf("Transaction failed: %v", err.Error()),
		)
	}

	PurgeCache(h.ItemTypes[h.ItemTypeEvent], m.Id)
	PurgeCache(h.ItemTypes[h.ItemTypeMicrocosm], m.MicrocosmId)

	return http.StatusOK, nil
}

func GetEvent(siteId int64, id int64, profileId int64) (EventType, int, error) {

	if id == 0 {
		return EventType{}, http.StatusNotFound, errors.New("Event not found")
	}

	// Get from cache if it's available
	mcKey := fmt.Sprintf(mcEventKeys[c.CacheDetail], id)
	if val, ok := c.CacheGet(mcKey, EventType{}); ok {

		m := val.(EventType)

		// TODO(buro9) 2014-05-05: We are not verifying that the cached
		// event belongs to this siteId

		status, err := m.FetchProfileSummaries(siteId)
		if err != nil {
			glog.Errorf("m.FetchProfileSummaries(%d) %+v", siteId, err)
			return EventType{}, status, err
		}

		status, err = m.GetAttending(profileId)
		if err != nil {
			glog.Errorf("m.GetAttending(%d) %+v", profileId, err)
			return EventType{}, status, err
		}

		return m, 0, nil
	}

	// Open db connection and retrieve resource
	db, err := h.GetConnection()
	if err != nil {
		glog.Errorf("h.GetConnection() %+v", err)
		return EventType{}, http.StatusInternalServerError, err
	}

	var m EventType
	err = db.QueryRow(`
SELECT e.event_id
      ,e.microcosm_id
      ,e.title
      ,e.created
      ,e.created_by

      ,e.edited
      ,e.edited_by
      ,e.edit_reason
      ,e.is_sticky
      ,e.is_open

      ,e.is_visible
      ,e.is_moderated
      ,e.is_deleted
      ,e."when"
      ,e.duration

      ,e."where"
      ,e.lat
      ,e.lon
      ,e.bounds_north
      ,e.bounds_east

      ,e.bounds_south
      ,e.bounds_west
      ,e.status
      ,e.rsvp_limit
      ,e.rsvp_attending

      ,e.rsvp_spaces
  FROM events e
       JOIN flags f ON f.site_id = $2
                   AND f.item_type_id = 9
                   AND f.item_id = e.event_id
 WHERE e.event_id = $1
   AND f.microcosm_is_deleted IS NOT TRUE
   AND f.microcosm_is_moderated IS NOT TRUE
   AND f.parent_is_deleted IS NOT TRUE
   AND f.parent_is_moderated IS NOT TRUE
   AND f.item_is_deleted IS NOT TRUE
   AND f.item_is_moderated IS NOT TRUE`,
		id,
		siteId,
	).Scan(
		&m.Id,
		&m.MicrocosmId,
		&m.Title,
		&m.Meta.Created,
		&m.Meta.CreatedById,

		&m.Meta.EditedNullable,
		&m.Meta.EditedByNullable,
		&m.Meta.EditReasonNullable,
		&m.Meta.Flags.Sticky,
		&m.Meta.Flags.Open,

		&m.Meta.Flags.Visible,
		&m.Meta.Flags.Moderated,
		&m.Meta.Flags.Deleted,
		&m.WhenNullable,
		&m.Duration,

		&m.WhereNullable,
		&m.Lat,
		&m.Lon,
		&m.North,
		&m.East,

		&m.South,
		&m.West,
		&m.Status,
		&m.RSVPLimit,
		&m.RSVPAttending,

		&m.RSVPSpaces,
	)
	if err == sql.ErrNoRows {
		return EventType{}, http.StatusNotFound,
			errors.New("Event not found")
	} else if err != nil {
		glog.Errorf("db.QueryRow(%d) %+v", id, err)
		return EventType{}, http.StatusInternalServerError,
			errors.New("Database query failed")
	}

	if m.Meta.EditReasonNullable.Valid {
		m.Meta.EditReason = m.Meta.EditReasonNullable.String
	}
	if m.Meta.EditedNullable.Valid {
		m.Meta.Edited = m.Meta.EditedNullable.Time.Format(time.RFC3339Nano)
	}
	if m.WhenNullable.Valid {
		m.When = m.WhenNullable.Time.Format(time.RFC3339Nano)
	}
	if m.WhereNullable.Valid {
		m.Where = m.WhereNullable.String
	}

	m.Meta.Links =
		[]h.LinkType{
			h.GetLink("self", "", h.ItemTypeEvent, m.Id),
			h.GetLink(
				"microcosm",
				GetMicrocosmTitle(m.MicrocosmId),
				h.ItemTypeMicrocosm,
				m.MicrocosmId,
			),
		}

	// Add meta links
	m.Meta.Links =
		[]h.LinkType{
			h.GetLink("self", "", h.ItemTypeEvent, m.Id),
			h.GetLink(
				"microcosm",
				GetMicrocosmTitle(m.MicrocosmId),
				h.ItemTypeMicrocosm,
				m.MicrocosmId,
			),
		}

	// Update cache
	c.CacheSet(mcKey, m, mcTtl)

	status, err := m.FetchProfileSummaries(siteId)
	if err != nil {
		glog.Errorf("m.FetchProfileSummaries(%d) %+v", siteId, err)
		return EventType{}, status, err
	}
	status, err = m.GetAttending(profileId)
	if err != nil {
		glog.Errorf("m.GetAttending(%d) %+v", profileId, err)
		return EventType{}, status, err
	}

	return m, http.StatusOK, nil
}

func GetEventSummary(
	siteId int64,
	id int64,
	profileId int64,
) (
	EventSummaryType,
	int,
	error,
) {

	if id == 0 {
		return EventSummaryType{}, http.StatusNotFound,
			errors.New("Event not found")
	}

	// Get from cache if it's available
	mcKey := fmt.Sprintf(mcEventKeys[c.CacheSummary], id)
	if val, ok := c.CacheGet(mcKey, EventSummaryType{}); ok {

		m := val.(EventSummaryType)

		status, err := m.FetchProfileSummaries(siteId)
		if err != nil {
			glog.Errorf("m.FetchProfileSummaries(%d) %+v", siteId, err)
			return EventSummaryType{}, status, err
		}

		status, err = m.GetAttending(profileId)
		if err != nil {
			glog.Errorf("m.GetAttending(%d) %+v", profileId, err)
			return EventSummaryType{}, status, err
		}

		return m, http.StatusOK, nil
	}

	// Open db connection and retrieve resource
	db, err := h.GetConnection()
	if err != nil {
		glog.Errorf("h.GetConnection() %+v", err)
		return EventSummaryType{}, http.StatusInternalServerError, err
	}

	var m EventSummaryType
	err = db.QueryRow(`
SELECT event_id
      ,microcosm_id
      ,title
      ,created
      ,created_by

      ,is_sticky
      ,is_open
      ,is_visible
      ,is_moderated
      ,is_deleted

      ,"when"
      ,duration
      ,"where"
      ,lat
      ,lon

      ,bounds_north
      ,bounds_east
      ,bounds_south
      ,bounds_west
      ,status

      ,rsvp_limit
      ,rsvp_attending
      ,rsvp_spaces
      ,(SELECT COUNT(*) AS total_comments
          FROM flags
         WHERE parent_item_type_id = 9
           AND parent_item_id = $1
           AND item_is_deleted IS NOT TRUE
           AND item_is_moderated IS NOT TRUE) AS comment_count
      ,view_count
 FROM events
WHERE event_id = $1
  AND is_deleted(9, event_id) IS FALSE`,
		id,
	).Scan(
		&m.Id,
		&m.MicrocosmId,
		&m.Title,
		&m.Meta.Created,
		&m.Meta.CreatedById,

		&m.Meta.Flags.Sticky,
		&m.Meta.Flags.Open,
		&m.Meta.Flags.Visible,
		&m.Meta.Flags.Moderated,
		&m.Meta.Flags.Deleted,

		&m.WhenNullable,
		&m.Duration,
		&m.WhereNullable,
		&m.Lat,
		&m.Lon,

		&m.North,
		&m.East,
		&m.South,
		&m.West,
		&m.Status,

		&m.RSVPLimit,
		&m.RSVPAttending,
		&m.RSVPSpaces,
		&m.CommentCount,
		&m.ViewCount,
	)
	if err == sql.ErrNoRows {
		return EventSummaryType{}, http.StatusInternalServerError,
			errors.New(fmt.Sprintf("Event with ID %d not found", id))

	} else if err != nil {
		glog.Errorf("db.QueryRow(%d, %d) %+v", siteId, id, err)
		return EventSummaryType{}, http.StatusInternalServerError,
			errors.New("Database query failed")
	}

	if m.WhenNullable.Valid {
		m.When = m.WhenNullable.Time.Format(time.RFC3339Nano)
	}

	if m.WhereNullable.Valid {
		m.Where = m.WhereNullable.String
	}

	lastComment, status, err :=
		GetLastComment(h.ItemTypes[h.ItemTypeEvent], m.Id)
	if err != nil {
		return EventSummaryType{}, status, errors.New(
			fmt.Sprintf("Error fetching last comment: %v", err.Error()),
		)
	}

	if lastComment.Valid {
		m.LastComment = lastComment
	}

	// Add meta links
	m.Meta.Links =
		[]h.LinkType{
			h.GetLink("self", "", h.ItemTypeEvent, m.Id),
			h.GetLink(
				"microcosm",
				GetMicrocosmTitle(m.MicrocosmId),
				h.ItemTypeMicrocosm, m.MicrocosmId,
			),
		}

	// Update cache
	c.CacheSet(mcKey, m, mcTtl)

	status, err = m.FetchProfileSummaries(siteId)
	if err != nil {
		glog.Errorf("m.FetchProfileSummaries(%d) %+v", siteId, err)
		return EventSummaryType{}, status, err
	}

	status, err = m.GetAttending(profileId)
	if err != nil {
		glog.Errorf("m.GetAttending(%d) %+v", profileId, err)
		return EventSummaryType{}, status, err
	}

	return m, http.StatusOK, nil
}

func GetEvents(
	siteId int64,
	profileId int64,
	attending bool,
	limit int64,
	offset int64,
) (
	[]EventSummaryType,
	int64,
	int64,
	int,
	error,
) {

	// Retrieve resources
	db, err := h.GetConnection()
	if err != nil {
		return []EventSummaryType{}, 0, 0, http.StatusInternalServerError, err
	}

	var whereAttending string
	if attending {
		whereAttending = `
   AND is_attending(item_id, $3)`
	}

	rows, err := db.Query(`--GetEvents
WITH m AS (
    SELECT m.microcosm_id
      FROM microcosms m
      LEFT JOIN ignores i ON i.profile_id = $3
                         AND i.item_type_id = 2
                         AND i.item_id = m.microcosm_id
     WHERE i.profile_id IS NULL
       AND (get_effective_permissions(m.site_id, m.microcosm_id, 2, m.microcosm_id, $3)).can_read IS TRUE
)
SELECT COUNT(*) OVER() AS total
      ,f.item_id
	  ,f.is_attending(f.item_id, $3)
  FROM flags f
  LEFT JOIN ignores i ON i.profile_id = $3
                     AND i.item_type_id = f.item_type_id
                     AND i.item_id = f.item_id
 WHERE f.site_id = $1
   AND i.profile_id IS NULL
   AND f.item_type_id = $2
   AND f.microcosm_is_deleted IS NOT TRUE
   AND f.microcosm_is_moderated IS NOT TRUE
   AND f.parent_is_deleted IS NOT TRUE
   AND f.parent_is_moderated IS NOT TRUE
   AND f.item_is_deleted IS NOT TRUE
   AND f.item_is_moderated IS NOT TRUE`+whereAttending+`
   AND f.microcosm_id IN (SELECT * FROM m)
 ORDER BY f.item_is_sticky DESC
         ,f.last_modified DESC
 LIMIT $4
OFFSET $5`,
		siteId,
		h.ItemTypes[h.ItemTypeEvent],
		profileId,
		limit,
		offset,
	)
	if err != nil {
		return []EventSummaryType{}, 0, 0, http.StatusInternalServerError,
			errors.New(
				fmt.Sprintf("Database query failed: %v", err.Error()),
			)
	}
	defer rows.Close()

	var ems []EventSummaryType

	var total int64
	for rows.Next() {
		var (
			id          int64
			isAttending bool
		)
		err = rows.Scan(
			&total,
			&id,
			&isAttending,
		)
		if err != nil {
			return []EventSummaryType{}, 0, 0, http.StatusInternalServerError,
				errors.New(
					fmt.Sprintf("Row parsing error: %v", err.Error()),
				)
		}

		m, status, err := GetEventSummary(siteId, id, profileId)
		if err != nil {
			return []EventSummaryType{}, 0, 0, status, err
		}

		m.Meta.Flags.Attending = isAttending
		ems = append(ems, m)
	}
	err = rows.Err()
	if err != nil {
		return []EventSummaryType{}, 0, 0, http.StatusInternalServerError,
			errors.New(
				fmt.Sprintf("Error fetching rows: %v", err.Error()),
			)
	}
	rows.Close()

	pages := h.GetPageCount(total, limit)
	maxOffset := h.GetMaxOffset(total, limit)

	if offset > maxOffset {
		return []EventSummaryType{}, 0, 0, http.StatusBadRequest, errors.New(
			fmt.Sprintf(
				"not enough records, offset (%d) would return an empty page.",
				offset,
			),
		)
	}

	return ems, total, pages, http.StatusOK, nil
}

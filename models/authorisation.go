package models

import (
	"github.com/golang/glog"

	h "github.com/microcosm-cc/microcosm/helpers"
)

type AuthContext struct {
	SiteId      int64
	ProfileId   int64
	IsSiteOwner bool
	MicrocosmId int64
	ItemTypeId  int64
	ItemId      int64
}

type PermissionType struct {
	CanCreate     bool        `json:"create"`
	CanRead       bool        `json:"read"`
	CanUpdate     bool        `json:"update"`
	CanDelete     bool        `json:"delete"`
	CanCloseOwn   bool        `json:"closeOwn"`
	CanOpenOwn    bool        `json:"openOwn"`
	CanReadOthers bool        `json:"readOthers"`
	IsGuest       bool        `json:"guest"`
	IsBanned      bool        `json:"banned"`
	IsOwner       bool        `json:"owner"`
	IsModerator   bool        `json:"moderator"`
	IsSiteOwner   bool        `json:"siteOwner"`
	Context       AuthContext `json:"-"`
	Valid         bool        `json:"-"`
}

func MakeAuthorisationContext(
	c *Context,
	m int64,
	t int64,
	i int64,
) AuthContext {

	return AuthContext{
		SiteId:      c.Site.Id,
		ProfileId:   c.Auth.ProfileId,
		IsSiteOwner: c.Auth.IsSiteOwner,
		MicrocosmId: m,
		ItemTypeId:  t,
		ItemId:      i,
	}
}

func GetPermission(ac AuthContext) PermissionType {

	if ac.ProfileId == 0 && ac.ItemTypeId == h.ItemTypes[h.ItemTypeSite] {
		// Guests can read site description, we can save a query
		m := PermissionType{Context: ac, Valid: true}
		m.CanRead = true
		m.IsGuest = true
		return m
	}

	tx, err := h.GetTransaction()
	if err != nil {
		glog.Errorf("h.GetTransaction() %+v", err)
		return PermissionType{}
	}
	defer tx.Rollback()

	// This is in a transaction because even though it looks like a read the
	// get_effective_permissions function *may* perform an insert into the
	// role_members_cache table.
	//
	// If we don't put this in a transaction it is possible that we have a
	// race condition on the insert that will cause one of the queries (the
	// latter) to fail.
	m := PermissionType{Context: ac, Valid: true}
	err = tx.QueryRow(`
SELECT can_create
      ,can_read
      ,can_update
      ,can_delete
      ,can_close_own
      ,can_open_own
      ,can_read_others
      ,is_guest
      ,is_banned
      ,is_owner
      ,is_superuser AS is_moderator
      ,is_site_owner
  FROM get_effective_permissions($1,$2,$3,$4,$5)`,
		ac.SiteId,
		ac.MicrocosmId,
		ac.ItemTypeId,
		ac.ItemId,
		ac.ProfileId,
	).Scan(
		&m.CanCreate,
		&m.CanRead,
		&m.CanUpdate,
		&m.CanDelete,
		&m.CanCloseOwn,
		&m.CanOpenOwn,
		&m.CanReadOthers,
		&m.IsGuest,
		&m.IsBanned,
		&m.IsOwner,
		&m.IsModerator,
		&m.IsSiteOwner,
	)
	if err != nil {
		nerr := tx.Rollback()
		if nerr != nil {
			glog.Errorf("Cannot rollback: %+v", nerr)
		}

		glog.Errorf(
			"stmt.QueryRow(%d, %d, %d, %d, %d).Scan() %+v\n",
			ac.SiteId,
			ac.MicrocosmId,
			ac.ItemTypeId,
			ac.ItemId,
			ac.ProfileId,
			err,
		)

		return PermissionType{}
	}

	err = tx.Commit()
	if err != nil {

		glog.Errorf(
			"tx.Commit() after stmt.QueryRow(%d, %d, %d, %d, %d) %+v\n",
			ac.SiteId,
			ac.MicrocosmId,
			ac.ItemTypeId,
			ac.ItemId,
			ac.ProfileId,
			err,
		)

		return PermissionType{}
	}

	return m
}

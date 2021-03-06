package models

import (
	"github.com/golang/glog"

	c "github.com/microcosm-cc/microcosm/cache"
	h "github.com/microcosm-cc/microcosm/helpers"
)

// Finds huddles that no longer have participants and deletes them
func DeleteOrphanedHuddles() {

	tx, err := h.GetTransaction()
	if err != nil {
		glog.Error(err)
		return
	}
	defer tx.Rollback()

	// Identify orphaned huddles
	rows, err := tx.Query(
		`SELECT h.huddle_id
  FROM huddles h
       LEFT OUTER JOIN huddle_profiles hp ON h.huddle_id = hp.huddle_id
 GROUP BY h.huddle_id, hp.huddle_id
HAVING COUNT(hp.huddle_id) = 0`)
	if err != nil {
		glog.Error(err)
		return
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var huddleId int64
		err = rows.Scan(&huddleId)
		if err != nil {
			glog.Error(err)
			return
		}
		ids = append(ids, huddleId)
	}
	err = rows.Err()
	if err != nil {
		glog.Error(err)
		return
	}
	rows.Close()

	if len(ids) == 0 {
		return
	}

	revisionsStmt, err := tx.Prepare(
		`DELETE
  FROM revisions
 WHERE comment_id IN (
       SELECT comment_id
         FROM comments
        WHERE item_type_id = 5
          AND item_id = $1`)
	if err != nil {
		glog.Error(err)
		return
	}

	commentsStmt, err := tx.Prepare(
		`DELETE
  FROM comments
 WHERE item_type_id = 5
   AND item_id = $1`)
	if err != nil {
		glog.Error(err)
		return
	}

	huddleStmt, err := tx.Prepare(
		`DELETE
  FROM huddles
 WHERE huddle_id = $1`)
	if err != nil {
		glog.Error(err)
		return
	}

	for _, huddleId := range ids {
		// delete comment + revisions that belong to this huddle
		// May well be best to expand the above SQL rather than execute lots
		// of single delete commands.

		_, err = revisionsStmt.Exec(huddleId)
		if err != nil {
			glog.Error(err)
			return
		}

		_, err = commentsStmt.Exec(huddleId)
		if err != nil {
			glog.Error(err)
			return
		}

		_, err = huddleStmt.Exec(huddleId)
		if err != nil {
			glog.Error(err)
			return
		}

	}

	tx.Commit()
}

// Updates the site stats across all sites.
func UpdateAllSiteStats() {

	db, err := h.GetConnection()
	if err != nil {
		glog.Error(err)
		return
	}

	rows, err := db.Query(
		`SELECT site_id FROM sites WHERE is_deleted IS NOT TRUE`,
	)
	if err != nil {
		glog.Error(err)
		return
	}
	defer rows.Close()

	// For each site, fetch stats and purge cache.
	ids := []int64{}
	for rows.Next() {

		var siteId int64
		err = rows.Scan(&siteId)
		if err != nil {
			glog.Error(err)
			return
		}

		ids = append(ids, siteId)
	}
	err = rows.Err()
	if err != nil {
		glog.Error(err)
		return
	}
	rows.Close()

	for _, siteId := range ids {
		err = UpdateSiteStats(siteId)
		if err != nil {
			glog.Error(err)
			return
		}
	}
}

// Updates the metrics used by the internal dashboard by the admins. This
// includes counts of the number of items, changes in active sites, etc.
func UpdateMetricsCron() {
	UpdateMetrics()
}

// Updates the count of items for microcosms, which is used to order the
// microcosms
//
// This is pure housekeeping, the numbers are maintained through increments and
// decrements as stuff is added and deleted, but there are edge cases that may
// result in the numbers not being accurate (batch deletions, things being
// deleted via PATCH, etc).
//
// This function is designed to calculate the real numbers and only update rows
// where the numbers are not the real numbers.
func UpdateMicrocosmItemCounts() {

	// No transaction as we don't care for accuracy on these updates
	// Note: This function doesn't even return errors, we don't even care
	// if the occasional UPDATE fails. All this effects are the ordering of
	// Microcosms on a page... this is fairly non-critical
	db, err := h.GetConnection()
	if err != nil {
		glog.Error(err)
		return
	}

	// Update item and comment counts
	_, err = db.Exec(
		`UPDATE microcosms m
   SET comment_count = s.comment_count
      ,item_count = s.item_count
  FROM (
           SELECT microcosm_id
                 ,SUM(item_count) AS item_count
                 ,SUM(comment_count) AS comment_count
             FROM (
                      -- Calculate item counts
                      SELECT microcosm_id
                            ,COUNT(*) AS item_count
                            ,0 AS comment_count
                        FROM flags
                       WHERE item_type_id IN (6,7,9)
                         AND microcosm_is_deleted IS NOT TRUE
                         AND microcosm_is_moderated IS NOT TRUE
                         AND parent_is_deleted IS NOT TRUE
                         AND parent_is_moderated IS NOT TRUE
                         AND item_is_deleted IS NOT TRUE
                         AND item_is_moderated IS NOT TRUE
                       GROUP BY microcosm_id
                       UNION
                      -- Calculate comment counts
                      SELECT microcosm_id
                            ,0 AS item_count
                            ,COUNT(*) AS comment_count
                        FROM flags
                       WHERE item_type_id = 4
                         AND parent_item_type_id IN (6,7,9)
                         AND microcosm_is_deleted IS NOT TRUE
                         AND microcosm_is_moderated IS NOT TRUE
                         AND parent_is_deleted IS NOT TRUE
                         AND parent_is_moderated IS NOT TRUE
                         AND item_is_deleted IS NOT TRUE
                         AND item_is_moderated IS NOT TRUE
                       GROUP BY microcosm_id
                  ) counts
            GROUP BY microcosm_id
       ) s
 WHERE m.microcosm_id = s.microcosm_id
   AND (
           m.item_count <> s.item_count
        OR m.comment_count <> s.comment_count
       )`)
	if err != nil {
		glog.Error(err)
		return
	}
}

func UpdateProfileCounts() {

	db, err := h.GetConnection()
	if err != nil {
		glog.Error(err)
		return
	}

	rows, err := db.Query(
		`SELECT site_id FROM sites WHERE is_deleted IS NOT TRUE`,
	)
	if err != nil {
		glog.Error(err)
		return
	}
	defer rows.Close()

	// For each site, fetch stats and purge cache.
	ids := []int64{}
	for rows.Next() {

		var siteId int64
		err = rows.Scan(&siteId)
		if err != nil {
			glog.Error(err)
			return
		}

		ids = append(ids, siteId)
	}
	err = rows.Err()
	if err != nil {
		glog.Error(err)
		return
	}
	rows.Close()

	for _, siteId := range ids {
		_, err = UpdateCommentCountForAllProfiles(siteId)
		if err != nil {
			glog.Error(err)
			return
		}
	}
}

// UpdateViewsCounts reads from the views table and will SUM the number of views
// and update all of the associated conversations and events with the new view
// count.
func UpdateViewCounts() {

	// No transaction as we don't care for accuracy on these updates
	// Note: This function doesn't even return errors, we don't even care
	// if the occasional UPDATE fails.
	tx, err := h.GetTransaction()
	if err != nil {
		glog.Error(err)
		return
	}
	defer tx.Rollback()

	type View struct {
		ItemTypeId int64
		ItemId     int64
	}

	rows, err := tx.Query(`--UpdateViewCounts
SELECT item_type_id
      ,item_id
  FROM views
 GROUP BY item_type_id, item_id`)
	if err != nil {
		glog.Error(err)
		return
	}
	defer rows.Close()

	var (
		views               []View
		updateConversations bool
		updateEvents        bool
		updatePolls         bool
	)
	for rows.Next() {
		var view View
		err = rows.Scan(
			&view.ItemTypeId,
			&view.ItemId,
		)
		if err != nil {
			glog.Error(err)
			return
		}

		switch view.ItemTypeId {
		case h.ItemTypes[h.ItemTypeConversation]:
			updateConversations = true
		case h.ItemTypes[h.ItemTypeEvent]:
			updateEvents = true
		case h.ItemTypes[h.ItemTypePoll]:
			updatePolls = true
		}

		views = append(views, view)
	}
	err = rows.Err()
	if err != nil {
		glog.Error(err)
		return
	}
	rows.Close()

	if len(views) == 0 {
		// No views to update
		return
	}

	// Our updates are a series of updates in the database, we don't even
	// read the records as why intervene like that?

	// Update conversations
	if updateConversations {
		_, err = tx.Exec(`--UpdateViewCounts
UPDATE conversations c
   SET view_count = view_count + v.views
  FROM (
        SELECT item_id
              ,COUNT(*) AS views
          FROM views
         WHERE item_type_id = 6
         GROUP BY item_id
       ) AS v
 WHERE c.conversation_id = v.item_id`)
		if err != nil {
			glog.Error(err)
			return
		}
	}

	// Update events
	if updateEvents {
		_, err = tx.Exec(`--UpdateViewCounts
UPDATE events e
   SET view_count = view_count + v.views
  FROM (
        SELECT item_id
              ,COUNT(*) AS views
          FROM views
         WHERE item_type_id = 9
         GROUP BY item_id
       ) AS v
 WHERE e.event_id = v.item_id`)
		if err != nil {
			glog.Error(err)
			return
		}
	}

	// Update polls
	if updatePolls {
		_, err = tx.Exec(`--UpdateViewCounts
UPDATE polls p
   SET view_count = view_count + v.views
  FROM (
        SELECT item_id
              ,COUNT(*) AS views
          FROM views
         WHERE item_type_id = 7
         GROUP BY item_id
       ) AS v
 WHERE p.poll_id = v.item_id;`)
		if err != nil {
			glog.Error(err)
			return
		}
	}

	// Clear views, and the quickest way to do that is just truncate the table
	_, err = tx.Exec(`TRUNCATE TABLE views`)
	if err != nil {
		glog.Error(err)
		return
	}

	tx.Commit()

	for _, view := range views {
		PurgeCacheByScope(c.CacheItem, view.ItemTypeId, view.ItemId)
	}

	return
}

// Updates the site_stats with the current number of people online on a site
func UpdateWhosOnline() {
	db, err := h.GetConnection()
	if err != nil {
		glog.Error(err)
		return
	}

	// Update item and comment counts
	_, err = db.Exec(`--UpdateWhosOnline
UPDATE site_stats s
   SET online_profiles = online
  FROM (
           SELECT site_id
                 ,COUNT(*) AS online
             FROM profiles
            WHERE last_active > NOW() - interval '90 minute'
            GROUP BY site_id
       ) p
 WHERE p.site_id = s.site_id`)
	if err != nil {
		glog.Error(err)
		return
	}

	// Purge the stats cache
	rows, err := db.Query(
		`SELECT site_id FROM sites WHERE is_deleted IS NOT TRUE`,
	)
	if err != nil {
		glog.Error(err)
		return
	}
	defer rows.Close()

	// For each site, fetch stats and purge cache.
	ids := []int64{}
	for rows.Next() {

		var siteId int64
		err = rows.Scan(&siteId)
		if err != nil {
			glog.Error(err)
			return
		}

		ids = append(ids, siteId)
	}
	err = rows.Err()
	if err != nil {
		glog.Error(err)
		return
	}
	rows.Close()

	for _, siteId := range ids {
		go PurgeCacheByScope(c.CacheCounts, h.ItemTypes[h.ItemTypeSite], siteId)
	}
}

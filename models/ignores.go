package models

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/golang/glog"

	h "github.com/microcosm-cc/microcosm/helpers"
)

type IgnoredType struct {
	Ignored h.ArrayType    `json:"ignored"`
	Meta    h.CoreMetaType `json:"meta"`
}

type IgnoreType struct {
	ProfileId  int64       `json:"-"`
	ItemTypeId int64       `json:"-"`
	ItemType   string      `json:"itemType,omitempty"`
	ItemId     int64       `json:"itemId,omitempty"`
	Item       interface{} `json:"item,omitempty"`
}

func (m *IgnoreType) Validate() (int, error) {

	if m.ProfileId <= 0 {
		return http.StatusBadRequest,
			errors.New(
				fmt.Sprintf(
					"profileId ('%d') must be a positive integer.",
					m.ProfileId,
				),
			)
	}

	if _, inMap := h.ItemTypes[m.ItemType]; !inMap {
		return http.StatusBadRequest,
			errors.New("You must specify a valid item type")
	} else {
		m.ItemTypeId = h.ItemTypes[m.ItemType]
	}

	if m.ItemId <= 0 {
		return http.StatusBadRequest,
			errors.New("You must specify an Item ID this comment belongs to")
	}

	return http.StatusOK, nil
}

func (m *IgnoreType) Update() (int, error) {

	status, err := m.Validate()
	if err != nil {
		return status, err
	}

	tx, err := h.GetTransaction()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	// Lack of error checking, it can only fail if it has been inserted already
	// and our answer "OK" remains the same if it exists after this action.
	tx.Exec(`--Create Ignore
INSERT INTO ignores (
    profile_id, item_type_id, item_id
) VALUES (
    $1, $2, $3
)`,
		m.ProfileId,
		m.ItemTypeId,
		m.ItemId,
	)
	if err == nil {
		tx.Commit()
	}

	return http.StatusOK, nil
}

func (m *IgnoreType) Delete() (int, error) {
	status, err := m.Validate()
	if err != nil {
		return status, err
	}

	tx, err := h.GetTransaction()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	// Lack of error checking, it can only fail if it has been deleted already
	// and our answer "OK" remains the same if it exists after this action.
	tx.Exec(`--Delete Ignore
DELETE
  FROM ignores
 WHERE profile_id = $1
   AND item_type_id = $2
   AND item_id = $3`,
		m.ProfileId,
		m.ItemTypeId,
		m.ItemId,
	)
	if err == nil {
		tx.Commit()
	}

	return http.StatusOK, nil
}

func GetIgnored(
	siteId int64,
	profileId int64,
	limit int64,
	offset int64,
) (
	[]IgnoreType,
	int64,
	int64,
	int,
	error,
) {

	db, err := h.GetConnection()
	if err != nil {
		glog.Errorf("h.GetConnection() %+v", err)
		return []IgnoreType{}, 0, 0, http.StatusInternalServerError, err
	}

	// This query intentionally does not provide has_unread() status. This is
	// to pacify angry people ignoring things, then unignoring on updates and
	// subsequently getting in to fights.
	sqlQuery := `--Get Ignores
SELECT COUNT(*) OVER() AS total
      ,profile_id
      ,item_type_id
      ,item_id
  FROM (
           SELECT i.profile_id
                 ,i.item_type_id
                 ,i.item_id
                 ,m.title
             FROM ignores i
             JOIN microcosms m ON m.microcosm_id = i.item_id
            WHERE i.profile_id = $1
              AND i.item_type_id = 2
            UNION
           SELECT i.profile_id
                 ,i.item_type_id
                 ,i.item_id
                 ,p.profile_name AS title
             FROM ignores i
             JOIN profiles p ON p.profile_id = i.item_id
            WHERE i.profile_id = $1
              AND i.item_type_id = 3
            UNION
           SELECT i.profile_id
                 ,i.item_type_id
                 ,i.item_id
                 ,si.title_text AS title
             FROM ignores i
             JOIN search_index si ON si.item_type_id = i.item_type_id
                                 AND si.item_id = i.item_id
            WHERE i.profile_id = $1
              AND i.item_type_id NOT IN (2,3)
       ) a
 ORDER BY item_type_id ASC
         ,title ASC
 LIMIT $2
OFFSET $3`

	rows, err := db.Query(sqlQuery, profileId, limit, offset)
	if err != nil {
		glog.Errorf(
			"db.Query(%d, %d, %d) %+v",
			profileId,
			limit,
			offset,
			err,
		)
		return []IgnoreType{}, 0, 0, http.StatusInternalServerError,
			errors.New("Database query failed")
	}
	defer rows.Close()

	var total int64
	ems := []IgnoreType{}
	for rows.Next() {
		m := IgnoreType{}
		err = rows.Scan(
			&total,
			&m.ProfileId,
			&m.ItemTypeId,
			&m.ItemId,
		)
		if err != nil {
			glog.Errorf("rows.Scan() %+v", err)
			return []IgnoreType{}, 0, 0, http.StatusInternalServerError,
				errors.New("Row parsing error")
		}

		itemType, err := h.GetItemTypeFromInt(m.ItemTypeId)
		if err != nil {
			glog.Errorf("h.GetItemTypeFromInt(%d) %+v", m.ItemTypeId, err)
			return []IgnoreType{}, 0, 0, http.StatusInternalServerError, err
		}
		m.ItemType = itemType

		ems = append(ems, m)
	}
	err = rows.Err()
	if err != nil {
		glog.Errorf("rows.Err() %+v", err)
		return []IgnoreType{}, 0, 0, http.StatusInternalServerError,
			errors.New("Error fetching rows")
	}
	rows.Close()

	pages := h.GetPageCount(total, limit)
	maxOffset := h.GetMaxOffset(total, limit)

	if offset > maxOffset {
		glog.Infoln("offset > maxOffset")
		return []IgnoreType{}, 0, 0, http.StatusBadRequest,
			errors.New(
				fmt.Sprintf("not enough records, "+
					"offset (%d) would return an empty page.", offset),
			)
	}

	// Get the first round of summaries
	var wg1 sync.WaitGroup
	chan1 := make(chan SummaryContainerRequest)
	defer close(chan1)

	seq := 0
	for i := 0; i < len(ems); i++ {
		go HandleSummaryContainerRequest(
			siteId,
			ems[i].ItemTypeId,
			ems[i].ItemId,
			ems[i].ProfileId,
			seq,
			chan1,
		)
		wg1.Add(1)
		seq++
	}

	resps := []SummaryContainerRequest{}
	for i := 0; i < seq; i++ {
		resp := <-chan1
		wg1.Done()

		resps = append(resps, resp)
	}
	wg1.Wait()

	for _, resp := range resps {
		if resp.Err != nil {
			return []IgnoreType{}, 0, 0, resp.Status, resp.Err
		}
	}

	sort.Sort(SummaryContainerRequestsBySeq(resps))

	seq = 0
	for i := 0; i < len(ems); i++ {
		ems[i].Item = resps[seq].Item.Summary
		seq++
	}

	return ems, total, pages, http.StatusOK, nil
}

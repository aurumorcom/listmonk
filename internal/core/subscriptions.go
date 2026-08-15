package core

import (
	"net/http"
	"time"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

// GetSubscriptions retrieves the subscriptions for a subscriber.
func (c *Core) GetSubscriptions(subID int, subUUID string, allLists bool) ([]models.Subscription, error) {
	var out []models.Subscription
	err := c.q.GetSubscriptions.Select(&out, subID, subUUID, allLists)
	if err != nil {
		c.log.Printf("error getting subscriptions: %v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "{globals.terms.subscribers}", "error", err.Error()))
	}

	return out, err
}

// AddSubscriptions adds list subscriptions to subscribers.
func (c *Core) AddSubscriptions(subIDs, listIDs []int, status string) error {
	if _, err := c.q.AddSubscribersToLists.Exec(pq.Array(subIDs), pq.Array(listIDs), status); err != nil {
		c.log.Printf("error adding subscriptions: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.subscribers}", "error", err.Error()))
	}

	if status == models.SubscriptionStatusConfirmed || status == "subscribed" {
		_ = c.EnrollSubscribersByList(subIDs, listIDs)
	}

	return nil
}

// AddSubscriptionsByQuery adds list subscriptions to subscribers by a given arbitrary query expression.
// sourceListIDs is the list of list IDs to filter the subscriber query with.
func (c *Core) AddSubscriptionsByQuery(searchStr, queryExp string, sourceListIDs, targetListIDs []int, status string, subStatus string) error {
	if sourceListIDs == nil {
		sourceListIDs = []int{}
	}

	err := c.q.ExecSubQueryTpl(searchStr, queryExp, c.q.AddSubscribersToListsByQuery, sourceListIDs, c.db, subStatus, pq.Array(targetListIDs), status)
	if err != nil {
		c.log.Printf("error adding subscriptions by query: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.subscribers}", "error", pqErrMsg(err)))
	}

	if status == models.SubscriptionStatusConfirmed || status == "subscribed" {
		// Auto-enroll all subscribers in target lists into active sequences
		for _, lID := range targetListIDs {
			_, _ = c.db.Exec(`INSERT INTO sequence_contacts (sequence_id, subscriber_id, status, current_step, next_send_at)
				SELECT DISTINCT sl.sequence_id, subl.subscriber_id, 'scheduled', 1, NOW()
				FROM sequence_lists sl
				JOIN subscriber_lists subl ON subl.list_id = sl.list_id AND subl.status = 'subscribed'
				JOIN sequences seq ON seq.id = sl.sequence_id AND seq.status = 'active'
				JOIN subscribers s ON s.id = subl.subscriber_id AND s.status = 'enabled'
				WHERE sl.list_id = $1
				ON CONFLICT (sequence_id, subscriber_id) DO NOTHING`, lID)
		}
	}

	return nil
}

// DeleteSubscriptions delete list subscriptions from subscribers.
func (c *Core) DeleteSubscriptions(subIDs, listIDs []int) error {
	if _, err := c.q.DeleteSubscriptions.Exec(pq.Array(subIDs), pq.Array(listIDs)); err != nil {
		c.log.Printf("error deleting subscriptions: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.subscribers}", "error", err.Error()))

	}

	_ = c.OptOutSubscribersByList(subIDs, listIDs)

	return nil
}

// DeleteSubscriptionsByQuery deletes list subscriptions from subscribers by a given arbitrary query expression.
// sourceListIDs is the list of list IDs to filter the subscriber query with.
func (c *Core) DeleteSubscriptionsByQuery(searchStr, queryExp string, sourceListIDs, targetListIDs []int, subStatus string) error {
	if sourceListIDs == nil {
		sourceListIDs = []int{}
	}

	err := c.q.ExecSubQueryTpl(searchStr, queryExp, c.q.DeleteSubscriptionsByQuery, sourceListIDs, c.db, subStatus, pq.Array(targetListIDs))
	if err != nil {
		c.log.Printf("error deleting subscriptions by query: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.subscribers}", "error", pqErrMsg(err)))
	}

	for _, lID := range targetListIDs {
		_, _ = c.db.Exec(`UPDATE sequence_contacts sc
			SET status = 'opted_out'
			WHERE sc.status IN ('scheduled', 'in_progress')
			  AND sc.sequence_id IN (SELECT sl.sequence_id FROM sequence_lists sl WHERE sl.list_id = $1)
			  AND NOT EXISTS (
			      SELECT 1 FROM sequence_lists sl2
			      JOIN subscriber_lists subl ON subl.list_id = sl2.list_id AND subl.status = 'subscribed'
			      WHERE sl2.sequence_id = sc.sequence_id AND subl.subscriber_id = sc.subscriber_id
			  )`, lID)
	}

	return nil
}

// UnsubscribeLists sets list subscriptions to 'unsubscribed'.
func (c *Core) UnsubscribeLists(subIDs, listIDs []int, listUUIDs []string) error {
	if _, err := c.q.UnsubscribeSubscribersFromLists.Exec(pq.Array(subIDs), pq.Array(listIDs), pq.StringArray(listUUIDs)); err != nil {
		c.log.Printf("error unsubscribing from lists: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.subscribers}", "error", err.Error()))
	}

	_ = c.OptOutSubscribersByList(subIDs, listIDs)

	return nil
}

// UnsubscribeListsByQuery sets list subscriptions to 'unsubscribed' by a given arbitrary query expression.
// sourceListIDs is the list of list IDs to filter the subscriber query with.
func (c *Core) UnsubscribeListsByQuery(searchStr, queryExp string, sourceListIDs, targetListIDs []int, subStatus string) error {
	if sourceListIDs == nil {
		sourceListIDs = []int{}
	}

	err := c.q.ExecSubQueryTpl(searchStr, queryExp, c.q.UnsubscribeSubscribersFromListsByQuery, sourceListIDs, c.db, subStatus, pq.Array(targetListIDs))
	if err != nil {
		c.log.Printf("error unsubscribing from lists by query: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorUpdating", "name", "{globals.terms.subscribers}", "error", pqErrMsg(err)))
	}

	return nil
}

// DeleteUnconfirmedSubscriptions sets list subscriptions to 'unsubscribed' by a given arbitrary query expression.
// sourceListIDs is the list of list IDs to filter the subscriber query with.
func (c *Core) DeleteUnconfirmedSubscriptions(beforeDate time.Time) (int, error) {
	res, err := c.q.DeleteUnconfirmedSubscriptions.Exec(beforeDate)
	if err != nil {
		c.log.Printf("error deleting unconfirmed subscribers: %v", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorDeleting", "name", "{globals.terms.subscribers}", "error", pqErrMsg(err)))
	}

	n, _ := res.RowsAffected()
	return int(n), nil
}

package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Object actions drive the communication lifecycle. Publishing an announcement stamps its
// publish date and fans it out into one pending notification per active delivery channel
// (rebuilt idempotently, so re-publishing refreshes the queue); an announcement can then be
// archived. A queued notification can be cancelled, and a failed one re-queued for another
// attempt with its retry date brought forward to today. A direct message is sent from the
// draft state, stamping its send date. All mutations run with access bypass and re-check
// their state transitions.

func init() {
	orm.RegisterObjectAction(announcementModel, "announcement_publish", actAnnouncementPublish)
	orm.RegisterObjectAction(announcementModel, "announcement_archive", actAnnouncementArchive)
	orm.RegisterObjectAction(notificationModel, "notification_cancel", actNotificationCancel)
	orm.RegisterObjectAction(notificationModel, "notification_retry", actNotificationRetry)
	orm.RegisterObjectAction(messageModel, "message_send", actMessageSend)
}

func today() string { return time.Now().Format(dateLayout) }

// recipientForAudience maps an announcement audience onto a notification recipient type.
func recipientForAudience(aud string) string {
	switch aud {
	case string(AudStudents), string(AudClass), string(AudSection):
		return string(RecStudent)
	case string(AudGuardians):
		return string(RecGuardian)
	case string(AudStaff):
		return string(RecStaff)
	default:
		return string(RecAll)
	}
}

func actAnnouncementPublish(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to publish an announcement")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, announcementModel, id)
	if err != nil {
		return "", err
	}
	st := strOr(rec["state"])
	if st != string(AnnDraft) && st != string(AnnPublished) {
		return "", fmt.Errorf("only a draft or published announcement can be (re)published (current status %q)", label(st))
	}
	channels, err := orm.Search(bypass, channelModel, [][]interface{}{{"active", "=", true}})
	if err != nil {
		return "", err
	}
	if err := orm.UpdateRecordByID(bypass, announcementModel, id, map[string]interface{}{
		"state":        string(AnnPublished),
		"publish_date": today(),
	}); err != nil {
		return "", err
	}
	// Rebuild the delivery queue idempotently: one pending notification per active channel.
	if _, err := orm.UnlinkWhere(bypass, notificationModel, [][]interface{}{{"announcement_id", "=", id}}); err != nil {
		return "", err
	}
	recipient := recipientForAudience(strOr(rec["audience"]))
	body := strOr(rec["body"])
	if body == "" {
		body = strOr(rec["summary"])
	}
	created := 0
	for _, ch := range channels {
		if _, err := orm.Create(bypass, orm.RegistryModel(notificationModel), map[string]interface{}{
			"announcement_id": id,
			"channel_id":      intOr(ch["id"]),
			"recipient_type":  recipient,
			"subject":         strOr(rec["title"]),
			"body":            body,
			"state":           string(NotifyPending),
			"attempts":        0,
			"max_attempts":    defaultMaxAttempts,
			"scheduled_at":    today(),
			"company_id":      intOr(rec["company_id"]),
		}); err != nil {
			return "", err
		}
		created++
	}
	if created == 0 {
		return "Announcement published (no active channels to queue)", nil
	}
	return fmt.Sprintf("Announcement published; queued %d notification(s)", created), nil
}

func actAnnouncementArchive(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to archive an announcement")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, announcementModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(AnnPublished) {
		return "", fmt.Errorf("only a published announcement can be archived (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, announcementModel, id, map[string]interface{}{"state": string(AnnArchived)}); err != nil {
		return "", err
	}
	return "Announcement archived", nil
}

func actNotificationCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to cancel a notification")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, notificationModel, id)
	if err != nil {
		return "", err
	}
	st := strOr(rec["state"])
	if st != string(NotifyPending) && st != string(NotifyFailed) {
		return "", fmt.Errorf("only a pending or failed notification can be cancelled (current status %q)", label(st))
	}
	if err := orm.UpdateRecordByID(bypass, notificationModel, id, map[string]interface{}{"state": string(NotifyCancelled)}); err != nil {
		return "", err
	}
	return "Notification cancelled", nil
}

// actNotificationRetry re-queues a failed delivery for another attempt, bringing its retry
// date forward so the next dispatch picks it up, provided it still has attempts left.
func actNotificationRetry(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to retry a notification")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, notificationModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(NotifyFailed) {
		return "", fmt.Errorf("only a failed notification can be retried (current status %q)", label(strOr(rec["state"])))
	}
	attempts := intOr(rec["attempts"])
	max := intOr(rec["max_attempts"])
	if !shouldAttempt(string(NotifyFailed), attempts, max) {
		return "", fmt.Errorf("no attempts left (reached the maximum of %d)", max)
	}
	if err := orm.UpdateRecordByID(bypass, notificationModel, id, map[string]interface{}{
		"state":         string(NotifyPending),
		"next_retry_at": today(),
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Notification re-queued (%d/%d attempts used)", attempts, max), nil
}

func actMessageSend(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to send a message")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, messageModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(MsgDraft) {
		return "", fmt.Errorf("only a draft message can be sent (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, messageModel, id, map[string]interface{}{
		"state":     string(MsgSent),
		"sent_date": today(),
	}); err != nil {
		return "", err
	}
	return "Message sent", nil
}

// ---------------------------------------------------------------------------
// Shared action helpers
// ---------------------------------------------------------------------------

func getRecord(ctx context.Context, model string, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil, fmt.Errorf("record not found")
	}
	return rec, nil
}

func requireOfficer(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "school.group_school_clerk") ||
		orm.UserHasGroupXML(ctx, uid, "school.group_school_manager")
}

var stateLabels = map[string]string{
	"draft": "Draft", "published": "Published", "archived": "Archived",
	"pending": "Pending", "sent": "Sent", "failed": "Failed", "cancelled": "Cancelled",
}

func label(state string) string {
	if l, ok := stateLabels[state]; ok {
		return l
	}
	return state
}

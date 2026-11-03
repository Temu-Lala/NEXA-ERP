package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// Collaboration actions (blueprint §14, §20). Meetings move draft to scheduled to
// held (or cancelled); a held meeting is where minutes are captured. Action items
// run open to in-progress to done, and completing one clears its overdue flag. A
// document is submitted for review then approved by someone other than its owner
// (separation of duties) and can be archived once superseded.

func init() {
	orm.RegisterObjectAction(meetingModel, "action_schedule", meetingSchedule)
	orm.RegisterObjectAction(meetingModel, "action_hold", meetingHold)
	orm.RegisterObjectAction(meetingModel, "action_cancel", meetingCancel)

	orm.RegisterObjectAction(actionModel, "action_start", itemStart)
	orm.RegisterObjectAction(actionModel, "action_complete", itemComplete)
	orm.RegisterObjectAction(actionModel, "action_cancel", itemCancel)

	orm.RegisterObjectAction(docModel, "action_submit", docSubmit)
	orm.RegisterObjectAction(docModel, "action_approve", docApprove)
	orm.RegisterObjectAction(docModel, "action_reject", docReject)
	orm.RegisterObjectAction(docModel, "action_archive", docArchive)
}

func meetingSchedule(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, meetingModel, id)
	if rec == nil {
		return "", fmt.Errorf("meeting not found")
	}
	if s(rec["state"]) != "draft" {
		return "", fmt.Errorf("only draft meetings can be scheduled")
	}
	if err := setState(bypass, meetingModel, id, "scheduled"); err != nil {
		return "", err
	}
	return "Meeting scheduled.", nil
}

func meetingHold(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, meetingModel, id)
	if rec == nil {
		return "", fmt.Errorf("meeting not found")
	}
	if s(rec["state"]) != "scheduled" {
		return "", fmt.Errorf("only scheduled meetings can be held")
	}
	if s(rec["minutes"]) == "" {
		return "", fmt.Errorf("record the minutes before marking a meeting held")
	}
	if err := setState(bypass, meetingModel, id, "held"); err != nil {
		return "", err
	}
	return "Meeting marked held.", nil
}

func meetingCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, meetingModel, id)
	if rec == nil {
		return "", fmt.Errorf("meeting not found")
	}
	if s(rec["state"]) == "held" {
		return "", fmt.Errorf("a held meeting cannot be cancelled")
	}
	if s(rec["state"]) == "cancelled" {
		return "", fmt.Errorf("meeting is already cancelled")
	}
	if err := setState(bypass, meetingModel, id, "cancelled"); err != nil {
		return "", err
	}
	return "Meeting cancelled.", nil
}

func itemStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, actionModel, id)
	if rec == nil {
		return "", fmt.Errorf("action item not found")
	}
	if s(rec["state"]) != "open" {
		return "", fmt.Errorf("only open action items can be started")
	}
	if err := orm.UpdateRecordByID(bypass, actionModel, id, map[string]interface{}{
		"state": "in_progress", "overdue": false,
	}); err != nil {
		return "", err
	}
	return "Action item started.", nil
}

func itemComplete(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, actionModel, id)
	if rec == nil {
		return "", fmt.Errorf("action item not found")
	}
	if s(rec["state"]) != "open" && s(rec["state"]) != "in_progress" {
		return "", fmt.Errorf("only open or in-progress action items can be completed")
	}
	if err := orm.UpdateRecordByID(bypass, actionModel, id, map[string]interface{}{
		"state": "done", "overdue": false,
	}); err != nil {
		return "", err
	}
	return "Action item completed.", nil
}

func itemCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, actionModel, id)
	if rec == nil {
		return "", fmt.Errorf("action item not found")
	}
	if s(rec["state"]) == "done" {
		return "", fmt.Errorf("a completed action item cannot be cancelled")
	}
	if err := orm.UpdateRecordByID(bypass, actionModel, id, map[string]interface{}{
		"state": "cancelled", "overdue": false,
	}); err != nil {
		return "", err
	}
	return "Action item cancelled.", nil
}

func docSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, docModel, id)
	if rec == nil {
		return "", fmt.Errorf("document not found")
	}
	if s(rec["state"]) != "draft" {
		return "", fmt.Errorf("only draft documents can be submitted for review")
	}
	if err := setState(bypass, docModel, id, "in_review"); err != nil {
		return "", err
	}
	return "Document submitted for review.", nil
}

// docApprove enforces that the approver is not the document owner.
func docApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, docModel, id)
	if rec == nil {
		return "", fmt.Errorf("document not found")
	}
	if s(rec["state"]) != "in_review" {
		return "", fmt.Errorf("only documents under review can be approved")
	}
	uid := orm.UIDFromContext(ctx)
	owner := idOf(rec["owner_id"])
	if uid > 0 && owner > 0 && uid == owner {
		return "", fmt.Errorf("a document must be approved by someone other than its owner")
	}
	upd := map[string]interface{}{"state": "approved"}
	if uid > 0 {
		upd["approved_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, docModel, id, upd); err != nil {
		return "", err
	}
	return "Document approved.", nil
}

func docReject(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, docModel, id)
	if rec == nil {
		return "", fmt.Errorf("document not found")
	}
	if s(rec["state"]) != "in_review" {
		return "", fmt.Errorf("only documents under review can be rejected")
	}
	if err := setState(bypass, docModel, id, "draft"); err != nil {
		return "", err
	}
	return "Document returned to draft.", nil
}

func docArchive(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, docModel, id)
	if rec == nil {
		return "", fmt.Errorf("document not found")
	}
	if s(rec["state"]) != "approved" {
		return "", fmt.Errorf("only approved documents can be archived")
	}
	if err := setState(bypass, docModel, id, "archived"); err != nil {
		return "", err
	}
	return "Document archived.", nil
}

// setState writes a single state value, refreshing the overdue flag is handled by
// the callers that need it.
func setState(ctx context.Context, model string, id int, state string) error {
	return orm.UpdateRecordByID(ctx, model, id, map[string]interface{}{"state": state})
}

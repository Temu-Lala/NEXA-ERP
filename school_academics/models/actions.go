package models

import (
	"context"
	"fmt"
	"strings"

	"nexa/core/orm"
)

// Object actions for academics. The timetable publish flow is the safety gate the
// blueprint asks for: a draft lesson may only be published once the conflict engine
// returns a clean report, and a standalone "check" action lets a scheduler preview the
// conflicts without changing state. Lesson plans move through submit -> approve/reject
// with the reviewer recorded. Publishing is Manager-gated; checking is open to clerks.

func init() {
	orm.RegisterObjectAction(entryModel, "timetable_check", actTimetableCheck)
	orm.RegisterObjectAction(entryModel, "timetable_publish", actTimetablePublish)
	orm.RegisterObjectAction(lessonModel, "lesson_submit", actLessonSubmit)
	orm.RegisterObjectAction(lessonModel, "lesson_approve", actLessonApprove)
	orm.RegisterObjectAction(lessonModel, "lesson_reject", actLessonReject)
}

// mergedEntry returns the candidate values for conflict checking, filling the denormalised
// teacher/section/subject from the assignment when the entry left them blank, and persists
// those fills so the stored row stays consistent with what was validated.
func mergedEntry(ctx context.Context, rec map[string]interface{}) (map[string]interface{}, error) {
	merged := make(map[string]interface{}, len(rec))
	for k, v := range rec {
		merged[k] = v
	}
	assignID := intOr(rec["assignment_id"])
	if assignID <= 0 {
		return merged, nil
	}
	assign, err := orm.SearchOne(ctx, assignmentModel, map[string]interface{}{"id": assignID})
	if err != nil || assign == nil {
		return merged, nil
	}
	fills := map[string]string{"teacher_id": "teacher_id", "section_id": "section_id", "subject_id": "subject_id"}
	changed := map[string]interface{}{}
	for col := range fills {
		if intOr(merged[col]) <= 0 && intOr(assign[col]) > 0 {
			merged[col] = assign[col]
			changed[col] = assign[col]
		}
	}
	if len(changed) > 0 {
		if err := orm.UpdateRecordByID(ctx, entryModel, intOr(rec["id"]), changed); err != nil {
			return merged, err
		}
	}
	return merged, nil
}

func actTimetableCheck(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to validate a timetable entry")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, entryModel, id)
	if err != nil {
		return "", err
	}
	merged, err := mergedEntry(bypass, rec)
	if err != nil {
		return "", err
	}
	conflicts, err := conflictsFor(bypass, merged)
	if err != nil {
		return "", err
	}
	if len(conflicts) == 0 {
		return "No conflicts — this lesson is safe to publish", nil
	}
	return "Conflicts detected:\n- " + strings.Join(conflicts, "\n- "), nil
}

func actTimetablePublish(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can publish a timetable entry")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, entryModel, id)
	if err != nil {
		return "", err
	}
	current := strOr(rec["state"])
	if current == string(TtPublished) {
		return "", fmt.Errorf("this lesson is already published")
	}
	if current == string(TtCancelled) {
		return "", fmt.Errorf("a cancelled lesson cannot be published")
	}
	merged, err := mergedEntry(bypass, rec)
	if err != nil {
		return "", err
	}
	conflicts, err := conflictsFor(bypass, merged)
	if err != nil {
		return "", err
	}
	if len(conflicts) > 0 {
		return "", fmt.Errorf("cannot publish: %s", strings.Join(conflicts, "; "))
	}
	if err := orm.UpdateRecordByID(bypass, entryModel, id, map[string]interface{}{
		"state": string(TtPublished),
	}); err != nil {
		return "", err
	}
	return "Lesson published", nil
}

func actLessonSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to submit a lesson plan")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, lessonModel, id)
	if err != nil {
		return "", err
	}
	current := strOr(rec["state"])
	if current != string(LpDraft) && current != string(LpRejected) {
		return "", fmt.Errorf("only a draft or rejected lesson plan can be submitted (current %s)", current)
	}
	if strings.TrimSpace(strOr(rec["topic"])) == "" {
		return "", fmt.Errorf("a lesson plan needs a topic before it can be submitted")
	}
	if err := orm.UpdateRecordByID(bypass, lessonModel, id, map[string]interface{}{
		"state": string(LpSubmitted),
	}); err != nil {
		return "", err
	}
	return "Lesson plan submitted for review", nil
}

func actLessonApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return reviewLesson(ctx, id, string(LpApproved))
}

func actLessonReject(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	if strings.TrimSpace(args["reason"]) == "" {
		return "", fmt.Errorf("a rejection must record a reason")
	}
	return reviewLesson(ctx, id, string(LpRejected))
}

// reviewLesson moves a submitted lesson plan to approved/rejected and records the reviewer.
func reviewLesson(ctx context.Context, id int, target string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can review a lesson plan")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, lessonModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(LpSubmitted) {
		return "", fmt.Errorf("only a submitted lesson plan can be reviewed (current %s)", strOr(rec["state"]))
	}
	if err := orm.UpdateRecordByID(bypass, lessonModel, id, map[string]interface{}{
		"state":          target,
		"approved_by_id": uid,
	}); err != nil {
		return "", err
	}
	if target == string(LpApproved) {
		return "Lesson plan approved", nil
	}
	return "Lesson plan rejected", nil
}

// ---------------------------------------------------------------------------
// Permission / record helpers (self-contained to this module)
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

func requireManager(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "school.group_school_manager")
}

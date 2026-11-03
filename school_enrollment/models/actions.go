package models

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"nexa/core/orm"
)

// Object actions drive the enrollment lifecycle. Each button maps to a transition in
// the engine's table. Confirm activates a placement (blocked if the learner already has
// an active enrollment for that academic year); Transfer closes the current enrollment
// and opens a fresh one in the new section/year so history is preserved; Graduate closes
// the enrollment and writes the alumni ledger row; Withdraw / Cancel end a placement.
// Every move appends to school.enrollment.history. Manager-only moves are the
// consequential placement re-writes (transfer, graduate, re-enroll).

func init() {
	orm.RegisterObjectAction(enrollmentModel, "enrollment_confirm", actConfirm)
	orm.RegisterObjectAction(enrollmentModel, "enrollment_transfer", actTransfer)
	orm.RegisterObjectAction(enrollmentModel, "enrollment_withdraw", actWithdraw)
	orm.RegisterObjectAction(enrollmentModel, "enrollment_graduate", actGraduate)
	orm.RegisterObjectAction(enrollmentModel, "enrollment_cancel", actCancel)
	orm.RegisterObjectAction(enrollmentModel, "enrollment_reenroll", actReenroll)
}

func actConfirm(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to confirm an enrollment")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, id)
	if err != nil {
		return "", err
	}
	current := strOr(rec["state"])
	if !allowed("confirm", current) {
		return "", fmt.Errorf("only a draft enrollment can be confirmed (current status %q)", label(current))
	}
	studentID := intOr(rec["student_id"])
	yearID := intOr(rec["academic_year_id"])
	dup, _ := activeEnrollments(bypass, studentID, yearID)
	if blocksDuplicate(dup) {
		return "", fmt.Errorf("learner already has an active enrollment for this academic year")
	}
	if err := orm.UpdateRecordByID(bypass, enrollmentModel, id, map[string]interface{}{
		"state":       string(EnrActive),
		"enroll_date": time.Now().UTC().Format(dateLayout),
	}); err != nil {
		return "", err
	}
	appendHistory(bypass, id, studentID, current, string(EnrActive), "confirm", 0, intOr(rec["section_id"]), reason(args), uid)
	return "Enrollment confirmed and activated", nil
}

func actWithdraw(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to withdraw an enrollment")
	}
	return closeMove(ctx, uid, id, "withdraw", args)
}

func actCancel(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to cancel an enrollment")
	}
	return closeMove(ctx, uid, id, "cancel", args)
}

// closeMove handles the simple active/draft -> closed transitions that just set the end
// date and state (withdraw, cancel), recording the history row.
func closeMove(ctx context.Context, uid, id int, action string, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, id)
	if err != nil {
		return "", err
	}
	current := strOr(rec["state"])
	if !allowed(action, current) {
		return "", fmt.Errorf("action %q is not allowed from status %q", action, label(current))
	}
	if err := orm.UpdateRecordByID(bypass, enrollmentModel, id, map[string]interface{}{
		"state":     targetState(action),
		"exit_date": time.Now().UTC().Format(dateLayout),
	}); err != nil {
		return "", err
	}
	appendHistory(bypass, id, intOr(rec["student_id"]), current, targetState(action), action, intOr(rec["section_id"]), 0, reason(args), uid)
	return fmt.Sprintf("Enrollment moved to %s", label(targetState(action))), nil
}

// actGraduate closes an active enrollment as graduated and creates the alumni record.
func actGraduate(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can graduate a learner")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, id)
	if err != nil {
		return "", err
	}
	current := strOr(rec["state"])
	if !allowed("graduate", current) {
		return "", fmt.Errorf("only an active enrollment can be graduated (current status %q)", label(current))
	}
	today := time.Now().UTC().Format(dateLayout)
	if err := orm.UpdateRecordByID(bypass, enrollmentModel, id, map[string]interface{}{
		"state":     string(EnrGraduated),
		"exit_date": today,
	}); err != nil {
		return "", err
	}
	studentID := intOr(rec["student_id"])
	classification := args["classification"]
	if classification == "" {
		classification = string(GradPass)
	}
	alumniID, err := orm.Create(bypass, orm.RegistryModel(alumniModel), map[string]interface{}{
		"student_id":       studentID,
		"enrollment_id":    id,
		"academic_year_id": intOr(rec["academic_year_id"]),
		"graduation_date":  today,
		"classification":   classification,
		"certificate_no":   args["certificate_no"],
		"company_id":       intOr(rec["company_id"]),
		"active":           true,
		"notes":            fmt.Sprintf("Graduated from enrollment %s", strOr(rec["enrollment_no"])),
	})
	if err != nil {
		return "", fmt.Errorf("enrollment graduated but the alumni record could not be created: %w", err)
	}
	appendHistory(bypass, id, studentID, current, string(EnrGraduated), "graduate", intOr(rec["section_id"]), 0, reason(args), uid)
	return fmt.Sprintf("Learner graduated (alumni record #%d)", alumniID), nil
}

// actTransfer closes the current enrollment and opens a new active one in the target
// placement, so the learner's enrollment chain stays auditable.
func actTransfer(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can transfer a learner")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, id)
	if err != nil {
		return "", err
	}
	current := strOr(rec["state"])
	if !allowed("transfer", current) {
		return "", fmt.Errorf("only an active enrollment can be transferred (current status %q)", label(current))
	}
	studentID := intOr(rec["student_id"])
	oldSection := intOr(rec["section_id"])

	newYearID := argInt(args, "to_academic_year_id", intOr(rec["academic_year_id"]))
	newClassID := argInt(args, "to_class_id", intOr(rec["class_id"]))
	newSectionID := argInt(args, "to_section_id", 0)
	newCampusID := argInt(args, "to_campus_id", intOr(rec["campus_id"]))
	if newSectionID == 0 {
		return "", fmt.Errorf("a transfer must name a destination section")
	}
	dup, _ := activeEnrollments(bypass, studentID, newYearID)
	if blocksDuplicate(dup) {
		return "", fmt.Errorf("learner already has an active enrollment in the destination year")
	}

	today := time.Now().UTC().Format(dateLayout)
	// Close the outgoing enrollment.
	if err := orm.UpdateRecordByID(bypass, enrollmentModel, id, map[string]interface{}{
		"state":     string(EnrTransferred),
		"exit_date": today,
	}); err != nil {
		return "", err
	}
	appendHistory(bypass, id, studentID, current, string(EnrTransferred), "transfer", oldSection, newSectionID, reason(args), uid)

	// Open the incoming enrollment.
	seq := studentEnrollmentCount(bypass, studentID) + 1
	newNo := fmt.Sprintf("%s-T%d", strOr(rec["enrollment_no"]), seq)
	newID, err := orm.Create(bypass, orm.RegistryModel(enrollmentModel), map[string]interface{}{
		"enrollment_no":    newNo,
		"student_id":       studentID,
		"academic_year_id": newYearID,
		"campus_id":        newCampusID,
		"class_id":         newClassID,
		"section_id":       newSectionID,
		"enroll_date":      today,
		"state":            string(EnrActive),
		"source":           "transfer",
		"company_id":       intOr(rec["company_id"]),
		"active":           true,
		"notes":            fmt.Sprintf("Transferred from enrollment %s", strOr(rec["enrollment_no"])),
	})
	if err != nil {
		return "", fmt.Errorf("outgoing enrollment closed but the transfer-in enrollment could not be created: %w", err)
	}
	appendHistory(bypass, newID, studentID, "", string(EnrActive), "transfer_in", oldSection, newSectionID, reason(args), uid)
	return fmt.Sprintf("Learner transferred; new enrollment %s opened", newNo), nil
}

// actReenroll opens a fresh active enrollment for a learner whose current chain is
// closed (e.g. re-enrolling for the next academic year after withdrawal).
func actReenroll(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can re-enroll a learner")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, id)
	if err != nil {
		return "", err
	}
	current := strOr(rec["state"])
	if current == string(EnrActive) || current == string(EnrDraft) {
		return "", fmt.Errorf("only a closed enrollment can be re-enrolled (current status %q)", label(current))
	}
	studentID := intOr(rec["student_id"])
	newYearID := argInt(args, "to_academic_year_id", 0)
	if newYearID == 0 {
		return "", fmt.Errorf("a re-enrollment must name the target academic year")
	}
	dup, _ := activeEnrollments(bypass, studentID, newYearID)
	if blocksDuplicate(dup) {
		return "", fmt.Errorf("learner already has an active enrollment in the target year")
	}
	newSectionID := argInt(args, "to_section_id", intOr(rec["section_id"]))
	today := time.Now().UTC().Format(dateLayout)
	seq := studentEnrollmentCount(bypass, studentID) + 1
	newNo := fmt.Sprintf("%s-R%d", strOr(rec["enrollment_no"]), seq)
	newID, err := orm.Create(bypass, orm.RegistryModel(enrollmentModel), map[string]interface{}{
		"enrollment_no":    newNo,
		"student_id":       studentID,
		"academic_year_id": newYearID,
		"campus_id":        intOr(rec["campus_id"]),
		"class_id":         argInt(args, "to_class_id", intOr(rec["class_id"])),
		"section_id":       newSectionID,
		"enroll_date":      today,
		"state":            string(EnrActive),
		"source":           "re-enrollment",
		"company_id":       intOr(rec["company_id"]),
		"active":           true,
		"notes":            fmt.Sprintf("Re-enrolled from enrollment %s", strOr(rec["enrollment_no"])),
	})
	if err != nil {
		return "", fmt.Errorf("re-enrollment could not be created: %w", err)
	}
	appendHistory(bypass, newID, studentID, "", string(EnrActive), "reenroll", 0, newSectionID, reason(args), uid)
	return fmt.Sprintf("Learner re-enrolled as %s", newNo), nil
}

// ---------------------------------------------------------------------------
// Shared helpers (self-contained to this module)
// ---------------------------------------------------------------------------

// activeEnrollments counts active placements for a learner in an academic year.
func activeEnrollments(ctx context.Context, studentID, yearID int) (int, error) {
	if studentID <= 0 || yearID <= 0 {
		return 0, nil
	}
	return orm.SearchCount(ctx, enrollmentModel, [][]interface{}{
		{"student_id", "=", studentID},
		{"academic_year_id", "=", yearID},
		{"state", "=", string(EnrActive)},
	})
}

// studentEnrollmentCount counts all enrollment rows for a learner (used to number
// generated transfer / re-enrollment records).
func studentEnrollmentCount(ctx context.Context, studentID int) int {
	if studentID <= 0 {
		return 0
	}
	n, err := orm.SearchCount(ctx, enrollmentModel, [][]interface{}{
		{"student_id", "=", studentID},
	})
	if err != nil {
		return 0
	}
	return n
}

func getRecord(ctx context.Context, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, enrollmentModel, map[string]interface{}{"id": id})
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

func reason(args map[string]string) string { return args["reason"] }

// argInt reads an integer arg, falling back to def when absent or unparsable.
func argInt(args map[string]string, key string, def int) int {
	if v, ok := args[key]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

var stateLabels = map[string]string{
	"draft": "Draft", "active": "Active", "transferred": "Transferred",
	"withdrawn": "Withdrawn", "graduated": "Graduated", "cancelled": "Cancelled",
}

func label(state string) string {
	if l, ok := stateLabels[state]; ok {
		return l
	}
	return state
}

package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Object actions drive the pipeline. Each button name maps to a transition in the
// engine's table; the action re-validates the legal move, enforces the group gate,
// appends the decision trail, and for "Enroll" additionally checks remaining seats
// and creates the school.student. No action lets a state jump that the transition
// table disallows, and enrollment stays a separate, Manager-gated, seat-guarded step.

func init() {
	orm.RegisterObjectAction(applicantModel, "applicant_submit", actSubmit)
	orm.RegisterObjectAction(applicantModel, "applicant_review", actReview)
	orm.RegisterObjectAction(applicantModel, "applicant_assess", actAssess)
	orm.RegisterObjectAction(applicantModel, "applicant_offer", actOffer)
	orm.RegisterObjectAction(applicantModel, "applicant_waitlist", actWaitlist)
	orm.RegisterObjectAction(applicantModel, "applicant_reject", actReject)
	orm.RegisterObjectAction(applicantModel, "applicant_accept", actAccept)
	orm.RegisterObjectAction(applicantModel, "applicant_withdraw", actWithdraw)
	orm.RegisterObjectAction(applicantModel, "applicant_enroll", actEnroll)
}

func actSubmit(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	return transitionTo(ctx, id, "submit", args, false)
}
func actReview(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	return transitionTo(ctx, id, "review", args, false)
}
func actAssess(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	return transitionTo(ctx, id, "assess", args, false)
}
func actOffer(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	return transitionTo(ctx, id, "offer", args, true)
}
func actWaitlist(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	return transitionTo(ctx, id, "waitlist", args, true)
}
func actReject(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	return transitionTo(ctx, id, "reject", args, true)
}
func actAccept(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	return transitionTo(ctx, id, "accept", args, false)
}
func actWithdraw(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	return transitionTo(ctx, id, "withdraw", args, false)
}

// managerOnly marks the consequential moves (offer/waitlist/reject) that require the
// Director group; the rest are open to a clerk or manager.
func transitionTo(ctx context.Context, id int, action string, args map[string]string, managerOnly bool) (string, error) {
	uid := orm.SecurityUID(ctx)
	if managerOnly {
		if !requireManager(ctx, uid) {
			return "", fmt.Errorf("only a School Manager/Director can take this admission decision")
		}
	} else if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission for this admission step")
	}

	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, applicantModel, id)
	if err != nil {
		return "", err
	}
	current := strOr(rec["state"])
	if !allowed(action, current) {
		return "", fmt.Errorf("action %q is not allowed from status %q", action, current)
	}

	// An offer may only go out once the required-document checklist is satisfied.
	if action == "offer" {
		complete, _ := computeDocs(bypass, rec)
		if ok, _ := complete.(bool); !ok {
			return "", fmt.Errorf("cannot offer: required documents are still outstanding")
		}
	}

	target := targetState(action)
	vals := map[string]interface{}{"state": target}
	if action == "offer" {
		vals["decision_by_id"] = uid
		vals["decision_date"] = time.Now().UTC().Format(dateLayout)
		if exp := args["offer_expiry"]; exp != "" {
			vals["offer_expiry"] = exp
		}
	}
	if action == "waitlist" {
		vals["decision_by_id"] = uid
		vals["decision_date"] = time.Now().UTC().Format(dateLayout)
	}
	if err := orm.UpdateRecordByID(bypass, applicantModel, id, vals); err != nil {
		return "", err
	}
	appendDecision(bypass, id, current, target, action, reason(args), uid)
	return fmt.Sprintf("Application moved to %s", label(target)), nil
}

// actEnroll is the one step that leaves admissions and enters the student ledger:
// it is Manager-gated, requires an accepted offer with a free seat, then creates the
// school.student and links it back. Admission approval (offer/accept) alone never
// enrolls anyone.
func actEnroll(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can enroll a student")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, applicantModel, id)
	if err != nil {
		return "", err
	}
	current := strOr(rec["state"])
	if !allowed("enroll", current) {
		return "", fmt.Errorf("only an accepted offer can be enrolled (current status %q)", current)
	}
	if intOr(rec["student_id"]) > 0 {
		return "", fmt.Errorf("this application has already been enrolled")
	}
	sectionID := intOr(rec["section_id"])
	if sectionID > 0 && !sectionHasSeat(bypass, sectionID) {
		return "", fmt.Errorf("no seat available in the selected section")
	}

	studentID, err := orm.Create(bypass, orm.RegistryModel(studentModel), map[string]interface{}{
		"name":             fmt.Sprintf("%s %s", strOr(rec["first_name"]), strOr(rec["last_name"])),
		"code":             strOr(rec["application_no"]),
		"gender":           strOr(rec["gender"]),
		"date_birth":       strOr(rec["date_birth"]),
		"class_id":         sectionOrClass(rec, "class_id"),
		"section_id":       sectionID,
		"academic_year_id": intOr(rec["academic_year_id"]),
		"guardian_name":    strOr(rec["guardian_name"]),
		"guardian_phone":   strOr(rec["guardian_phone"]),
		"guardian_email":   strOr(rec["guardian_email"]),
		"admission_date":   time.Now().UTC().Format(dateLayout),
		"status":           "active",
		"company_id":       intOr(rec["company_id"]),
		"notes":            fmt.Sprintf("Enrolled from admission application %s", strOr(rec["application_no"])),
	})
	if err != nil {
		return "", fmt.Errorf("could not create the student record: %w", err)
	}
	if err := orm.UpdateRecordByID(bypass, applicantModel, id, map[string]interface{}{
		"state":      string(StateEnrolled),
		"student_id": studentID,
	}); err != nil {
		return "", err
	}
	appendDecision(bypass, id, current, string(StateEnrolled), "enroll", reason(args), uid)
	return fmt.Sprintf("Student enrolled (record #%d)", studentID), nil
}

// ---------------------------------------------------------------------------
// Permission / record helpers (self-contained to this module)
// ---------------------------------------------------------------------------

func requireOfficer(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "school.group_school_clerk") ||
		orm.UserHasGroupXML(ctx, uid, "school.group_school_manager")
}

func requireManager(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "school.group_school_manager")
}

func getRecord(ctx context.Context, model string, id int) (map[string]interface{}, error) {
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil, fmt.Errorf("record not found")
	}
	return rec, nil
}

func reason(args map[string]string) string {
	if r := args["reason"]; r != "" {
		return r
	}
	return ""
}

// sectionOrClass extracts an m2o id value (works whether the ORM returns an int or
// a [id, name] pair) for the class link when copying an applicant to a student.
func sectionOrClass(rec map[string]interface{}, field string) int {
	return intOr(rec[field])
}

var stateLabels = map[string]string{
	"draft": "Draft", "submitted": "Submitted", "under_review": "Under Review",
	"assessment": "Assessment", "offered": "Offered", "waitlisted": "Waitlisted",
	"accepted": "Accepted", "rejected": "Rejected", "enrolled": "Enrolled",
	"cancelled": "Cancelled", "expired": "Expired",
}

func label(state string) string {
	if l, ok := stateLabels[state]; ok {
		return l
	}
	return state
}

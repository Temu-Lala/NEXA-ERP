package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Object actions enforce the operational guards that keep the school's resources
// consistent. A transport assignment is confirmed only when the route's vehicle still has
// a free seat. A library loan is issued only when a copy is available (decrementing the
// book's available copies) and is returned by restoring a copy and levying a fine for any
// overdue days. A boarder is admitted only when the dormitory has a free bed and is
// released on the day of leaving. An inventory movement is posted by folding its quantity
// into the item's on-hand stock, and an issue that would drive stock negative is refused.
// A cafeteria subscription moves through pending, active, paused and ended. Everything is
// applied through the tested pure helpers, with access bypass and state re-checks.

const (
	defaultLoanDays   = 14
	defaultFinePerDay = 0.5
)

func init() {
	orm.RegisterObjectAction(transportAssign, "transport_assignment_confirm", actTransportConfirm)
	orm.RegisterObjectAction(loanModel, "loan_issue", actLoanIssue)
	orm.RegisterObjectAction(loanModel, "loan_return", actLoanReturn)
	orm.RegisterObjectAction(boarderModel, "boarder_admit", actBoarderAdmit)
	orm.RegisterObjectAction(boarderModel, "boarder_release", actBoarderRelease)
	orm.RegisterObjectAction(movementModel, "movement_post", actMovementPost)
	orm.RegisterObjectAction(subscriptionModel, "subscription_activate", actSubscriptionActivate)
	orm.RegisterObjectAction(subscriptionModel, "subscription_end", actSubscriptionEnd)
}

func today() string { return time.Now().Format(dateLayout) }

func actTransportConfirm(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to confirm a transport assignment")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, transportAssign, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(TAssignPending) {
		return "", fmt.Errorf("only a pending assignment can be confirmed (current status %q)", label(strOr(rec["state"])))
	}
	routeID := intOr(rec["route_id"])
	route, err := getRecord(bypass, routeModel, routeID)
	if err != nil {
		return "", err
	}
	capacity := 0
	if vid := intOr(route["vehicle_id"]); vid > 0 {
		veh, err := getRecord(bypass, vehicleModel, vid)
		if err != nil {
			return "", err
		}
		capacity = intOr(veh["capacity_seats"])
	}
	used, err := orm.SearchCount(bypass, transportAssign, [][]interface{}{
		{"route_id", "=", routeID},
		{"state", "=", string(TAssignActive)},
	})
	if err != nil {
		return "", err
	}
	if !seatAvailable(capacity, used) {
		return "", fmt.Errorf("route %s has no free seats (capacity %d, assigned %d)", strOr(route["name"]), capacity, used)
	}
	if err := orm.UpdateRecordByID(bypass, transportAssign, id, map[string]interface{}{"state": string(TAssignActive)}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Assignment confirmed (%d/%d seats used)", used+1, capacity), nil
}

func actLoanIssue(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to issue a loan")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, loanModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(LoanRequested) {
		return "", fmt.Errorf("only a requested loan can be issued (current status %q)", label(strOr(rec["state"])))
	}
	bookID := intOr(rec["book_id"])
	book, err := getRecord(bypass, bookModel, bookID)
	if err != nil {
		return "", err
	}
	avail := intOr(book["available_copies"])
	if avail <= 0 {
		return "", fmt.Errorf("no copies of %s are available", strOr(book["title"]))
	}
	loanDate := strOr(rec["loan_date"])
	if loanDate == "" {
		loanDate = today()
	}
	dueDate := strOr(rec["due_date"])
	if dueDate == "" {
		dueDate = addDays(loanDate, defaultLoanDays)
	}
	if err := orm.UpdateRecordByID(bypass, bookModel, bookID, map[string]interface{}{"available_copies": avail - 1}); err != nil {
		return "", err
	}
	if err := orm.UpdateRecordByID(bypass, loanModel, id, map[string]interface{}{
		"state":     string(LoanBorrowed),
		"loan_date": loanDate,
		"due_date":  dueDate,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Loan issued; due %s", dueDate), nil
}

func actLoanReturn(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to return a loan")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, loanModel, id)
	if err != nil {
		return "", err
	}
	st := strOr(rec["state"])
	if st != string(LoanBorrowed) && st != string(LoanOverdue) {
		return "", fmt.Errorf("only a borrowed or overdue loan can be returned (current status %q)", label(st))
	}
	returnDate := today()
	fine := loanFine(overdueDays(strOr(rec["due_date"]), returnDate), defaultFinePerDay)
	bookID := intOr(rec["book_id"])
	if book, err := getRecord(bypass, bookModel, bookID); err == nil {
		total := intOr(book["total_copies"])
		avail := intOr(book["available_copies"]) + 1
		if total > 0 && avail > total {
			avail = total
		}
		_ = orm.UpdateRecordByID(bypass, bookModel, bookID, map[string]interface{}{"available_copies": avail})
	}
	if err := orm.UpdateRecordByID(bypass, loanModel, id, map[string]interface{}{
		"state":       string(LoanReturned),
		"return_date": returnDate,
		"fine_amount": fine,
	}); err != nil {
		return "", err
	}
	if fine > 0 {
		return fmt.Sprintf("Loan returned; fine %.2f for late return", fine), nil
	}
	return "Loan returned on time", nil
}

func actBoarderAdmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to admit a boarder")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, boarderModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(BoardPending) {
		return "", fmt.Errorf("only a pending boarder can be admitted (current status %q)", label(strOr(rec["state"])))
	}
	dormID := intOr(rec["dormitory_id"])
	dorm, err := getRecord(bypass, dormModel, dormID)
	if err != nil {
		return "", err
	}
	occupied, err := orm.SearchCount(bypass, boarderModel, [][]interface{}{
		{"dormitory_id", "=", dormID},
		{"state", "=", string(BoardActive)},
	})
	if err != nil {
		return "", err
	}
	if !seatAvailable(intOr(dorm["capacity"]), occupied) {
		return "", fmt.Errorf("dormitory %s is full (%d/%d beds)", strOr(dorm["name"]), occupied, intOr(dorm["capacity"]))
	}
	if err := orm.UpdateRecordByID(bypass, boarderModel, id, map[string]interface{}{
		"state":     string(BoardActive),
		"date_from": today(),
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Boarder admitted (%d/%d beds)", occupied+1, intOr(dorm["capacity"])), nil
}

func actBoarderRelease(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to release a boarder")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, boarderModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(BoardActive) {
		return "", fmt.Errorf("only an active boarder can be released (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, boarderModel, id, map[string]interface{}{
		"state":   string(BoardLeft),
		"date_to": today(),
	}); err != nil {
		return "", err
	}
	return "Boarder released", nil
}

// actMovementPost folds a draft movement into the item's on-hand stock. An issue that
// would drive stock negative is refused, leaving both records unchanged.
func actMovementPost(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to post a movement")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, movementModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(MvDraft) {
		return "", fmt.Errorf("only a draft movement can be posted (current status %q)", label(strOr(rec["state"])))
	}
	itemID := intOr(rec["item_id"])
	item, err := getRecord(bypass, itemModel, itemID)
	if err != nil {
		return "", err
	}
	newQty, ok := applyMovement(floatOr(item["on_hand_qty"]), floatOr(rec["quantity"]), strOr(rec["move_type"]))
	if !ok {
		return "", fmt.Errorf("issue of %.2f exceeds on-hand stock of %.2f", floatOr(rec["quantity"]), floatOr(item["on_hand_qty"]))
	}
	md := strOr(rec["move_date"])
	if md == "" {
		md = today()
		_ = orm.UpdateRecordByID(bypass, movementModel, id, map[string]interface{}{"move_date": md})
	}
	if err := orm.UpdateRecordByID(bypass, itemModel, itemID, map[string]interface{}{"on_hand_qty": newQty}); err != nil {
		return "", err
	}
	if err := orm.UpdateRecordByID(bypass, movementModel, id, map[string]interface{}{"state": string(MvPosted)}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Movement posted; on hand now %.2f", newQty), nil
}

func actSubscriptionActivate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to activate a subscription")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, subscriptionModel, id)
	if err != nil {
		return "", err
	}
	st := strOr(rec["state"])
	if st != string(SubPending) && st != string(SubPaused) {
		return "", fmt.Errorf("only a pending or paused subscription can be activated (current status %q)", label(st))
	}
	if err := orm.UpdateRecordByID(bypass, subscriptionModel, id, map[string]interface{}{"state": string(SubActive)}); err != nil {
		return "", err
	}
	return "Subscription activated", nil
}

func actSubscriptionEnd(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to end a subscription")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, subscriptionModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) == string(SubEnded) {
		return "", fmt.Errorf("this subscription has already ended")
	}
	if err := orm.UpdateRecordByID(bypass, subscriptionModel, id, map[string]interface{}{
		"state":   string(SubEnded),
		"date_to": today(),
	}); err != nil {
		return "", err
	}
	return "Subscription ended", nil
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

func requireManager(ctx context.Context, uid int) bool {
	return orm.UserHasGroupXML(ctx, uid, "school.group_school_manager")
}

var stateLabels = map[string]string{
	"pending": "Pending", "active": "Active", "cancelled": "Cancelled", "left": "Left",
	"requested": "Requested", "borrowed": "Borrowed", "returned": "Returned", "overdue": "Overdue",
	"draft": "Draft", "posted": "Posted", "paused": "Paused", "ended": "Ended",
}

func label(state string) string {
	if l, ok := stateLabels[state]; ok {
		return l
	}
	return state
}

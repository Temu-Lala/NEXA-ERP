package models

import (
	"context"
	"fmt"
	"math"
	"time"

	"nexa/core/event"
	"nexa/core/orm"
)

// CSAT behaviour layer. A single resolution event enqueues a satisfaction
// invitation for the resolved ticket (idempotent — one open survey per ticket),
// and answering a survey marks it answered and, when the rating is low, posts an
// internal alert onto the ticket's own message thread. Live satisfaction metrics
// (average rating, CSAT %, response count, low-rating flag) are non-stored
// computes so they never drift from the underlying responses.

func init() {
	orm.RegisterCompute("helpdesk.survey", "avg_rating", nil, computeSurveyAvgRating)
	orm.RegisterCompute("helpdesk.survey", "response_count", nil, computeSurveyResponseCount)
	orm.RegisterCompute("helpdesk.survey", "satisfaction", nil, computeSurveySatisfaction)
	orm.RegisterCompute("helpdesk.survey.response", "low_rating", nil, computeResponseLowRating)

	event.Subscribe(orm.EventRecordUpdated, onTicketUpdatedForCSAT)
	event.Subscribe(orm.EventRecordCreated, onResponseChanged)
	event.Subscribe(orm.EventRecordUpdated, onResponseChanged)
}

// ---------------------------------------------------------------------------
// Delivery: enqueue a survey when a ticket is resolved (once per ticket)
// ---------------------------------------------------------------------------

func onTicketUpdatedForCSAT(ctx context.Context, ev event.Event) error {
	model, _ := ev.Payload["model"].(string)
	if model != "helpdesk.ticket" {
		return nil
	}
	id, ok := orm.CoerceInt64(ev.Payload["id"])
	if !ok || id <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": int(id)})
	if err != nil || rec == nil {
		return nil
	}
	if orm.AsString(rec["state"]) != "resolved" {
		return nil
	}
	// Idempotency: skip if the ticket already has any survey response.
	existing, err := orm.SearchCount(bypass, "helpdesk.survey.response",
		[][]interface{}{{"ticket_id", '=', int(id)}})
	if err == nil && existing > 0 {
		return nil
	}
	surveyID := defaultSurveyID(bypass)
	inst, ok := orm.Registry["helpdesk.survey.response"]
	if !ok {
		return nil
	}
	vals := map[string]interface{}{
		"ticket_id":   int(id),
		"state":       string(ResponseStateSent),
		"sent_at":     nowDT(),
		"partner_id":  intOr(rec["partner_id"]),
		"agent_id":    intOr(rec["assigned_to"]),
		"team_id":     intOr(rec["team_id"]),
		"category_id": intOr(rec["category_id"]),
		"company_id":  intOr(rec["company_id"]),
	}
	if surveyID > 0 {
		vals["survey_id"] = surveyID
	}
	_, _ = orm.Create(bypass, inst, vals)
	return nil
}

// ---------------------------------------------------------------------------
// Response lifecycle: mark answered + low-rating alert
// ---------------------------------------------------------------------------

func onResponseChanged(ctx context.Context, ev event.Event) error {
	model, _ := ev.Payload["model"].(string)
	if model != "helpdesk.survey.response" {
		return nil
	}
	id, ok := orm.CoerceInt64(ev.Payload["id"])
	if !ok || id <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": int(id)})
	if err != nil || rec == nil {
		return nil
	}
	rating := intOr(rec["rating"])
	if rating <= 0 {
		return nil
	}

	updates := map[string]interface{}{}
	if orm.AsString(rec["state"]) != string(ResponseStateAnswered) {
		updates["state"] = string(ResponseStateAnswered)
		updates["answered_at"] = nowDT()
	}

	low := isLow(bypass, rec, rating)
	if low && !orm.AsBool(rec["alerted"]) {
		ticketID := intOr(rec["ticket_id"])
		comment := trimStr(orm.AsString(rec["comment"]))
		note := fmt.Sprintf("[CSAT] Low rating received: %d/5.", rating)
		if comment != "" {
			note += " Feedback: " + comment
		}
		addTicketNote(bypass, ticketID, note, intOr(rec["agent_id"]))
		updates["alerted"] = true
		// Record the alert in the ticket history for the audit trail.
		if hist, ok := orm.Registry["helpdesk.ticket.history"]; ok && ticketID > 0 {
			_, _ = orm.Create(bypass, hist, map[string]interface{}{
				"ticket_id": ticketID,
				"action":    "note",
				"field":     "csat",
				"new_value": fmt.Sprintf("%d", rating),
				"summary":   note,
				"date":      nowDT(),
			})
		}
	}

	if len(updates) > 0 {
		_ = orm.UpdateRecordByID(bypass, "helpdesk.survey.response", int(id), updates)
	}
	return nil
}

func isLow(ctx context.Context, rec map[string]interface{}, rating int) bool {
	threshold := 2
	if sid := intOr(rec["survey_id"]); sid > 0 {
		if s, err := orm.SearchOne(ctx, "helpdesk.survey", map[string]interface{}{"id": sid}); err == nil && s != nil {
			if t := intOr(s["low_rating_at"]); t > 0 {
				threshold = t
			}
		}
	}
	return rating <= threshold
}

// ---------------------------------------------------------------------------
// Computed metrics
// ---------------------------------------------------------------------------

func computeSurveyAvgRating(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	rows := answeredRatings(ctx, rec["id"])
	if len(rows) == 0 {
		return 0.0, nil
	}
	var sum int
	for _, r := range rows {
		sum += r
	}
	return round2(float64(sum) / float64(len(rows))), nil
}

func computeSurveyResponseCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	sid, ok := orm.CoerceInt64(rec["id"])
	if !ok || sid <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), "helpdesk.survey.response",
		[][]interface{}{{"survey_id", '=', int(sid)}})
	if err != nil {
		return 0, err
	}
	return n, nil
}

func computeSurveySatisfaction(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	sid, ok := orm.CoerceInt64(rec["id"])
	if !ok || sid <= 0 {
		return 0.0, nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	scale := 5
	if s, err := orm.SearchOne(bypass, "helpdesk.survey", map[string]interface{}{"id": int(sid)}); err == nil && s != nil {
		if sc := intOr(s["scale"]); sc > 0 {
			scale = sc
		}
	}
	cutoff := int(math.Ceil(float64(scale) * 0.8)) // 4 for a 5-point scale
	rows := answeredRatings(bypass, rec["id"])
	if len(rows) == 0 {
		return 0.0, nil
	}
	satisfied := 0
	for _, r := range rows {
		if r >= cutoff {
			satisfied++
		}
	}
	return round1(float64(satisfied) * 100 / float64(len(rows))), nil
}

func answeredRatings(ctx context.Context, surveyID interface{}) []int {
	sid, ok := orm.CoerceInt64(surveyID)
	if !ok || sid <= 0 {
		return nil
	}
	rows, err := orm.Search(ctx, "helpdesk.survey.response",
		[][]interface{}{{"survey_id", "=", int(sid)}, {"rating", ">", 0}})
	if err != nil {
		return nil
	}
	out := make([]int, 0, len(rows))
	for _, r := range rows {
		if v := intOr(r["rating"]); v > 0 {
			out = append(out, v)
		}
	}
	return out
}

func computeResponseLowRating(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	rating := intOr(rec["rating"])
	if rating <= 0 {
		return false, nil
	}
	return isLow(orm.ContextWithBypass(ctx, true), rec, rating), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func defaultSurveyID(ctx context.Context) int {
	rows, err := orm.SearchPage(ctx, "helpdesk.survey",
		[][]interface{}{{"active", "=", true}}, 1, 0, "id ASC")
	if err != nil || len(rows) == 0 {
		return 0
	}
	return intOr(rows[0]["id"])
}

func addTicketNote(ctx context.Context, ticketID int, body string, authorID int) {
	if ticketID <= 0 {
		return
	}
	inst, ok := orm.Registry["helpdesk.ticket.message"]
	if !ok {
		return
	}
	vals := map[string]interface{}{
		"ticket_id":    ticketID,
		"body":         body,
		"message_type": "note",
		"is_internal":  true,
	}
	if authorID > 0 {
		vals["author_id"] = authorID
	}
	_, _ = orm.Create(ctx, inst, vals)
}

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func round1(f float64) float64 {
	return math.Round(f*10) / 10
}

func trimStr(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func nowDT() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}

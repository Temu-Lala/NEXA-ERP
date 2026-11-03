package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Object actions drive the assessment lifecycle. An exam moves draft -> scheduled ->
// conducted -> published; grades may only be entered once it is conducted and are frozen
// when published (publish is blocked with no scores). A report card is generated from the
// learner's published-exam scores (aggregated per subject through the grading scale),
// then submitted and approved by a manager; only approved report cards feed a transcript.
// A transcript is generated from all of a learner's approved report cards and issued with
// a certificate reference. Generation steps rebuild their child lines idempotently.

func init() {
	orm.RegisterObjectAction(examModel, "exam_schedule", actExamSchedule)
	orm.RegisterObjectAction(examModel, "exam_conduct", actExamConduct)
	orm.RegisterObjectAction(examModel, "exam_publish", actExamPublish)

	orm.RegisterObjectAction(reportModel, "report_generate", actReportGenerate)
	orm.RegisterObjectAction(reportModel, "report_submit", actReportSubmit)
	orm.RegisterObjectAction(reportModel, "report_approve", actReportApprove)
	orm.RegisterObjectAction(reportModel, "report_reject", actReportReject)

	orm.RegisterObjectAction(transcriptModel, "transcript_generate", actTranscriptGenerate)
	orm.RegisterObjectAction(transcriptModel, "transcript_issue", actTranscriptIssue)
}

// ---------------------------------------------------------------------------
// Exam lifecycle
// ---------------------------------------------------------------------------

func actExamSchedule(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to schedule an exam")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, examModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(ExamDraft) {
		return "", fmt.Errorf("only a draft exam can be scheduled (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, examModel, id, map[string]interface{}{"state": string(ExamScheduled)}); err != nil {
		return "", err
	}
	return "Exam scheduled", nil
}

func actExamConduct(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to conduct an exam")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, examModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(ExamScheduled) {
		return "", fmt.Errorf("only a scheduled exam can be conducted (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, examModel, id, map[string]interface{}{"state": string(ExamConducted)}); err != nil {
		return "", err
	}
	return "Exam marked as conducted; grades can now be entered", nil
}

func actExamPublish(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can publish exam results")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, examModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(ExamConducted) {
		return "", fmt.Errorf("only a conducted exam can be published (current status %q)", label(strOr(rec["state"])))
	}
	count, err := orm.SearchCount(bypass, scoreModel, [][]interface{}{{"exam_id", "=", id}})
	if err != nil {
		return "", err
	}
	if count == 0 {
		return "", fmt.Errorf("cannot publish an exam with no scores recorded")
	}
	if err := orm.UpdateRecordByID(bypass, examModel, id, map[string]interface{}{"state": string(ExamPublished)}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Results published (%d score(s) now frozen)", count), nil
}

// ---------------------------------------------------------------------------
// Report card
// ---------------------------------------------------------------------------

// actReportGenerate rebuilds the report card's subject lines from the learner's scores on
// published exams in the report's section/term/year, aggregating multiple exams of the
// same subject and grading each aggregate through the resolved scale. Allowed only while
// the card is a draft or was rejected.
func actReportGenerate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to generate a report card")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, reportModel, id)
	if err != nil {
		return "", err
	}
	state := strOr(rec["state"])
	if state != string(RptDraft) && state != string(RptRejected) {
		return "", fmt.Errorf("only a draft or rejected report card can be generated (current status %q)", label(state))
	}
	studentID := intOr(rec["student_id"])
	yearID := intOr(rec["academic_year_id"])
	sectionID := intOr(rec["section_id"])
	termID := intOr(rec["term_id"])
	scaleID := intOr(rec["scale_id"])
	if studentID <= 0 || yearID <= 0 {
		return "", fmt.Errorf("the report card must name a student and academic year before it can be generated")
	}
	bands := bandsForScale(bypass, scaleID)

	// Gather the learner's published exams in scope.
	domain := [][]interface{}{
		{"academic_year_id", "=", yearID},
		{"state", "=", string(ExamPublished)},
	}
	if sectionID > 0 {
		domain = append(domain, []interface{}{"section_id", "=", sectionID})
	}
	if termID > 0 {
		domain = append(domain, []interface{}{"term_id", "=", termID})
	}
	exams, err := orm.Search(bypass, examModel, domain)
	if err != nil {
		return "", fmt.Errorf("published exams could not be loaded: %w", err)
	}

	type agg struct {
		max, obtained float64
	}
	bySubject := map[int]*agg{}
	order := []int{}
	for _, e := range exams {
		examID := intOr(e["id"])
		subjectID := intOr(e["subject_id"])
		if subjectID <= 0 {
			continue
		}
		score, err := orm.SearchOne(bypass, scoreModel, map[string]interface{}{
			"exam_id":    examID,
			"student_id": studentID,
		})
		if err != nil || score == nil {
			continue // learner not scored on this exam
		}
		a := bySubject[subjectID]
		if a == nil {
			a = &agg{}
			bySubject[subjectID] = a
			order = append(order, subjectID)
		}
		a.max += floatOr(e["max_marks"])
		if !boolOr(score["absent"]) {
			a.obtained += floatOr(score["marks_obtained"])
		}
	}
	if len(order) == 0 {
		return "", fmt.Errorf("no published exam scores were found for this learner in scope")
	}

	// Rebuild lines idempotently.
	if _, err := orm.UnlinkWhere(bypass, reportLineModel, [][]interface{}{{"report_card_id", "=", id}}); err != nil {
		return "", fmt.Errorf("existing report lines could not be cleared: %w", err)
	}
	seq := 0
	for _, subjectID := range order {
		a := bySubject[subjectID]
		pct := percentage(a.obtained, a.max)
		grade, points := pickBand(pct, bands)
		seq++
		if _, err := orm.Create(bypass, orm.RegistryModel(reportLineModel), map[string]interface{}{
			"report_card_id": id,
			"subject_id":     subjectID,
			"max_marks":      a.max,
			"obtained_marks": a.obtained,
			"weight":         1,
			"percentage":     pct,
			"grade":          grade,
			"grade_points":   points,
			"remark":         bandRemark(bands, pct),
			"sequence":       seq,
		}); err != nil {
			return "", fmt.Errorf("a subject line could not be written: %w", err)
		}
	}
	return fmt.Sprintf("Report card generated with %d subject line(s)", seq), nil
}

func actReportSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to submit a report card")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, reportModel, id)
	if err != nil {
		return "", err
	}
	state := strOr(rec["state"])
	if state != string(RptDraft) && state != string(RptRejected) {
		return "", fmt.Errorf("only a draft or rejected report card can be submitted (current status %q)", label(state))
	}
	lines, err := orm.SearchCount(bypass, reportLineModel, [][]interface{}{{"report_card_id", "=", id}})
	if err != nil {
		return "", err
	}
	if lines == 0 {
		return "", fmt.Errorf("generate the report card before submitting it")
	}
	if err := orm.UpdateRecordByID(bypass, reportModel, id, map[string]interface{}{"state": string(RptSubmitted)}); err != nil {
		return "", err
	}
	return "Report card submitted for approval", nil
}

func actReportApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can approve a report card")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, reportModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(RptSubmitted) {
		return "", fmt.Errorf("only a submitted report card can be approved (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, reportModel, id, map[string]interface{}{
		"state":          string(RptApproved),
		"approved_by_id": uid,
		"approve_date":   time.Now().UTC().Format(dateLayout),
	}); err != nil {
		return "", err
	}
	return "Report card approved", nil
}

func actReportReject(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can reject a report card")
	}
	if args["reason"] == "" {
		return "", fmt.Errorf("a reason is required to reject a report card")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, reportModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(RptSubmitted) {
		return "", fmt.Errorf("only a submitted report card can be rejected (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, reportModel, id, map[string]interface{}{
		"state":          string(RptRejected),
		"approved_by_id": uid,
		"remarks":        appendRemark(strOr(rec["remarks"]), "Rejected: "+args["reason"]),
	}); err != nil {
		return "", err
	}
	return "Report card rejected; regenerate or revise and resubmit", nil
}

// ---------------------------------------------------------------------------
// Transcript
// ---------------------------------------------------------------------------

// actTranscriptGenerate rebuilds the transcript's course lines from every approved report
// card the learner holds, carrying each subject's credit-weighted result. Allowed while a
// draft.
func actTranscriptGenerate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to generate a transcript")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, transcriptModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(TrxDraft) {
		return "", fmt.Errorf("only a draft transcript can be generated (current status %q)", label(strOr(rec["state"])))
	}
	studentID := intOr(rec["student_id"])
	cards, err := orm.Search(bypass, reportModel, [][]interface{}{
		{"student_id", "=", studentID},
		{"state", "=", string(RptApproved)},
	})
	if err != nil {
		return "", fmt.Errorf("approved report cards could not be loaded: %w", err)
	}
	if len(cards) == 0 {
		return "", fmt.Errorf("no approved report cards found for this learner")
	}
	if _, err := orm.UnlinkWhere(bypass, trxLineModel, [][]interface{}{{"transcript_id", "=", id}}); err != nil {
		return "", fmt.Errorf("existing transcript lines could not be cleared: %w", err)
	}
	seq := 0
	for _, c := range cards {
		cardID := intOr(c["id"])
		lines, err := orm.Search(bypass, reportLineModel, [][]interface{}{{"report_card_id", "=", cardID}})
		if err != nil {
			continue
		}
		for _, l := range lines {
			seq++
			if _, err := orm.Create(bypass, orm.RegistryModel(trxLineModel), map[string]interface{}{
				"transcript_id":    id,
				"report_card_id":   cardID,
				"academic_year_id": intOr(c["academic_year_id"]),
				"term_id":          intOr(c["term_id"]),
				"subject_id":       intOr(l["subject_id"]),
				"credits":          nonZero(floatOr(l["weight"]), 1),
				"percentage":       floatOr(l["percentage"]),
				"grade":            strOr(l["grade"]),
				"grade_points":     floatOr(l["grade_points"]),
				"sequence":         seq,
			}); err != nil {
				return "", fmt.Errorf("a transcript line could not be written: %w", err)
			}
		}
	}
	if seq == 0 {
		return "", fmt.Errorf("the learner's approved report cards carry no subject results")
	}
	return fmt.Sprintf("Transcript generated with %d course line(s)", seq), nil
}

func actTranscriptIssue(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireManager(ctx, uid) {
		return "", fmt.Errorf("only a School Manager/Director can issue a transcript")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, transcriptModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(TrxDraft) {
		return "", fmt.Errorf("only a draft transcript can be issued (current status %q)", label(strOr(rec["state"])))
	}
	lines, err := orm.SearchCount(bypass, trxLineModel, [][]interface{}{{"transcript_id", "=", id}})
	if err != nil {
		return "", err
	}
	if lines == 0 {
		return "", fmt.Errorf("generate the transcript before issuing it")
	}
	cert := args["certificate_no"]
	if cert == "" {
		cert = strOr(rec["certificate_no"])
	}
	if cert == "" {
		cert = fmt.Sprintf("TRX-%d-%d", intOr(rec["student_id"]), id)
	}
	if err := orm.UpdateRecordByID(bypass, transcriptModel, id, map[string]interface{}{
		"state":          string(TrxIssued),
		"issue_date":     time.Now().UTC().Format(dateLayout),
		"certificate_no": cert,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Transcript issued (certificate %s)", cert), nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// bandRemark returns the remark of the qualifying band (empty if none).
func bandRemark(bands []band, pct float64) string {
	var best *band
	for i := range bands {
		b := bands[i]
		if pct >= b.minPct && (best == nil || b.minPct > best.minPct) {
			best = &b
		}
	}
	if best == nil {
		return ""
	}
	return best.remark
}

func nonZero(v, fallback float64) float64 {
	if v <= 0 {
		return fallback
	}
	return v
}

func appendRemark(existing, extra string) string {
	if existing == "" {
		return extra
	}
	return existing + "\n" + extra
}

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
	"draft": "Draft", "scheduled": "Scheduled", "conducted": "Conducted", "published": "Published",
	"submitted": "Submitted", "approved": "Approved", "rejected": "Rejected", "issued": "Issued",
}

func label(state string) string {
	if l, ok := stateLabels[state]; ok {
		return l
	}
	return state
}

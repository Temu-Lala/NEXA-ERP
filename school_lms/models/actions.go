package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
)

// Object actions drive the LMS lifecycle. A course, assignment or quiz is published by an
// officer before learners engage (a quiz publish is blocked when it has no questions); an
// assignment can be closed to stop submissions. A learner's submission is graded by an
// officer, who records a score (validated against the assignment's maximum) and feedback.
// A quiz attempt is auto-graded deterministically when submitted: every recorded answer is
// scored against the question bank (MCQ by selected option, true/false and short answer by
// text match) and the results are folded into a stored score, maximum, percentage and pass
// flag. All mutations run with access bypass and re-check state transitions.

func init() {
	orm.RegisterObjectAction(courseModel, "course_publish", actCoursePublish)
	orm.RegisterObjectAction(courseModel, "course_archive", actCourseArchive)

	orm.RegisterObjectAction(assignmentModel, "assignment_publish", actAssignmentPublish)
	orm.RegisterObjectAction(assignmentModel, "assignment_close", actAssignmentClose)

	orm.RegisterObjectAction(submissionModel, "submission_grade", actSubmissionGrade)

	orm.RegisterObjectAction(quizModel, "quiz_publish", actQuizPublish)

	orm.RegisterObjectAction(attemptModel, "quiz_attempt_submit", actQuizAttemptSubmit)
}

func actCoursePublish(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to publish a course")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, courseModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(CourseDraft) {
		return "", fmt.Errorf("only a draft course can be published (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, courseModel, id, map[string]interface{}{"state": string(CoursePublished)}); err != nil {
		return "", err
	}
	return "Course published", nil
}

func actCourseArchive(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to archive a course")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, courseModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(CoursePublished) {
		return "", fmt.Errorf("only a published course can be archived (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, courseModel, id, map[string]interface{}{"state": string(CourseArchived)}); err != nil {
		return "", err
	}
	return "Course archived", nil
}

func actAssignmentPublish(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to publish an assignment")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, assignmentModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(AssignDraft) {
		return "", fmt.Errorf("only a draft assignment can be published (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, assignmentModel, id, map[string]interface{}{"state": string(AssignPublished)}); err != nil {
		return "", err
	}
	return "Assignment published", nil
}

func actAssignmentClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to close an assignment")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, assignmentModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(AssignPublished) {
		return "", fmt.Errorf("only a published assignment can be closed (current status %q)", label(strOr(rec["state"])))
	}
	if err := orm.UpdateRecordByID(bypass, assignmentModel, id, map[string]interface{}{"state": string(AssignClosed)}); err != nil {
		return "", err
	}
	return "Assignment closed", nil
}

// actSubmissionGrade records a score and feedback and marks a submission graded. The
// score is validated against the parent assignment's maximum, and grading a submission
// whose assignment is still a draft is refused.
func actSubmissionGrade(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to grade a submission")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, submissionModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) == string(SubGraded) {
		return "", fmt.Errorf("this submission is already graded")
	}
	aid := intOr(rec["assignment_id"])
	asg, err := getRecord(bypass, assignmentModel, aid)
	if err != nil {
		return "", err
	}
	if strOr(asg["state"]) == string(AssignDraft) {
		return "", fmt.Errorf("cannot grade a submission for an unpublished assignment")
	}
	maxScore := floatOr(asg["max_score"])
	score := floatOr(rec["score"])
	if score < 0 || (maxScore > 0 && score > maxScore) {
		return "", fmt.Errorf("score %.2f is out of range for this assignment (max %.2f)", score, maxScore)
	}
	if err := orm.UpdateRecordByID(bypass, submissionModel, id, map[string]interface{}{
		"state":        string(SubGraded),
		"graded_by_id": uid,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Submission graded (%.2f / %.2f)", score, maxScore), nil
}

func actQuizPublish(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to publish a quiz")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, quizModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) != string(QuizDraft) {
		return "", fmt.Errorf("only a draft quiz can be published (current status %q)", label(strOr(rec["state"])))
	}
	n, err := orm.SearchCount(bypass, quizLineModel, [][]interface{}{{"quiz_id", "=", id}})
	if err != nil {
		return "", err
	}
	if n <= 0 {
		return "", fmt.Errorf("cannot publish a quiz with no questions")
	}
	if err := orm.UpdateRecordByID(bypass, quizModel, id, map[string]interface{}{"state": string(QuizPublished)}); err != nil {
		return "", err
	}
	return "Quiz published", nil
}

// actQuizAttemptSubmit grades an in-progress attempt: each recorded answer is scored
// against the question bank, and the results are aggregated into stored score/max/
// percentage/passed fields. Re-submitting an already graded attempt is refused.
func actQuizAttemptSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !requireOfficer(ctx, uid) {
		return "", fmt.Errorf("insufficient permission to submit an attempt")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := getRecord(bypass, attemptModel, id)
	if err != nil {
		return "", err
	}
	if strOr(rec["state"]) == string(AttGraded) {
		return "", fmt.Errorf("this attempt has already been graded")
	}
	qzID := intOr(rec["quiz_id"])
	quiz, err := getRecord(bypass, quizModel, qzID)
	if err != nil {
		return "", err
	}
	passPct := floatOr(quiz["pass_percentage"])

	// Per-question effective points from the quiz lines (override or bank value).
	linePts := map[int]float64{}
	lineRows, _ := orm.Search(bypass, quizLineModel, [][]interface{}{{"quiz_id", "=", qzID}})
	for _, ln := range lineRows {
		linePts[intOr(ln["question_id"])] = floatOr(ln["points"])
	}

	answers, err := orm.Search(bypass, answerModel, [][]interface{}{{"attempt_id", "=", id}})
	if err != nil {
		return "", err
	}
	if len(answers) == 0 {
		return "", fmt.Errorf("cannot submit an attempt with no answers")
	}

	var scored, maxes []float64
	for _, a := range answers {
		qid := intOr(a["question_id"])
		q, err := orm.SearchOne(bypass, questionModel, map[string]interface{}{"id": qid})
		if err != nil || q == nil {
			continue
		}
		qMax := effectivePoints(linePts[qid], floatOr(q["points"]))
		correct, earned := gradeAnswer(answerGrade{
			QuestionType:     strOr(q["question_type"]),
			QuestionPoints:   floatOr(q["points"]),
			LinePoints:       linePts[qid],
			SelectedOptionID: intOr(a["selected_option_id"]),
			CorrectOptionIDs: correctOptionIDs(bypass, qid),
			AnswerText:       strOr(a["answer_text"]),
			CorrectAnswer:    strOr(q["correct_answer"]),
		})
		if err := orm.UpdateRecordByID(bypass, answerModel, intOr(a["id"]), map[string]interface{}{
			"correct":       correct,
			"points_scored": earned,
		}); err != nil {
			return "", err
		}
		scored = append(scored, earned)
		maxes = append(maxes, qMax)
	}

	score, max, pct, passed := aggregateAttempt(scored, maxes, passPct)
	if err := orm.UpdateRecordByID(bypass, attemptModel, id, map[string]interface{}{
		"state":        string(AttGraded),
		"submitted_at": time.Now().Format(dateLayout),
		"score":        score,
		"max_points":   max,
		"percentage":   pct,
		"passed":       passed,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Attempt graded (%.2f / %.2f, %.2f%%, %s)", score, max, pct, passFail(passed)), nil
}

// correctOptionIDs loads the ids of a question's correct options.
func correctOptionIDs(ctx context.Context, questionID int) []int {
	rows, err := orm.Search(ctx, optionModel, [][]interface{}{
		{"question_id", "=", questionID},
		{"correct", "=", true},
	})
	if err != nil {
		return nil
	}
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, intOr(r["id"]))
	}
	return ids
}

func passFail(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
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
	"draft": "Draft", "published": "Published", "archived": "Archived", "closed": "Closed",
	"submitted": "Submitted", "graded": "Graded", "in_progress": "In Progress",
}

func label(state string) string {
	if l, ok := stateLabels[state]; ok {
		return l
	}
	return state
}

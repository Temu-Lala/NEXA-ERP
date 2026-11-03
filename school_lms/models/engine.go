package models

import (
	"context"
	"math"
	"strings"

	"nexa/core/orm"
)

// Behaviour layer for the LMS. The scoring arithmetic — percentage, MCQ correctness,
// short/true-false matching, weighted attempt aggregation and pass evaluation, plus
// course-progress completion — is pure and unit-tested. The computes project child counts
// (materials, assignments, quizzes, submissions, questions, attempts, learners), a quiz's
// total points (per-line override or the question's own points) and a learner's progress
// percentage. Attempt grading is materialized by the submit action onto stored fields so
// a graded attempt is deterministic and auditable.

const (
	courseModel     = "school.lms.course"
	materialModel   = "school.lms.material"
	assignmentModel = "school.lms.assignment"
	submissionModel = "school.lms.submission"
	questionModel   = "school.lms.question"
	optionModel     = "school.lms.question.option"
	quizModel       = "school.lms.quiz"
	quizLineModel   = "school.lms.quiz.line"
	attemptModel    = "school.lms.quiz.attempt"
	answerModel     = "school.lms.quiz.answer"
	progressModel   = "school.lms.progress"

	dateLayout = "2006-01-02"
)

// round2 keeps percentages tidy to two decimals.
func round2(f float64) float64 { return math.Round(f*100) / 100 }

// percentage is obtained/max as a percentage; a non-positive max yields 0.
func percentage(obtained, max float64) float64 {
	if max <= 0 {
		return 0
	}
	return round2(obtained / max * 100)
}

// isLate reports whether a submission dated onOrAfter the due date counts as late. A
// blank due date is never late; a blank submitted date (not yet submitted) is not late.
// Dates are compared as ISO yyyy-mm-dd strings, which order chronologically.
func isLate(submitted, due string) bool {
	if due == "" || submitted == "" {
		return false
	}
	return submitted > due
}

// progressPct is viewed/total as a percentage; a non-positive total yields 0 and a
// learner who viewed more than exists is capped at 100.
func progressPct(viewed, total int) float64 {
	if total <= 0 {
		return 0
	}
	if viewed >= total {
		return 100
	}
	if viewed <= 0 {
		return 0
	}
	return round2(float64(viewed) / float64(total) * 100)
}

// completionStatus classifies a progress percentage into a lifecycle label.
func completionStatus(pct float64) string {
	switch {
	case pct >= 100:
		return "completed"
	case pct > 0:
		return "in_progress"
	default:
		return "not_started"
	}
}

// answerGrade is the pure grading of a single answered question.
type answerGrade struct {
	QuestionType     string
	QuestionPoints   float64
	LinePoints       float64 // per-quiz override; <=0 falls back to QuestionPoints
	SelectedOptionID int
	CorrectOptionIDs []int
	AnswerText       string
	CorrectAnswer    string
}

// effectivePoints resolves the points a question is worth in a quiz: the per-line
// override when positive, otherwise the bank's own value.
func effectivePoints(linePoints, questionPoints float64) float64 {
	if linePoints > 0 {
		return linePoints
	}
	return questionPoints
}

// gradeAnswer scores one answer: multiple-choice compares the selected option against
// the set of correct options; true/false and short answer compare the text case- and
// whitespace-insensitively. A correct answer earns the question's effective points.
func gradeAnswer(g answerGrade) (bool, float64) {
	var correct bool
	switch g.QuestionType {
	case "multiple_choice":
		for _, id := range g.CorrectOptionIDs {
			if id == g.SelectedOptionID && id != 0 {
				correct = true
				break
			}
		}
	default:
		want := strings.TrimSpace(g.CorrectAnswer)
		got := strings.TrimSpace(g.AnswerText)
		correct = want != "" && strings.EqualFold(want, got)
	}
	if !correct {
		return false, 0
	}
	return true, effectivePoints(g.LinePoints, g.QuestionPoints)
}

// aggregateAttempt folds per-answer results into a final score, the achievable maximum,
// a percentage and a pass flag against the quiz pass mark.
func aggregateAttempt(scored []float64, maxes []float64, passPct float64) (score, max, pct float64, passed bool) {
	for _, v := range scored {
		score += v
	}
	for _, v := range maxes {
		max += v
	}
	pct = percentage(score, max)
	passed = max > 0 && pct >= passPct
	return round2(score), round2(max), pct, passed
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterCompute(courseModel, "material_count", []string{"id"}, computeCourseMaterialCount)
	orm.RegisterCompute(courseModel, "assignment_count", []string{"id"}, computeCourseAssignmentCount)
	orm.RegisterCompute(courseModel, "quiz_count", []string{"id"}, computeCourseQuizCount)
	orm.RegisterCompute(courseModel, "enrolled_count", []string{"id"}, computeCourseEnrolledCount)
	orm.RegisterCompute(assignmentModel, "submission_count", []string{"id"}, computeAssignmentSubmissionCount)
	orm.RegisterCompute(assignmentModel, "graded_count", []string{"id"}, computeAssignmentGradedCount)
	orm.RegisterCompute(questionModel, "option_count", []string{"id"}, computeQuestionOptionCount)
	orm.RegisterCompute(quizModel, "question_count", []string{"id"}, computeQuizQuestionCount)
	orm.RegisterCompute(quizModel, "total_points", []string{"id"}, computeQuizTotalPoints)
	orm.RegisterCompute(quizModel, "attempt_count", []string{"id"}, computeQuizAttemptCount)
	orm.RegisterCompute(progressModel, "percent_complete", []string{"materials_viewed", "course_id"}, computeProgressPercent)
}

func computeCourseMaterialCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), materialModel, [][]interface{}{{"course_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeCourseAssignmentCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), assignmentModel, [][]interface{}{{"course_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeCourseQuizCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), quizModel, [][]interface{}{{"course_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeCourseEnrolledCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), progressModel, [][]interface{}{{"course_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeAssignmentSubmissionCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), submissionModel, [][]interface{}{{"assignment_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeAssignmentGradedCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), submissionModel, [][]interface{}{
		{"assignment_id", "=", intOr(rec["id"])},
		{"state", "=", string(SubGraded)},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeQuestionOptionCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), optionModel, [][]interface{}{{"question_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeQuizQuestionCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), quizLineModel, [][]interface{}{{"quiz_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeQuizAttemptCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), attemptModel, [][]interface{}{{"quiz_id", "=", intOr(rec["id"])}})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// computeQuizTotalPoints sums the effective points of a quiz's questions, honouring each
// line's override and falling back to the question bank's own point value.
func computeQuizTotalPoints(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	lines, err := orm.Search(bypass, quizLineModel, [][]interface{}{{"quiz_id", "=", intOr(rec["id"])}})
	if err != nil || len(lines) == 0 {
		return 0, nil
	}
	qPoints := map[int]float64{}
	for _, ln := range lines {
		qid := intOr(ln["question_id"])
		if qid <= 0 {
			continue
		}
		if _, ok := qPoints[qid]; !ok {
			if q, err := orm.SearchOne(bypass, questionModel, map[string]interface{}{"id": qid}); err == nil && q != nil {
				qPoints[qid] = floatOr(q["points"])
			}
		}
	}
	var total float64
	for _, ln := range lines {
		qid := intOr(ln["question_id"])
		total += effectivePoints(floatOr(ln["points"]), qPoints[qid])
	}
	return round2(total), nil
}

// computeProgressPercent derives a learner's completion percentage from the number of
// materials viewed against the course's total published-and-active material count.
func computeProgressPercent(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	total, err := orm.SearchCount(bypass, materialModel, [][]interface{}{{"course_id", "=", intOr(rec["course_id"])}})
	if err != nil {
		return 0, nil
	}
	return progressPct(intOr(rec["materials_viewed"]), total), nil
}

// ---------------------------------------------------------------------------
// Shared coercion helpers (ORM values arrive as interface{})
// ---------------------------------------------------------------------------

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func strOr(v interface{}) string { return orm.AsString(v) }

func boolOr(v interface{}) bool { return orm.AsBool(v) }

func floatOr(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

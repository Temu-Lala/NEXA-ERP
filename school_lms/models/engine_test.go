package models

import "testing"

func TestPercentage(t *testing.T) {
	if got := percentage(45, 90); got != 50 {
		t.Fatalf("45/90 want 50 got %v", got)
	}
	if got := percentage(10, 0); got != 0 {
		t.Fatalf("zero max want 0 got %v", got)
	}
	if got := percentage(2, 3); got != 66.67 {
		t.Fatalf("2/3 want 66.67 got %v", got)
	}
}

func TestIsLate(t *testing.T) {
	if isLate("2026-05-10", "2026-05-09") != true {
		t.Fatal("submitted after due should be late")
	}
	if isLate("2026-05-09", "2026-05-09") != false {
		t.Fatal("same day should not be late")
	}
	if isLate("", "2026-05-09") != false {
		t.Fatal("blank submitted should not be late")
	}
	if isLate("2026-05-10", "") != false {
		t.Fatal("blank due should never be late")
	}
}

func TestProgressPctAndStatus(t *testing.T) {
	if got := progressPct(3, 6); got != 50 {
		t.Fatalf("3/6 want 50 got %v", got)
	}
	if got := progressPct(10, 6); got != 100 {
		t.Fatalf("over-total should cap at 100 got %v", got)
	}
	if got := progressPct(0, 0); got != 0 {
		t.Fatalf("zero total want 0 got %v", got)
	}
	if completionStatus(100) != "completed" {
		t.Fatal("100 should be completed")
	}
	if completionStatus(33) != "in_progress" {
		t.Fatal("33 should be in_progress")
	}
	if completionStatus(0) != "not_started" {
		t.Fatal("0 should be not_started")
	}
}

func TestEffectivePoints(t *testing.T) {
	if got := effectivePoints(0, 5); got != 5 {
		t.Fatalf("no override should use bank value got %v", got)
	}
	if got := effectivePoints(8, 5); got != 8 {
		t.Fatalf("positive override should win got %v", got)
	}
}

func TestGradeAnswerMCQ(t *testing.T) {
	correct, pts := gradeAnswer(answerGrade{
		QuestionType:     "multiple_choice",
		QuestionPoints:   4,
		SelectedOptionID: 22,
		CorrectOptionIDs: []int{21, 22},
	})
	if !correct || pts != 4 {
		t.Fatalf("matching MCQ option want correct/4 got %v/%v", correct, pts)
	}
	correct, pts = gradeAnswer(answerGrade{
		QuestionType:     "multiple_choice",
		QuestionPoints:   4,
		SelectedOptionID: 99,
		CorrectOptionIDs: []int{21, 22},
	})
	if correct || pts != 0 {
		t.Fatalf("wrong MCQ option want incorrect/0 got %v/%v", correct, pts)
	}
}

func TestGradeAnswerText(t *testing.T) {
	correct, pts := gradeAnswer(answerGrade{
		QuestionType:   "short_answer",
		QuestionPoints: 3,
		AnswerText:     "  Paris ",
		CorrectAnswer:  "paris",
	})
	if !correct || pts != 3 {
		t.Fatalf("case/space-insensitive match want correct/3 got %v/%v", correct, pts)
	}
	correct, _ = gradeAnswer(answerGrade{
		QuestionType:   "true_false",
		QuestionPoints: 1,
		AnswerText:     "false",
		CorrectAnswer:  "true",
	})
	if correct {
		t.Fatal("mismatched true/false should be incorrect")
	}
	// blank correct answer can never be matched
	correct, _ = gradeAnswer(answerGrade{QuestionType: "short_answer", AnswerText: "anything", CorrectAnswer: ""})
	if correct {
		t.Fatal("blank correct answer should never match")
	}
}

func TestAggregateAttempt(t *testing.T) {
	score, max, pct, passed := aggregateAttempt([]float64{4, 0, 2}, []float64{4, 4, 2}, 50)
	if score != 6 || max != 10 || pct != 60 || !passed {
		t.Fatalf("want 6/10/60/pass got %v/%v/%v/%v", score, max, pct, passed)
	}
	score, max, pct, passed = aggregateAttempt([]float64{1, 0}, []float64{4, 4}, 50)
	if score != 1 || max != 8 || pct != 12.5 || passed {
		t.Fatalf("want 1/8/12.5/fail got %v/%v/%v/%v", score, max, pct, passed)
	}
	// zero maximum never passes
	_, _, _, passed = aggregateAttempt(nil, nil, 0)
	if passed {
		t.Fatal("empty attempt with zero max should not pass")
	}
}

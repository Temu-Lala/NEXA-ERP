package models

import (
	"context"
	"strconv"
	"strings"

	"nexa/core/orm"
)

// Behaviour layer for academics. The timetable conflict engine is pure and
// unit-tested: given a candidate lesson, the lessons already on that weekday, the
// teacher's allowed weekdays and the room capacity it returns every rule the
// assignment would break. The DB-aware conflictsFor() and publish action both reuse
// it, and the computes project live prerequisite / outcome counts and a conflict flag.

const (
	subjectModel    = "school.subject"
	prereqModel     = "school.subject.prerequisite"
	curriculumModel = "school.curriculum"
	outcomeModel    = "school.curriculum.outcome"
	assignmentModel = "school.teaching.assignment"
	entryModel      = "school.timetable.entry"
	lessonModel     = "school.lesson.plan"

	classroomModel = "school.classroom"
)

// entryView is the minimal, DB-free projection of a timetable lesson the conflict
// engine reasons about. Duration is normalised to at least 1 period.
type entryView struct {
	ID           int
	TeacherID    int
	SectionID    int
	RoomID       int
	Weekday      int
	Start        int // period number the lesson begins at
	Duration     int // number of consecutive periods
	StudentCount int
}

// overlaps reports whether two lessons on the same weekday occupy overlapping period
// ranges on the period axis. A non-positive duration is treated as a single period.
func overlaps(aStart, aDur, bStart, bDur int) bool {
	if aDur <= 0 {
		aDur = 1
	}
	if bDur <= 0 {
		bDur = 1
	}
	return aStart < bStart+bDur && bStart < aStart+aDur
}

// teacherAvailable reports whether a weekday is permitted. An empty allow-list means
// the teacher has no day restriction.
func teacherAvailable(allowedDays []int, weekday int) bool {
	if len(allowedDays) == 0 {
		return true
	}
	for _, d := range allowedDays {
		if d == weekday {
			return true
		}
	}
	return false
}

// withinCapacity reports whether the expected headcount fits the room. A capacity of 0
// or less means the room is uncapped.
func withinCapacity(students, capacity int) bool {
	if capacity <= 0 {
		return true
	}
	return students <= capacity
}

// findConflicts returns a human-readable list of every conflict rule `cand` breaks
// against `existing` lessons plus its own availability and room capacity. An empty
// slice means the assignment is safe to publish.
func findConflicts(cand entryView, existing []entryView, allowedDays []int, roomCapacity int) []string {
	var out []string
	for _, e := range existing {
		if e.ID == cand.ID {
			continue
		}
		if !overlaps(cand.Start, cand.Duration, e.Start, e.Duration) {
			continue
		}
		if cand.TeacherID > 0 && e.TeacherID == cand.TeacherID {
			out = append(out, "teacher double-booked in another class at the same time")
		}
		if cand.SectionID > 0 && e.SectionID == cand.SectionID {
			out = append(out, "the same section is in two lessons simultaneously")
		}
		if cand.RoomID > 0 && e.RoomID == cand.RoomID {
			out = append(out, "two classes scheduled in the same room for the same period")
		}
	}
	if !teacherAvailable(allowedDays, cand.Weekday) {
		out = append(out, "the teacher is not available on the chosen weekday")
	}
	if !withinCapacity(cand.StudentCount, roomCapacity) {
		out = append(out, "the expected number of students exceeds the room capacity")
	}
	return out
}

// parseIntList parses a comma-separated list of integers, ignoring blanks and junk.
func parseIntList(csv string) []int {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if n, err := strconv.Atoi(p); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func init() {
	orm.RegisterCompute(subjectModel, "prereq_count", []string{"id"}, computePrereq)
	orm.RegisterCompute(curriculumModel, "outcome_count", []string{"id"}, computeOutcome)
	orm.RegisterCompute(entryModel, "has_conflict", []string{
		"weekday", "period_number", "duration", "section_id", "teacher_id",
		"classroom_id", "student_count", "assignment_id", "id",
	}, computeConflict)
}

func computePrereq(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id := intOr(rec["id"])
	if id <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), prereqModel, [][]interface{}{
		{"subject_id", "=", id},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeOutcome(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id := intOr(rec["id"])
	if id <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), outcomeModel, [][]interface{}{
		{"curriculum_id", "=", id},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func computeConflict(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	conflicts, err := conflictsFor(ctx, rec)
	if err != nil {
		return false, nil
	}
	return len(conflicts) > 0, nil
}

// conflictsFor runs the pure engine against the live schedule: it loads every non-
// cancelled lesson on the candidate's weekday, resolves the teacher's allowed weekdays
// from the assignment and the room capacity from the classroom, then returns the list of
// broken rules. Shared by the compute and the publish action.
func conflictsFor(ctx context.Context, rec map[string]interface{}) ([]string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	cand := entryViewFrom(rec)

	rows, err := orm.Search(bypass, entryModel, [][]interface{}{
		{"weekday", "=", cand.Weekday},
	})
	if err != nil {
		return nil, err
	}
	existing := make([]entryView, 0, len(rows))
	for _, r := range rows {
		if strOr(r["state"]) == string(TtCancelled) {
			continue
		}
		existing = append(existing, entryViewFrom(r))
	}

	allowedDays := teacherAllowedDays(bypass, intOr(rec["assignment_id"]))
	roomCap := roomCapacity(bypass, cand.RoomID)
	return findConflicts(cand, existing, allowedDays, roomCap), nil
}

func entryViewFrom(rec map[string]interface{}) entryView {
	dur := intOr(rec["duration"])
	if dur <= 0 {
		dur = 1
	}
	return entryView{
		ID:           intOr(rec["id"]),
		TeacherID:    intOr(rec["teacher_id"]),
		SectionID:    intOr(rec["section_id"]),
		RoomID:       intOr(rec["classroom_id"]),
		Weekday:      intOr(rec["weekday"]),
		Start:        intOr(rec["period_number"]),
		Duration:     dur,
		StudentCount: intOr(rec["student_count"]),
	}
}

func teacherAllowedDays(ctx context.Context, assignmentID int) []int {
	if assignmentID <= 0 {
		return nil
	}
	assign, err := orm.SearchOne(ctx, assignmentModel, map[string]interface{}{"id": assignmentID})
	if err != nil || assign == nil {
		return nil
	}
	return parseIntList(strOr(assign["allowed_weekdays"]))
}

func roomCapacity(ctx context.Context, roomID int) int {
	if roomID <= 0 {
		return 0
	}
	room, err := orm.SearchOne(ctx, classroomModel, map[string]interface{}{"id": roomID})
	if err != nil || room == nil {
		return 0
	}
	return intOr(room["capacity"])
}

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func strOr(v interface{}) string {
	return orm.AsString(v)
}

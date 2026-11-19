package models

import (
	"context"
	"errors"
	"strings"
	"time"

	"nexa/core/orm"
)

const leadModel = "crm.lead"

func init() {
	orm.RegisterCompute(leadModel, "days_in_stage", []string{"date_last_stage_update"}, computeLeadDaysInStage)
}

// computeLeadDaysInStage reports how long the deal has sat in its current
// stage — the basis for stage-aging highlights on the pipeline board.
func computeLeadDaysInStage(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	raw := rec["date_last_stage_update"]
	if raw == nil || raw == false || raw == "" {
		return 0, nil
	}
	ts, err := parseStageTimestamp(raw)
	if err != nil {
		return 0, nil
	}
	days := int(time.Since(ts).Hours() / 24)
	if days < 0 {
		return 0, nil
	}
	return days, nil
}

func parseStageTimestamp(v interface{}) (time.Time, error) {
	s := strings.TrimSpace(orm.AsString(v))
	if s == "" {
		return time.Time{}, errors.New("empty timestamp")
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unrecognized timestamp")
}

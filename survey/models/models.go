package models

import (
	"nexa/core/sdk"
)

type SurveyState string

const (
	SurveyStateDraft  SurveyState = "draft"
	SurveyStateOpen   SurveyState = "open"
	SurveyStateClosed SurveyState = "closed"
)

type QuestionType string

const (
	QuestionTypeText   QuestionType = "text"
	QuestionTypeChoice QuestionType = "choice"
	QuestionTypeRating QuestionType = "rating"
)

type SurveySurvey struct {
	sdk.Model `nexa:"model=survey.survey"`

	Name        sdk.String                 `nexa:"required,string=Survey Title"`
	Description sdk.Text                   `nexa:"string=Description"`
	State       sdk.Selection[SurveyState] `nexa:"string=Status,default=draft"`
	Active      sdk.Boolean                `nexa:"string=Active,default=true"`
}

type SurveyQuestion struct {
	sdk.Model `nexa:"model=survey.question"`

	Name         sdk.String                  `nexa:"required,string=Question"`
	SurveyID     sdk.Many2One[SurveySurvey]  `nexa:"required,string=Survey"`
	QuestionType sdk.Selection[QuestionType] `nexa:"string=Type,default=text"`
	Sequence     sdk.Integer                 `nexa:"string=Sequence,default=0"`
	Help         sdk.Text                    `nexa:"string=Help"`
}

func init() {
	sdk.MustRegister("survey",
		&SurveySurvey{},
		&SurveyQuestion{},
	)
}

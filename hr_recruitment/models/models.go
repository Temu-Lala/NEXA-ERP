package models

import (
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
)

// Cross-module reference aliases.
type (
	HrEmployee   = hrmodels.HrEmployee
	HrDepartment = hrmodels.HrDepartment
	HrJob        = hrmodels.HrJob
	HrApplicant  = hrmodels.HrApplicant
)

type OpeningState string

const (
	OpeningStateDraft    OpeningState = "draft"
	OpeningStateApproved OpeningState = "approved"
	OpeningStateOpen     OpeningState = "open"
	OpeningStateDone     OpeningState = "done"
	OpeningStateClosed   OpeningState = "closed"
	OpeningStateRefused  OpeningState = "refused"
)

type InterviewRating string

const (
	InterviewRatingBad       InterviewRating = "bad"
	InterviewRatingAverage   InterviewRating = "average"
	InterviewRatingGood      InterviewRating = "good"
	InterviewRatingExcellent InterviewRating = "excellent"
)

type InterviewState string

const (
	InterviewStatePending   InterviewState = "pending"
	InterviewStateScheduled InterviewState = "scheduled"
	InterviewStatePassed    InterviewState = "passed"
	InterviewStateFailed    InterviewState = "failed"
)

type OfferState string

const (
	OfferStateDraft    OfferState = "draft"
	OfferStateSent     OfferState = "sent"
	OfferStateAccepted OfferState = "accepted"
	OfferStateRefused  OfferState = "refused"
)

type HrJobOpening struct {
	sdk.Model `nexa:"model=hr.job.opening"`

	Name              sdk.String                  `nexa:"required,string=Position"`
	JobID             sdk.Many2One[HrJob]         `nexa:"string=Job"`
	DepartmentID      sdk.Many2One[HrDepartment]  `nexa:"string=Department"`
	RecruiterID       sdk.Many2One[HrEmployee]    `nexa:"string=Recruiter"`
	ExpectedDate      sdk.Date                    `nexa:"string=Expected Date"`
	NumberOfPositions sdk.Integer                 `nexa:"string=Number of Positions,default=1"`
	Description       sdk.Text                    `nexa:"string=Job Description"`
	Note              sdk.Text                    `nexa:"string=Internal Notes"`
	State             sdk.Selection[OpeningState] `nexa:"string=Status,default=draft"`
	Active            sdk.Boolean                 `nexa:"string=Active,default=true"`
}

type HrRecruitmentInterview struct {
	sdk.Model `nexa:"model=hr.recruitment.interview"`

	ApplicantID   sdk.Many2One[HrApplicant]      `nexa:"required,string=Applicant"`
	OpeningID     sdk.Many2One[HrJobOpening]     `nexa:"string=Job Opening"`
	InterviewerID sdk.Many2One[HrEmployee]       `nexa:"string=Interviewer"`
	InterviewDate sdk.DateTime                   `nexa:"string=Interview Date"`
	Location      sdk.String                     `nexa:"string=Location"`
	Rating        sdk.Selection[InterviewRating] `nexa:"string=Rating,default=average"`
	State         sdk.Selection[InterviewState]  `nexa:"string=Status,default=pending"`
	Note          sdk.Text                       `nexa:"string=Feedback"`
}

type HrRecruitmentOffer struct {
	sdk.Model `nexa:"model=hr.recruitment.offer"`

	ApplicantID   sdk.Many2One[HrApplicant]  `nexa:"required,string=Applicant"`
	OpeningID     sdk.Many2One[HrJobOpening] `nexa:"string=Job Opening"`
	OfferDate     sdk.Date                   `nexa:"string=Offer Date"`
	ExpectedStart sdk.Date                   `nexa:"string=Expected Start Date"`
	Salary        sdk.Numeric                `nexa:"string=Proposed Salary,precision=18,scale=2,default=0"`
	State         sdk.Selection[OfferState]  `nexa:"string=Status,default=draft"`
	EmployeeID    sdk.Many2One[HrEmployee]   `nexa:"string=Created Employee"`
	Note          sdk.Text                   `nexa:"string=Offer Content"`
}

func init() {
	sdk.MustRegister("hr_recruitment",
		&HrJobOpening{},
		&HrRecruitmentInterview{},
		&HrRecruitmentOffer{},
	)
}

package models

import (
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
)

// Cross-module reference aliases.
type (
	HrEmployee = hrmodels.HrEmployee
)

type CourseType string

const (
	CourseTypeClassroom CourseType = "classroom"
	CourseTypeVirtual   CourseType = "virtual"
	CourseTypeSelfPaced CourseType = "self_paced"
	CourseTypeBlended   CourseType = "blended"
	CourseTypeOnTheJob  CourseType = "on_the_job"
)

type EnrollmentState string

const (
	EnrollmentStateDraft      EnrollmentState = "draft"
	EnrollmentStateConfirmed  EnrollmentState = "confirmed"
	EnrollmentStateInProgress EnrollmentState = "in_progress"
	EnrollmentStateCompleted  EnrollmentState = "completed"
	EnrollmentStateCancelled  EnrollmentState = "cancelled"
)

type SessionState string

const (
	SessionStatePlanned    SessionState = "planned"
	SessionStateOpen       SessionState = "open"
	SessionStateInProgress SessionState = "in_progress"
	SessionStateDone       SessionState = "done"
	SessionStateCancelled  SessionState = "cancelled"
)

// HrTrainingCourse is a catalog item for training programs.
type HrTrainingCourse struct {
	sdk.Model `nexa:"model=hr.training.course"`

	Name          sdk.String                `nexa:"required,string=Course"`
	Code          sdk.String                `nexa:"string=Code"`
	CourseType    sdk.Selection[CourseType] `nexa:"string=Type,default=classroom"`
	Instructor    sdk.String                `nexa:"string=Instructor"`
	Duration      sdk.Numeric               `nexa:"string=Duration (Hours),precision=18,scale=2,default=0"`
	Cost          sdk.Numeric               `nexa:"string=Cost,precision=18,scale=2,default=0"`
	Description   sdk.Text                  `nexa:"string=Description"`
	Prerequisites sdk.Text                  `nexa:"string=Prerequisites"`
	Mandatory     sdk.Boolean               `nexa:"string=Mandatory,default=false"`
	Active        sdk.Boolean               `nexa:"string=Active,default=true"`
}

// HrTrainingSession is a scheduled delivery of a course.
type HrTrainingSession struct {
	sdk.Model `nexa:"model=hr.training.session"`

	CourseID  sdk.Many2One[HrTrainingCourse] `nexa:"required,string=Course"`
	Name      sdk.String                     `nexa:"string=Session Name"`
	DateStart sdk.DateTime                   `nexa:"required,string=Start"`
	DateEnd   sdk.DateTime                   `nexa:"required,string=End"`
	Location  sdk.String                     `nexa:"string=Location"`
	Capacity  sdk.Integer                    `nexa:"string=Capacity,default=20"`
	State     sdk.Selection[SessionState]    `nexa:"string=Status,default=planned"`
}

// HrTrainingEnrollment records an employee's participation in a session.
type HrTrainingEnrollment struct {
	sdk.Model `nexa:"model=hr.training.enrollment"`

	SessionID      sdk.Many2One[HrTrainingSession] `nexa:"required,string=Session"`
	EmployeeID     sdk.Many2One[HrEmployee]        `nexa:"required,string=Employee"`
	EnrollmentDate sdk.Date                        `nexa:"string=Enrollment Date"`
	CompletionDate sdk.Date                        `nexa:"string=Completion Date"`
	Score          sdk.Numeric                     `nexa:"string=Score,precision=18,scale=2,default=0"`
	State          sdk.Selection[EnrollmentState]  `nexa:"string=Status,default=draft"`
	Notes          sdk.Text                        `nexa:"string=Notes"`
}

// HrTrainingCertificate records a certification earned by an employee.
type HrTrainingCertificate struct {
	sdk.Model `nexa:"model=hr.training.certificate"`

	EmployeeID sdk.Many2One[HrEmployee]       `nexa:"required,string=Employee"`
	CourseID   sdk.Many2One[HrTrainingCourse] `nexa:"string=Course"`
	Name       sdk.String                     `nexa:"required,string=Certificate"`
	IssueDate  sdk.Date                       `nexa:"string=Issue Date"`
	ExpiryDate sdk.Date                       `nexa:"string=Expiry Date"`
	Issuer     sdk.String                     `nexa:"string=Issuing Body"`
	Notes      sdk.Text                       `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("hr_training",
		&HrTrainingCourse{},
		&HrTrainingSession{},
		&HrTrainingEnrollment{},
		&HrTrainingCertificate{},
	)
}

package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	guardiansmodels "nexa_addons/school_guardians/models"
	staffmodels "nexa_addons/school_staff/models"
)

// School Portal (blueprint self-service section). This is the parent and student
// self-service surface layered on top of the school records, with ownership isolation
// as its backbone. A school.portal.profile binds one authenticated portal USER to the
// person records they own: a parent/guardian profile points at a school.guardian, a
// student profile points at their own school.student, a staff profile at a
// school.staff.member. The set of learners a profile may touch is DERIVED, never typed:
// a learner is in scope if it is the profile's own student record or is linked to the
// profile's guardian through an active school.student.guardian relationship. Financial
// visibility is an explicit opt-in that mirrors the guardian link's can_view_financial
// flag, so "which parent can see the fees" stays governed by the guardians module.
//
// On top of that spine sit the surfaces themselves: a catalog of school.portal.widget
// dashboard tiles targeted per role (optionally gated on financial visibility), a
// school.portal.request self-service workflow (absence, late arrival, early dismissal,
// document, fee plan, academic advisory, general enquiry) with an enforced draft ->
// submitted -> under review -> approved/rejected -> fulfilled state machine, an
// append-only school.portal.request.history trail that records every status move and can
// never be edited, and an immutable school.portal.access.log audit ledger that captures
// who reached which learner record through the portal. The isolation predicate, the
// request state machine and the age/visibility arithmetic are pure and unit-tested.
// Navigated under the single School app.

type (
	CoreUser             = basemodels.CoreUser
	CoreCompany          = basemodels.CoreCompany
	SchoolStudent        = schoolmodels.SchoolStudent
	SchoolGuardian       = guardiansmodels.SchoolGuardian
	SchoolStudentGuardia = guardiansmodels.SchoolStudentGuardian
	SchoolStaffMember    = staffmodels.SchoolStaffMember
)

// PortalRole is the capacity in which a portal user signs in.
type PortalRole string

const (
	RoleParent  PortalRole = "parent"
	RoleStudent PortalRole = "student"
	RoleStaff   PortalRole = "staff"
)

// RequestCategory classifies a self-service request.
type RequestCategory string

const (
	CatAbsence        RequestCategory = "absence"
	CatLateArrival    RequestCategory = "late_arrival"
	CatEarlyDismissal RequestCategory = "early_dismissal"
	CatDocument       RequestCategory = "document"
	CatFeePlan        RequestCategory = "fee_plan"
	CatAdvisory       RequestCategory = "academic_advisory"
	CatInquiry        RequestCategory = "inquiry"
	CatOther          RequestCategory = "other"
)

// RequestState is the self-service request lifecycle.
type RequestState string

const (
	ReqDraft       RequestState = "draft"
	ReqSubmitted   RequestState = "submitted"
	ReqUnderReview RequestState = "under_review"
	ReqApproved    RequestState = "approved"
	ReqRejected    RequestState = "rejected"
	ReqFulfilled   RequestState = "fulfilled"
	ReqCancelled   RequestState = "cancelled"
)

// AccessAction is the kind of portal data touch recorded in the audit log.
type AccessAction string

const (
	ActView     AccessAction = "view"
	ActUpdate   AccessAction = "update"
	ActDownload AccessAction = "download"
)

// SchoolPortalProfile is the ownership/isolation spine: one portal user, the person
// records they own and the flags that govern what their self-service surface exposes.
type SchoolPortalProfile struct {
	sdk.Model `nexa:"model=school.portal.profile"`

	Name   sdk.String                `nexa:"required,size=120,string=Display Name"`
	Code   sdk.String                `nexa:"index,size=40,string=Code"`
	UserID sdk.Many2One[CoreUser]    `nexa:"required,index,string=Portal User"`
	Role   sdk.Selection[PortalRole] `nexa:"string=Portal Role,default=parent,index,selection=parent:Parent / Guardian,student:Student,staff:Staff"`

	GuardianID sdk.Many2One[SchoolGuardian]    `nexa:"index,string=Guardian Record"`
	StudentID  sdk.Many2One[SchoolStudent]     `nexa:"index,string=Student Record"`
	StaffID    sdk.Many2One[SchoolStaffMember] `nexa:"index,string=Staff Record"`

	DefaultStudentID sdk.Many2One[SchoolStudent] `nexa:"index,string=Active Learner"`

	CanViewFinancial  sdk.Boolean `nexa:"string=Can View Financial,default=false"`
	CanSubmitRequests sdk.Boolean `nexa:"string=Can Submit Requests,default=true"`

	LastAccess sdk.DateTime `nexa:"string=Last Access"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	AccessibleLearnerCount sdk.Integer `nexa:"string=Learners In Scope,compute=compute_profile_learner_count"`
	RoleLabel              sdk.String  `nexa:"string=Role,compute=compute_profile_role_label"`
}

// SchoolPortalWidget is a dashboard tile surfaced to one or more portal roles.
type SchoolPortalWidget struct {
	sdk.Model `nexa:"model=school.portal.widget"`

	Code        sdk.String  `nexa:"required,index,size=60,string=Code"`
	Name        sdk.String  `nexa:"required,size=120,string=Name"`
	Description sdk.Text    `nexa:"string=Description"`
	Sequence    sdk.Integer `nexa:"string=Sequence,default=10"`

	ShowForParent  sdk.Boolean `nexa:"string=Show To Parents,default=true"`
	ShowForStudent sdk.Boolean `nexa:"string=Show To Students,default=true"`
	ShowForStaff   sdk.Boolean `nexa:"string=Show To Staff,default=false"`

	LinkModel sdk.String  `nexa:"size=80,string=Data Model"`
	Icon      sdk.String  `nexa:"size=40,string=Icon"`
	Financial sdk.Boolean `nexa:"string=Requires Financial Access,default=false"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

// SchoolPortalRequest is a self-service request raised through the portal.
type SchoolPortalRequest struct {
	sdk.Model `nexa:"model=school.portal.request"`

	Reference sdk.String `nexa:"index,size=40,string=Reference"`

	Category   sdk.Selection[RequestCategory]    `nexa:"string=Category,default=inquiry,index,selection=absence:Absence,late_arrival:Late Arrival,early_dismissal:Early Dismissal,document:Document Request,fee_plan:Fee Plan,academic_advisory:Academic Advisory,inquiry:General Enquiry,other:Other"`
	ProfileID  sdk.Many2One[SchoolPortalProfile] `nexa:"index,string=Submitted By"`
	StudentID  sdk.Many2One[SchoolStudent]       `nexa:"required,index,string=Student"`
	GuardianID sdk.Many2One[SchoolGuardian]      `nexa:"index,string=Guardian"`

	Reason  sdk.Text `nexa:"string=Reason"`
	Details sdk.Text `nexa:"string=Details"`

	DateFrom sdk.Date `nexa:"string=Date From"`
	DateTo   sdk.Date `nexa:"string=Date To"`

	State          sdk.Selection[RequestState] `nexa:"string=Status,default=draft,index,selection=draft:Draft,submitted:Submitted,under_review:Under Review,approved:Approved,rejected:Rejected,fulfilled:Fulfilled,cancelled:Cancelled"`
	SubmittedDate  sdk.Date                    `nexa:"string=Submitted Date"`
	ResolvedDate   sdk.Date                    `nexa:"string=Resolved Date"`
	ResolutionNote sdk.Text                    `nexa:"string=Resolution Note"`
	RejectReason   sdk.Text                    `nexa:"string=Rejection Reason"`

	Histories sdk.One2Many[SchoolPortalRequestHistory] `nexa:"string=History,inverse=request_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`

	AgeDays    sdk.Integer `nexa:"string=Age (Days),compute=compute_request_age_days"`
	Actionable sdk.Boolean `nexa:"string=Needs Action,compute=compute_request_actionable"`
}

// SchoolPortalRequestHistory is the append-only trail of request status moves. Rows are
// written by the engine and are never edited or deleted.
type SchoolPortalRequestHistory struct {
	sdk.Model `nexa:"model=school.portal.request.history"`

	RequestID sdk.Many2One[SchoolPortalRequest] `nexa:"required,index,string=Request"`
	FromState sdk.String                        `nexa:"size=30,string=From"`
	ToState   sdk.String                        `nexa:"size=30,index,string=To"`
	Action    sdk.String                        `nexa:"size=60,string=Action"`
	ActorID   sdk.Integer                       `nexa:"string=Actor"`
	Note      sdk.Text                          `nexa:"string=Note"`
	Timestamp sdk.DateTime                      `nexa:"string=Timestamp"`
}

// SchoolPortalAccessLog is the immutable audit of portal data access for ownership-
// isolation compliance. Rows are written by the engine and are never edited or deleted.
type SchoolPortalAccessLog struct {
	sdk.Model `nexa:"model=school.portal.access.log"`

	ProfileID   sdk.Many2One[SchoolPortalProfile] `nexa:"required,index,string=Profile"`
	UserID      sdk.Many2One[CoreUser]            `nexa:"index,string=Portal User"`
	StudentID   sdk.Many2One[SchoolStudent]       `nexa:"index,string=Student"`
	TargetModel sdk.String                        `nexa:"size=80,index,string=Data Model"`
	RecordID    sdk.Integer                       `nexa:"string=Record"`
	Action      sdk.Selection[AccessAction]       `nexa:"string=Action,default=view,index,selection=view:View,update:Update,download:Download"`
	Financial   sdk.Boolean                       `nexa:"string=Financial,default=false"`
	AccessedAt  sdk.DateTime                      `nexa:"string=Accessed"`
	RemoteAddr  sdk.String                        `nexa:"size=60,string=Remote Address"`
}

func init() {
	sdk.MustRegister("school_portal",
		&SchoolPortalProfile{},
		&SchoolPortalWidget{},
		&SchoolPortalRequest{},
		&SchoolPortalRequestHistory{},
		&SchoolPortalAccessLog{},
	)
}

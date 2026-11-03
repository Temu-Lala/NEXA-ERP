package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	setupmodels "nexa_addons/school_setup/models"
)

// School Governance (blueprint governance section). This companion centralises how the
// school is RUN rather than what it records: governed POLICIES for grading, admission,
// fees, attendance and academics — each an effective-dated, versioned configuration whose
// concrete knobs are typed key/value PARAMETERS (pass mark, minimum age, fee due days,
// attendance threshold …); a native APPROVAL workflow with an append-only DECISION log and
// strict separation of duties (a requester may never approve their own submission) that
// gates a policy going live; ACCESS-ROLE mappings that bind a security group to a governed
// area and an approval capability; and INTEGRATION connectors describing external systems
// (SIS, payment, email, SMS, ERP …) by endpoint and a secret REFERENCE only — never the
// secret itself. The effectiveness test, the approval state machine, the separation-of-
// duties predicate, the sync-eligibility gate and the expiry/version arithmetic are pure
// and unit-tested; a daily cron archives policies whose effective window has closed. It
// reuses campuses and classes from the school data model. Navigated under the School app.

type (
	CoreUser     = basemodels.CoreUser
	CoreCompany  = basemodels.CoreCompany
	SchoolClass  = schoolmodels.SchoolClass
	SchoolCampus = setupmodels.SchoolCampus
)

// PolicyCategory is the domain a governance policy governs.
type PolicyCategory string

const (
	PolGrading    PolicyCategory = "grading"
	PolAdmission  PolicyCategory = "admission"
	PolFee        PolicyCategory = "fee"
	PolAttendance PolicyCategory = "attendance"
	PolAcademic   PolicyCategory = "academic"
	PolAssessment PolicyCategory = "assessment"
	PolOther      PolicyCategory = "other"
)

// PolicyState is the governance lifecycle of a policy.
type PolicyState string

const (
	PstDraft    PolicyState = "draft"
	PstInReview PolicyState = "in_review"
	PstActive   PolicyState = "active"
	PstArchived PolicyState = "archived"
)

// ParamType is the data type of a policy parameter value.
type ParamType string

const (
	PTInteger ParamType = "integer"
	PTFloat   ParamType = "float"
	PTBoolean ParamType = "boolean"
	PTString  ParamType = "string"
	PTDate    ParamType = "date"
	PTJson    ParamType = "json"
)

// ApprovalState is the review lifecycle of a policy change request.
type ApprovalState string

const (
	AprPending   ApprovalState = "pending"
	AprApproved  ApprovalState = "approved"
	AprRejected  ApprovalState = "rejected"
	AprCancelled ApprovalState = "cancelled"
)

// ApprovalAction is a recorded step in the decision trail.
type ApprovalAction string

const (
	AadSubmit  ApprovalAction = "submit"
	AadApprove ApprovalAction = "approve"
	AadReject  ApprovalAction = "reject"
	AadCancel  ApprovalAction = "cancel"
)

// AccessScope is the permission level an access role grants.
type AccessScope string

const (
	ScopeRead    AccessScope = "read"
	ScopeWrite   AccessScope = "write"
	ScopeApprove AccessScope = "approve"
	ScopeAdmin   AccessScope = "admin"
)

// IntegrationSystem is the kind of external system a connector targets.
type IntegrationSystem string

const (
	IntSIS      IntegrationSystem = "sis"
	IntPayment  IntegrationSystem = "payment"
	IntEmail    IntegrationSystem = "email"
	IntSMS      IntegrationSystem = "sms"
	IntERP      IntegrationSystem = "erp"
	IntCalendar IntegrationSystem = "calendar"
	IntOther    IntegrationSystem = "other"
)

// IntegrationState is the operational status of a connector.
type IntegrationState string

const (
	IstDraft  IntegrationState = "draft"
	IstActive IntegrationState = "active"
	IstPaused IntegrationState = "paused"
	IstError  IntegrationState = "error"
)

// SchoolGovernancePolicy is an effective-dated, versioned configuration for one domain.
type SchoolGovernancePolicy struct {
	sdk.Model `nexa:"model=school.governance.policy"`

	Code        sdk.String                    `nexa:"required,index,size=60,string=Code"`
	Name        sdk.String                    `nexa:"required,size=140,string=Name"`
	Category    sdk.Selection[PolicyCategory] `nexa:"string=Category,default=academic,index,selection=grading:Grading,admission:Admission,fee:Fees,attendance:Attendance,academic:Academic,assessment:Assessment,other:Other"`
	Description sdk.Text                      `nexa:"string=Description"`
	Version     sdk.Integer                   `nexa:"string=Version,default=1"`
	State       sdk.Selection[PolicyState]    `nexa:"string=Status,default=draft,index,selection=draft:Draft,in_review:In Review,active:Active,archived:Archived"`

	EffectiveFrom sdk.Date `nexa:"index,string=Effective From"`
	EffectiveTo   sdk.Date `nexa:"string=Effective To"`

	CampusID sdk.Many2One[SchoolCampus] `nexa:"index,string=Campus"`
	ClassID  sdk.Many2One[SchoolClass]  `nexa:"index,string=Class"`

	ApprovalRequired   sdk.Boolean `nexa:"string=Approval Required,default=true"`
	ApproverGroupXmlid sdk.String  `nexa:"size=120,string=Approver Group"`

	Parameters sdk.One2Many[SchoolGovernancePolicyParameter] `nexa:"string=Parameters,inverse=policy_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	IsEffective     sdk.Boolean `nexa:"string=Effective Now,compute=compute_policy_effective"`
	ParameterCount  sdk.Integer `nexa:"string=Parameters,compute=compute_policy_parameter_count"`
	PendingApproval sdk.Boolean `nexa:"string=Pending Approval,compute=compute_policy_pending_approval"`
}

// SchoolGovernancePolicyParameter is a typed configuration knob on a policy.
type SchoolGovernancePolicyParameter struct {
	sdk.Model `nexa:"model=school.governance.policy.parameter"`

	PolicyID    sdk.Many2One[SchoolGovernancePolicy] `nexa:"required,index,string=Policy"`
	Name        sdk.String                           `nexa:"required,size=120,string=Name"`
	Value       sdk.String                           `nexa:"size=240,string=Value"`
	ParamType   sdk.Selection[ParamType]             `nexa:"string=Type,default=string,index,selection=integer:Integer,float:Float,boolean:Boolean,string:String,date:Date,json:JSON"`
	Sequence    sdk.Integer                          `nexa:"string=Sequence,default=10"`
	Description sdk.Text                             `nexa:"string=Description"`
}

// SchoolGovernanceApproval is a request to bring a policy change into force.
type SchoolGovernanceApproval struct {
	sdk.Model `nexa:"model=school.governance.approval"`

	Code        sdk.String                           `nexa:"index,size=40,string=Reference"`
	PolicyID    sdk.Many2One[SchoolGovernancePolicy] `nexa:"required,index,string=Policy"`
	Action      sdk.String                           `nexa:"size=60,string=Requested Action"`
	RequestedBy sdk.Integer                          `nexa:"string=Requested By"`
	Reason      sdk.Text                             `nexa:"string=Reason"`
	State       sdk.Selection[ApprovalState]         `nexa:"string=Status,default=pending,index,selection=pending:Pending,approved:Approved,rejected:Rejected,cancelled:Cancelled"`

	SubmittedDate sdk.Date    `nexa:"string=Submitted"`
	DecidedBy     sdk.Integer `nexa:"string=Decided By"`
	DecidedDate   sdk.Date    `nexa:"string=Decided"`
	DecisionNote  sdk.Text    `nexa:"string=Decision Note"`

	Decisions sdk.One2Many[SchoolGovernanceApprovalDecision] `nexa:"string=Decisions,inverse=approval_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`

	CanApprove sdk.Boolean `nexa:"string=You Can Approve,compute=compute_approval_can_approve"`
}

// SchoolGovernanceApprovalDecision is the append-only trail of an approval's moves.
type SchoolGovernanceApprovalDecision struct {
	sdk.Model `nexa:"model=school.governance.approval.decision"`

	ApprovalID sdk.Many2One[SchoolGovernanceApproval] `nexa:"required,index,string=Approval"`
	Action     sdk.Selection[ApprovalAction]          `nexa:"string=Action,index,selection=submit:Submit,approve:Approve,reject:Reject,cancel:Cancel"`
	ActorID    sdk.Integer                            `nexa:"string=Actor"`
	Note       sdk.Text                               `nexa:"string=Note"`
	Timestamp  sdk.DateTime                           `nexa:"string=Timestamp"`
}

// SchoolGovernanceAccessRole binds a security group to a governed area and capability.
type SchoolGovernanceAccessRole struct {
	sdk.Model `nexa:"model=school.governance.access.role"`

	Name         sdk.String                    `nexa:"required,size=120,string=Name"`
	Code         sdk.String                    `nexa:"index,size=60,string=Code"`
	GroupXmlid   sdk.String                    `nexa:"size=120,string=Security Group"`
	Scope        sdk.Selection[AccessScope]    `nexa:"string=Scope,default=read,index,selection=read:Read,write:Write,approve:Approve,admin:Admin"`
	GovernedArea sdk.Selection[PolicyCategory] `nexa:"string=Governed Area,default=other,index,selection=grading:Grading,admission:Admission,fee:Fees,attendance:Attendance,academic:Academic,assessment:Assessment,other:Other"`
	CanApprove   sdk.Boolean                   `nexa:"string=Can Approve,default=false"`
	Description  sdk.Text                      `nexa:"string=Description"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

// SchoolGovernanceIntegration describes an external system connector (never stores the
// secret, only a reference to where it is kept).
type SchoolGovernanceIntegration struct {
	sdk.Model `nexa:"model=school.governance.integration"`

	Name                sdk.String                       `nexa:"required,size=120,string=Name"`
	Code                sdk.String                       `nexa:"index,size=60,string=Code"`
	SystemType          sdk.Selection[IntegrationSystem] `nexa:"string=System,default=other,index,selection=sis:SIS,payment:Payment,email:Email,sms:SMS,erp:ERP,calendar:Calendar,other:Other"`
	Endpoint            sdk.String                       `nexa:"size=240,string=Endpoint"`
	ApiUser             sdk.String                       `nexa:"size=120,string=API User"`
	SecretRef           sdk.String                       `nexa:"size=120,string=Secret Reference"`
	Config              sdk.Text                         `nexa:"string=Configuration"`
	SyncIntervalMinutes sdk.Integer                      `nexa:"string=Sync Interval (min),default=60"`
	State               sdk.Selection[IntegrationState]  `nexa:"string=Status,default=draft,index,selection=draft:Draft,active:Active,paused:Paused,error:Error"`
	LastSync            sdk.DateTime                     `nexa:"string=Last Sync"`
	LastError           sdk.Text                         `nexa:"string=Last Error"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	DaysSinceSync sdk.Integer `nexa:"string=Days Since Sync,compute=compute_integration_days_since_sync"`
}

func init() {
	sdk.MustRegister("school_governance",
		&SchoolGovernancePolicy{},
		&SchoolGovernancePolicyParameter{},
		&SchoolGovernanceApproval{},
		&SchoolGovernanceApprovalDecision{},
		&SchoolGovernanceAccessRole{},
		&SchoolGovernanceIntegration{},
	)
}

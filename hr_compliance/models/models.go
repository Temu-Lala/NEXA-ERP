package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
)

// HR-F10 "Relations, Compliance & Offboarding". It adds the governance and
// exit-side capabilities the anchor modules lack, without migrating any of their
// tables and without re-creating what already exists (documents/expiry live in
// hr + hr_people, self-service surfaces + case SLA live in hr_selfservice). Here
// we model: policy publication with per-employee acknowledgement, confidential
// grievance case management, disciplinary records, workplace incidents and
// investigations, a compliance matrix that detects policy/training/certification
// gaps, resignation & termination workflows (notice period, handover/offboarding
// checklist, IT-access revocation, final-leave capture, record retention), and a
// medically-isolated record set. Sensitive records (grievance, disciplinary,
// medical) are gated to restricted groups through the ACL CSV and a read-only
// ownership rule on acknowledgements. Lifecycle / payroll numbers are only ever
// captured as decisions and references — nothing here silently posts a payment or
// edits pay data; the employee's status is changed only when an APPROVED
// termination is explicitly completed.

type (
	HrEmployee   = hrmodels.HrEmployee
	HrDepartment = hrmodels.HrDepartment
	HrJob        = hrmodels.HrJob
	CoreUser     = basemodels.CoreUser
	CoreCompany  = basemodels.CoreCompany
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// PolicyCategory buckets a policy for search and default acknowledgement scope.
type PolicyCategory string

const (
	PolicyConduct    PolicyCategory = "code_of_conduct"
	PolicySafety     PolicyCategory = "safety"
	PolicyHarassment PolicyCategory = "harassment"
	PolicySecurity   PolicyCategory = "security"
	PolicyLeave      PolicyCategory = "leave"
	PolicyPayroll    PolicyCategory = "payroll"
	PolicyIT         PolicyCategory = "it"
	PolicyOther      PolicyCategory = "other"
)

// PolicyState is the publication lifecycle of a policy.
type PolicyState string

const (
	PolicyDraft     PolicyState = "draft"
	PolicyPublished PolicyState = "published"
	PolicyArchived  PolicyState = "archived"
)

// AckState tracks an individual employee's response to a published policy.
type AckState string

const (
	AckPending      AckState = "pending"
	AckAcknowledged AckState = "acknowledged"
	AckDeclined     AckState = "declined"
)

// GrievanceCategory classifies a complaint.
type GrievanceCategory string

const (
	GrvHarassment  GrievanceCategory = "harassment"
	GrvDiscrim     GrievanceCategory = "discrimination"
	GrvRetaliation GrievanceCategory = "retaliation"
	GrvConditions  GrievanceCategory = "working_conditions"
	GrvPay         GrievanceCategory = "pay"
	GrvOther       GrievanceCategory = "other"
)

// CaseState is shared by grievance and incident case flows.
type CaseState string

const (
	CaseDraft         CaseState = "draft"
	CaseSubmitted     CaseState = "submitted"
	CaseInvestigating CaseState = "investigating"
	CaseResolved      CaseState = "resolved"
	CaseClosed        CaseState = "closed"
	CaseCancelled     CaseState = "cancelled"
)

// Severity is reused across grievances, discipline and incidents.
type Severity string

const (
	SevLow      Severity = "low"
	SevMedium   Severity = "medium"
	SevHigh     Severity = "high"
	SevCritical Severity = "critical"
)

// DisciplinaryType is the measure recorded against an employee.
type DisciplinaryType string

const (
	DiscVerbal      DisciplinaryType = "verbal_warning"
	DiscWritten     DisciplinaryType = "written_warning"
	DiscSuspension  DisciplinaryType = "suspension"
	DiscDemotion    DisciplinaryType = "demotion"
	DiscTermination DisciplinaryType = "termination"
	DiscOther       DisciplinaryType = "other"
)

// DisciplinaryState is the disciplinary-case lifecycle.
type DisciplinaryState string

const (
	DiscDraft    DisciplinaryState = "draft"
	DiscIssued   DisciplinaryState = "issued"
	DiscAppealed DisciplinaryState = "appealed"
	DiscUpheld   DisciplinaryState = "upheld"
	DiscClosed   DisciplinaryState = "closed"
)

// IncidentCategory classifies a workplace incident.
type IncidentCategory string

const (
	IncInjury   IncidentCategory = "injury"
	IncProperty IncidentCategory = "property"
	IncSecurity IncidentCategory = "security"
	IncNearMiss IncidentCategory = "near_miss"
	IncOther    IncidentCategory = "other"
)

// ComplianceCategory is what an employee must satisfy.
type ComplianceCategory string

const (
	CompPolicy      ComplianceCategory = "policy"
	CompTraining    ComplianceCategory = "training"
	CompCertificate ComplianceCategory = "certificate"
	CompMedical     ComplianceCategory = "medical"
	CompDocument    ComplianceCategory = "document"
)

// ComplianceState is the per-employee compliance verdict.
type ComplianceState string

const (
	CompPending      ComplianceState = "pending"
	CompCompliant    ComplianceState = "compliant"
	CompNonCompliant ComplianceState = "non_compliant"
	CompExpired      ComplianceState = "expired"
)

// CompScope decides which employees a requirement applies to.
type CompScope string

const (
	ScopeAll        CompScope = "all"
	ScopeDepartment CompScope = "department"
	ScopeJob        CompScope = "job"
)

// TerminationType is the reason a separation is initiated.
type TerminationType string

const (
	TermResignation TerminationType = "resignation"
	TermTermination TerminationType = "termination"
	TermRetired     TerminationType = "retired"
	TermContractEnd TerminationType = "contract_end"
	TermAbandonment TerminationType = "abandonment"
	TermOther       TerminationType = "other"
)

// TerminationState is the offboarding workflow lifecycle.
type TerminationState string

const (
	TermDraft     TerminationState = "draft"
	TermSubmitted TerminationState = "submitted"
	TermApproved  TerminationState = "approved"
	TermCompleted TerminationState = "completed"
	TermCancelled TerminationState = "cancelled"
)

// OffboardCategory groups handover checklist items.
type OffboardCategory string

const (
	ObIT        OffboardCategory = "it_access"
	ObEquipment OffboardCategory = "equipment"
	ObFinance   OffboardCategory = "finance"
	ObManager   OffboardCategory = "manager_kt"
	ObEmployee  OffboardCategory = "employee"
	ObAdmin     OffboardCategory = "admin"
)

// MedicalRecordType categorises a (confidential) medical entry.
type MedicalRecordType string

const (
	MedGeneral      MedicalRecordType = "general"
	MedFitness      MedicalRecordType = "fitness"
	MedOccupational MedicalRecordType = "occupational"
	MedVaccination  MedicalRecordType = "vaccination"
	MedIncident     MedicalRecordType = "incident_related"
	MedOther        MedicalRecordType = "other"
)

// CaseHistoryAction labels an append-only audit entry.
type CaseHistoryAction string

const (
	HistCreated CaseHistoryAction = "created"
	HistState   CaseHistoryAction = "state"
	HistAssign  CaseHistoryAction = "assignment"
	HistAck     CaseHistoryAction = "acknowledgement"
	HistIssue   CaseHistoryAction = "issued"
	HistNote    CaseHistoryAction = "note"
)

// ---------------------------------------------------------------------------
// Policies & acknowledgement
// ---------------------------------------------------------------------------

// HrPolicy is a company policy document with an effective date and version.
type HrPolicy struct {
	sdk.Model `nexa:"model=hr.policy"`

	Name          sdk.String                    `nexa:"required,string=Title"`
	Code          sdk.String                    `nexa:"string=Code,index"`
	Category      sdk.Selection[PolicyCategory] `nexa:"string=Category,index,default=other,selection=code_of_conduct:Code of Conduct,safety:Safety,harassment:Harassment,security:Security,leave:Leave,payroll:Payroll,it:IT,other:Other"`
	Version       sdk.Integer                   `nexa:"string=Version,default=1"`
	EffectiveDate sdk.Date                      `nexa:"string=Effective Date,index"`
	Body          sdk.Text                      `nexa:"string=Content"`
	RequiresAck   sdk.Boolean                   `nexa:"string=Requires Acknowledgement,default=true"`
	State         sdk.Selection[PolicyState]    `nexa:"string=Status,index,default=draft,selection=draft:Draft,published:Published,archived:Archived"`
	Active        sdk.Boolean                   `nexa:"string=Active,default=true"`

	AckRequiredCount sdk.Integer `nexa:"string=Pending,compute=compute_policy_ack_pending"`
}

// HrPolicyAck is one employee's response to a published policy. The stored
// employee_user_id lets a read-only ownership rule scope an employee's own rows.
type HrPolicyAck struct {
	sdk.Model `nexa:"model=hr.policy.ack"`

	PolicyID       sdk.Many2One[HrPolicy]   `nexa:"required,string=Policy,index"`
	EmployeeID     sdk.Many2One[HrEmployee] `nexa:"required,string=Employee,index"`
	EmployeeUserID sdk.Integer              `nexa:"string=User,compute=compute_ack_employee_user,store"`
	State          sdk.Selection[AckState]  `nexa:"string=Status,index,default=pending,selection=pending:Pending,acknowledged:Acknowledged,declined:Declined"`
	AckDate        sdk.DateTime             `nexa:"string=Acknowledged On"`
	Note           sdk.Text                 `nexa:"string=Note"`
}

// ---------------------------------------------------------------------------
// Grievances (confidential case management) + audit
// ---------------------------------------------------------------------------

// HrGrievance is a confidential complaint case with an investigation workflow.
type HrGrievance struct {
	sdk.Model `nexa:"model=hr.grievance"`

	Name         sdk.String                       `nexa:"required,string=Subject"`
	Number       sdk.String                       `nexa:"string=Reference,index"`
	EmployeeID   sdk.Many2One[HrEmployee]         `nexa:"required,string=Complainant,index"`
	RespondentID sdk.Many2One[HrEmployee]         `nexa:"string=Respondent"`
	Category     sdk.Selection[GrievanceCategory] `nexa:"string=Category,index,default=other,selection=harassment:Harassment,discrimination:Discrimination,retaliation:Retaliation,working_conditions:Working Conditions,pay:Pay,other:Other"`
	Severity     sdk.Selection[Severity]          `nexa:"string=Severity,index,default=medium,selection=low:Low,medium:Medium,high:High,critical:Critical"`
	Description  sdk.Text                         `nexa:"string=Description"`
	State        sdk.Selection[CaseState]         `nexa:"string=Status,index,default=draft,selection=draft:Draft,submitted:Submitted,investigating:Investigating,resolved:Resolved,closed:Closed,cancelled:Cancelled"`
	AssignedID   sdk.Many2One[CoreUser]           `nexa:"string=Assigned To"`
	Confidential sdk.Boolean                      `nexa:"string=Confidential,default=true"`
	Resolution   sdk.Text                         `nexa:"string=Resolution"`
	ClosedDate   sdk.DateTime                     `nexa:"string=Closed On"`
	CreatedBy    sdk.Integer                      `nexa:"string=Raised By"`

	History sdk.One2Many[HrGrievanceHistory] `nexa:"string=History,inverse=grievance_id"`
}

// HrGrievanceHistory is an immutable audit trail for a grievance.
type HrGrievanceHistory struct {
	sdk.Model `nexa:"model=hr.grievance.history"`

	GrievanceID sdk.Many2One[HrGrievance]        `nexa:"required,string=Grievance,index"`
	Action      sdk.Selection[CaseHistoryAction] `nexa:"string=Action,index,default=note,selection=created:Created,state:Status Change,assignment:Assignment,issue:Issued,note:Note"`
	Field       sdk.String                       `nexa:"string=Field,size=60"`
	OldValue    sdk.String                       `nexa:"string=From,size=200"`
	NewValue    sdk.String                       `nexa:"string=To,size=200"`
	PerformedBy sdk.Many2One[CoreUser]           `nexa:"string=By"`
	PerformedAt sdk.DateTime                     `nexa:"string=When"`
}

// ---------------------------------------------------------------------------
// Disciplinary records (restricted, separate audit)
// ---------------------------------------------------------------------------

// HrDisciplinaryCase records a disciplinary measure against an employee, with
// optional links to the originating grievance or incident.
type HrDisciplinaryCase struct {
	sdk.Model `nexa:"model=hr.disciplinary.case"`

	Name        sdk.String                       `nexa:"required,string=Title"`
	EmployeeID  sdk.Many2One[HrEmployee]         `nexa:"required,string=Employee,index"`
	CaseType    sdk.Selection[DisciplinaryType]  `nexa:"string=Measure,index,default=verbal_warning,selection=verbal_warning:Verbal Warning,written_warning:Written Warning,suspension:Suspension,demotion:Demotion,termination:Termination,other:Other"`
	Severity    sdk.Selection[Severity]          `nexa:"string=Severity,default=medium,selection=low:Low,medium:Medium,high:High,critical:Critical"`
	GrievanceID sdk.Many2One[HrGrievance]        `nexa:"string=Related Grievance"`
	IncidentID  sdk.Many2One[HrIncident]         `nexa:"string=Related Incident"`
	IssueDate   sdk.Date                         `nexa:"string=Issue Date"`
	ExpiryDate  sdk.Date                         `nexa:"string=Expires On"`
	Measures    sdk.Text                         `nexa:"string=Measures"`
	State       sdk.Selection[DisciplinaryState] `nexa:"string=Status,index,default=draft,selection=draft:Draft,issued:Issued,appealed:Appealed,upheld:Upheld,closed:Closed"`
	IssuedBy    sdk.Integer                      `nexa:"string=Issued By"`
	Decision    sdk.Text                         `nexa:"string=Decision Note"`

	History sdk.One2Many[HrDisciplinaryHistory] `nexa:"string=History,inverse=case_id"`
}

// HrDisciplinaryHistory is the separate audit trail for a disciplinary case.
type HrDisciplinaryHistory struct {
	sdk.Model `nexa:"model=hr.disciplinary.history"`

	CaseID      sdk.Many2One[HrDisciplinaryCase] `nexa:"required,string=Case,index"`
	Action      sdk.Selection[CaseHistoryAction] `nexa:"string=Action,index,default=note,selection=created:Created,state:Status Change,issue:Issued,note:Note"`
	Field       sdk.String                       `nexa:"string=Field,size=60"`
	OldValue    sdk.String                       `nexa:"string=From,size=200"`
	NewValue    sdk.String                       `nexa:"string=To,size=200"`
	PerformedBy sdk.Many2One[CoreUser]           `nexa:"string=By"`
	PerformedAt sdk.DateTime                     `nexa:"string=When"`
}

// ---------------------------------------------------------------------------
// Workplace incidents & investigations
// ---------------------------------------------------------------------------

// HrIncident captures a workplace incident and its investigation.
type HrIncident struct {
	sdk.Model `nexa:"model=hr.incident"`

	Name             sdk.String                      `nexa:"required,string=Title"`
	Number           sdk.String                      `nexa:"string=Reference,index"`
	EmployeeID       sdk.Many2One[HrEmployee]        `nexa:"string=Affected Employee,index"`
	IncidentDate     sdk.DateTime                    `nexa:"string=Incident Date,index"`
	Location         sdk.String                      `nexa:"string=Location"`
	Category         sdk.Selection[IncidentCategory] `nexa:"string=Category,index,default=other,selection=injury:Injury,property:Property,security:Security,near_miss:Near Miss,other:Other"`
	Severity         sdk.Selection[Severity]         `nexa:"string=Severity,default=medium,selection=low:Low,medium:Medium,high:High,critical:Critical"`
	Description      sdk.Text                        `nexa:"string=Description"`
	ImmediateAction  sdk.Text                        `nexa:"string=Immediate Action"`
	RootCause        sdk.Text                        `nexa:"string=Root Cause"`
	CorrectiveAction sdk.Text                        `nexa:"string=Corrective Action"`
	State            sdk.Selection[CaseState]        `nexa:"string=Status,index,default=draft,selection=draft:Draft,submitted:Submitted,investigating:Investigating,resolved:Resolved,closed:Closed,cancelled:Cancelled"`
	ReportedBy       sdk.Integer                     `nexa:"string=Reported By"`
	ClosedDate       sdk.DateTime                    `nexa:"string=Closed On"`
}

// ---------------------------------------------------------------------------
// Compliance matrix (gap detection)
// ---------------------------------------------------------------------------

// HrComplianceRequirement defines an obligation and who it applies to.
type HrComplianceRequirement struct {
	sdk.Model `nexa:"model=hr.compliance.requirement"`

	Name          sdk.String                        `nexa:"required,string=Requirement"`
	Category      sdk.Selection[ComplianceCategory] `nexa:"string=Type,index,default=training,selection=policy:Policy,training:Training,certificate:Certification,medical:Medical,document:Document"`
	Scope         sdk.Selection[CompScope]          `nexa:"string=Applies To,default=all,selection=all:All Employees,department:By Department,job:By Job"`
	DepartmentID  sdk.Many2One[HrDepartment]        `nexa:"string=Department"`
	JobID         sdk.Many2One[HrJob]               `nexa:"string=Job Position"`
	PolicyID      sdk.Many2One[HrPolicy]            `nexa:"string=Policy"`
	FrequencyDays sdk.Integer                       `nexa:"string=Frequency (Days),default=0,help=0 = one-time requirement"`
	Mandatory     sdk.Boolean                       `nexa:"string=Mandatory,default=true"`
	Active        sdk.Boolean                       `nexa:"string=Active,default=true"`
}

// HrComplianceLine is a per-employee, per-requirement verdict refreshed by cron.
type HrComplianceLine struct {
	sdk.Model `nexa:"model=hr.compliance.line"`

	EmployeeID    sdk.Many2One[HrEmployee]              `nexa:"required,string=Employee,index"`
	RequirementID sdk.Many2One[HrComplianceRequirement] `nexa:"required,string=Requirement,index"`
	Category      sdk.Selection[ComplianceCategory]     `nexa:"string=Type,default=training"`
	State         sdk.Selection[ComplianceState]        `nexa:"string=Status,index,default=pending,selection=pending:Pending,compliant:Compliant,non_compliant:Non-Compliant,expired:Expired"`
	DueDate       sdk.Date                              `nexa:"string=Due Date,index"`
	LastCompleted sdk.Date                              `nexa:"string=Last Completed"`
	EvidenceRef   sdk.String                            `nexa:"string=Evidence"`
	Note          sdk.Text                              `nexa:"string=Note"`
	Mandatory     sdk.Boolean                           `nexa:"string=Mandatory,default=true"`
}

// ---------------------------------------------------------------------------
// Termination / offboarding workflow
// ---------------------------------------------------------------------------

// HrTermination drives resignation/termination with notice period, handover,
// final-leave capture and record retention. Completion (an approved workflow)
// applies the employee's separation status explicitly.
type HrTermination struct {
	sdk.Model `nexa:"model=hr.termination"`

	Name           sdk.String                      `nexa:"required,string=Summary"`
	EmployeeID     sdk.Many2One[HrEmployee]        `nexa:"required,string=Employee,index"`
	Type           sdk.Selection[TerminationType]  `nexa:"string=Type,index,default=resignation,selection=resignation:Resignation,termination:Termination,retired:Retired,contract_end:Contract End,abandonment:Abandonment,other:Other"`
	InitDate       sdk.Date                        `nexa:"string=Initiation Date"`
	NoticeDays     sdk.Integer                     `nexa:"string=Notice (Days),default=0"`
	LastWorkingDay sdk.Date                        `nexa:"string=Last Working Day"`
	Reason         sdk.Text                        `nexa:"string=Reason"`
	State          sdk.Selection[TerminationState] `nexa:"string=Status,index,default=draft,selection=draft:Draft,submitted:Submitted,approved:Approved,completed:Completed,cancelled:Cancelled"`
	RequestedBy    sdk.Integer                     `nexa:"string=Requested By"`
	ApprovedBy     sdk.Integer                     `nexa:"string=Approved By"`
	DecideDate     sdk.Date                        `nexa:"string=Decision Date"`
	HandoverNotes  sdk.Text                        `nexa:"string=Handover"`
	FinalLeaveDays sdk.Numeric                     `nexa:"string=Leave Balance,precision=8,scale=2"`
	FinalSettleRef sdk.String                      `nexa:"string=Final Settlement Ref,size=60"`
	RetentionUntil sdk.Date                        `nexa:"string=Retain Until"`
	Archived       sdk.Boolean                     `nexa:"string=Archived,default=false"`

	Lines sdk.One2Many[HrOffboardingLine] `nexa:"string=Checklist,inverse=termination_id"`
}

// HrOffboardingLine is one handover / revocation task on a termination.
type HrOffboardingLine struct {
	sdk.Model `nexa:"model=hr.offboarding.line"`

	TerminationID sdk.Many2One[HrTermination]     `nexa:"required,string=Termination,index"`
	Name          sdk.String                      `nexa:"required,string=Task"`
	Category      sdk.Selection[OffboardCategory] `nexa:"string=Area,index,default=admin,selection=it_access:IT Access,equipment:Equipment Return,finance:Finance,manager_kt:Knowledge Transfer,employee:Employee,admin:Admin"`
	ResponsibleID sdk.Many2One[CoreUser]          `nexa:"string=Responsible"`
	Required      sdk.Boolean                     `nexa:"string=Required,default=true"`
	Done          sdk.Boolean                     `nexa:"string=Done,default=false"`
	DoneDate      sdk.DateTime                    `nexa:"string=Done On"`
	Sequence      sdk.Integer                     `nexa:"string=Sequence,default=10"`
}

// ---------------------------------------------------------------------------
// Sensitive / medical records (isolated from ordinary profile access)
// ---------------------------------------------------------------------------

// HrMedicalRecord stores confidential health data, reachable only through a
// restricted group — deliberately separate from the employee profile.
type HrMedicalRecord struct {
	sdk.Model `nexa:"model=hr.medical.record"`

	EmployeeID   sdk.Many2One[HrEmployee]         `nexa:"required,string=Employee,index"`
	RecordType   sdk.Selection[MedicalRecordType] `nexa:"string=Type,index,default=general,selection=general:General,fitness:Fitness Test,occupational:Occupational Health,vaccination:Vaccination,incident_related:Incident Related,other:Other"`
	Description  sdk.Text                         `nexa:"string=Description"`
	Practitioner sdk.String                       `nexa:"string=Practitioner"`
	RecordDate   sdk.Date                         `nexa:"string=Record Date"`
	ValidUntil   sdk.Date                         `nexa:"string=Valid Until,index"`
	CompanyID    sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("hr_compliance",
		&HrPolicy{},
		&HrPolicyAck{},
		&HrGrievance{},
		&HrGrievanceHistory{},
		&HrDisciplinaryCase{},
		&HrDisciplinaryHistory{},
		&HrIncident{},
		&HrComplianceRequirement{},
		&HrComplianceLine{},
		&HrTermination{},
		&HrOffboardingLine{},
		&HrMedicalRecord{},
	)
}

package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
)

// HR-F1 "People & Organization". It deepens the hr anchor without migrating any
// of its tables: it adds an effective-dated, append-only employment history
// (who reported to whom, in which department/position, on which cost centre and
// where, and from when), a job-grade ladder with salary bands, structured
// employee relationships (dependents and emergency contacts), and expiry alerts
// for contracts/probation/documents. The employment snapshot and the expiry
// scan are driven from Go (event + cron) so history is never lost and is always
// reconstructable; nothing here silently mutates the employee's live org fields.

type (
	HrEmployee   = hrmodels.HrEmployee
	HrDepartment = hrmodels.HrDepartment
	HrJob        = hrmodels.HrJob
	CoreCompany  = basemodels.CoreCompany
	CoreUser     = basemodels.CoreUser
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// HistoryReason explains why a new employment snapshot was opened.
type HistoryReason string

const (
	HistoryHire           HistoryReason = "hire"
	HistoryTransfer       HistoryReason = "transfer"
	HistoryPromotion      HistoryReason = "promotion"
	HistoryReorganization HistoryReason = "reorganization"
	HistoryAdjustment     HistoryReason = "adjustment"
	HistoryEnd            HistoryReason = "end"
)

// RelationshipKind separates who an employee depends on / would call.
type RelationshipKind string

const (
	RelKindDependent        RelationshipKind = "dependent"
	RelKindEmergencyContact RelationshipKind = "emergency_contact"
	RelKindOtherContact     RelationshipKind = "other_contact"
)

// RelationshipType is the personal tie to the employee.
type RelationshipType string

const (
	RelSpouse  RelationshipType = "spouse"
	RelChild   RelationshipType = "child"
	RelParent  RelationshipType = "parent"
	RelSibling RelationshipType = "sibling"
	RelOther   RelationshipType = "other"
)

// AlertType is what is approaching expiry.
type AlertType string

const (
	AlertContract  AlertType = "contract"
	AlertProbation AlertType = "probation"
	AlertDocument  AlertType = "document"
)

// AlertState tracks acknowledgement so a scan is idempotent.
type AlertState string

const (
	AlertOpen     AlertState = "open"
	AlertAcked    AlertState = "acked"
	AlertResolved AlertState = "resolved"
)

// ---------------------------------------------------------------------------
// Job grades + salary bands
// ---------------------------------------------------------------------------

// HrJobGrade is a pay-grade rung with an indicative salary band. Jobs point at a
// grade; employees inherit the band through their job. Bands are advisory ranges
// (min/mid/max) — they never auto-write anything into payroll.
type HrJobGrade struct {
	sdk.Model `nexa:"model=hr.job.grade"`

	Name        sdk.String  `nexa:"required,string=Grade"`
	Code        sdk.String  `nexa:"string=Code,index"`
	Level       sdk.Integer `nexa:"string=Level,default=1"`
	SalaryMin   sdk.Numeric `nexa:"string=Band Min,precision=12,scale=2"`
	SalaryMid   sdk.Numeric `nexa:"string=Band Mid,precision=12,scale=2"`
	SalaryMax   sdk.Numeric `nexa:"string=Band Max,precision=12,scale=2"`
	Description sdk.Text    `nexa:"string=Description"`
	Active      sdk.Boolean `nexa:"string=Active,default=true"`
}

// HrJobGradeExtension adds a grade link to the anchor's hr.job without touching
// the hr module's own model file. Requires manifest depends on hr.
type HrJobGradeExtension struct {
	sdk.Model `nexa:"inherit=hr.job"`

	GradeID sdk.Many2One[HrJobGrade] `nexa:"string=Job Grade"`
}

// ---------------------------------------------------------------------------
// Effective-dated employment history (append-only)
// ---------------------------------------------------------------------------

// HrEmploymentHistory is an immutable snapshot of an employee's organisational
// placement over a validity window. The engine keeps exactly one open row per
// employee (valid_to empty); org-field changes close the open row and open a new
// one, preserving the full history.
type HrEmploymentHistory struct {
	sdk.Model `nexa:"model=hr.employment.history"`

	EmployeeID     sdk.Many2One[HrEmployee]     `nexa:"required,string=Employee,index"`
	DepartmentID   sdk.Many2One[HrDepartment]   `nexa:"string=Department"`
	JobID          sdk.Many2One[HrJob]          `nexa:"string=Job Position"`
	ManagerID      sdk.Many2One[HrEmployee]     `nexa:"string=Manager"`
	EmploymentType sdk.String                   `nexa:"string=Employment Type"`
	WorkLocation   sdk.String                   `nexa:"string=Work Location"`
	CostCenter     sdk.String                   `nexa:"string=Cost Center"`
	GradeName      sdk.String                   `nexa:"string=Grade"`
	ValidFrom      sdk.Date                     `nexa:"required,string=Valid From,index"`
	ValidTo        sdk.Date                     `nexa:"string=Valid To,index"`
	Reason         sdk.Selection[HistoryReason] `nexa:"string=Reason,default=hire,selection=hire:On Hire,transfer:Transfer,promotion:Promotion,reorganization:Reorganization,adjustment:Adjustment,end:End of Employment"`
	ChangeNote     sdk.Text                     `nexa:"string=Note"`
	CompanyID      sdk.Many2One[CoreCompany]    `nexa:"string=Company"`

	IsCurrent sdk.Boolean `nexa:"string=Current,compute=compute_history_current"`
}

// ---------------------------------------------------------------------------
// Employee relationships (dependents + emergency contacts)
// ---------------------------------------------------------------------------

// HrEmployeeRelationship records a person tied to an employee — a dependent (for
// benefits/insurance) or an emergency contact. The anchor keeps only flat
// emergency-contact fields; this normalises them into queryable, multiple rows.
type HrEmployeeRelationship struct {
	sdk.Model `nexa:"model=hr.employee.relationship"`

	EmployeeID sdk.Many2One[HrEmployee]        `nexa:"required,string=Employee,index"`
	Kind       sdk.Selection[RelationshipKind] `nexa:"string=Kind,index,default=dependent,selection=dependent:Dependent,emergency_contact:Emergency Contact,other_contact:Other Contact"`
	Name       sdk.String                      `nexa:"required,string=Name"`
	Relation   sdk.Selection[RelationshipType] `nexa:"string=Relationship,default=other,selection=spouse:Spouse,child:Child,parent:Parent,sibling:Sibling,other:Other"`
	BirthDate  sdk.Date                        `nexa:"string=Birth Date"`
	Phone      sdk.String                      `nexa:"string=Phone"`
	Email      sdk.String                      `nexa:"string=Email"`
	IsPrimary  sdk.Boolean                     `nexa:"string=Primary Contact,default=false"`
	Notes      sdk.Text                        `nexa:"string=Notes"`
	CompanyID  sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
	Active     sdk.Boolean                     `nexa:"string=Active,default=true"`
}

// ---------------------------------------------------------------------------
// Expiry alerts (contract / probation / document)
// ---------------------------------------------------------------------------

// HrExpiryAlert is a scan-produced reminder surfaced to HR. Rows are created
// idempotently by the cron (deduped on employee+type+reference+due-date while
// open), so repeated runs never spam duplicates.
type HrExpiryAlert struct {
	sdk.Model `nexa:"model=hr.expiry.alert"`

	EmployeeID sdk.Many2One[HrEmployee]  `nexa:"string=Employee,index"`
	AlertType  sdk.Selection[AlertType]  `nexa:"string=Type,index,default=contract,selection=contract:Contract,probation:Probation,document:Document"`
	Reference  sdk.String                `nexa:"string=Reference"`
	Message    sdk.Text                  `nexa:"string=Message"`
	DueDate    sdk.Date                  `nexa:"string=Due Date,index"`
	State      sdk.Selection[AlertState] `nexa:"string=Status,index,default=open,selection=open:Open,acked:Acknowledged,resolved:Resolved"`
	DetectedAt sdk.DateTime              `nexa:"string=Detected"`
	CompanyID  sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("hr_people",
		&HrJobGrade{},
		&HrJobGradeExtension{},
		&HrEmploymentHistory{},
		&HrEmployeeRelationship{},
		&HrExpiryAlert{},
	)
}

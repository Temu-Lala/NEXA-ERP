package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
)

// Grade promotion/retention and bulk student import. Promotion batches capture
// a per-student decision (pass=>next grade, fail=>repeat) for a from/to
// academic-year transition; import batches stage large sets of student rows
// before they are applied to enrollment. These models keep the audit trail; the
// batch operations themselves are executed by privileged (Manager) users.

type (
	CoreCompany        = basemodels.CoreCompany
	SchoolStudent      = schoolmodels.SchoolStudent
	SchoolClass        = schoolmodels.SchoolClass
	SchoolSection      = schoolmodels.SchoolSection
	SchoolAcademicYear = schoolmodels.SchoolAcademicYear
)

type PromotionBatchState string

const (
	PromDraft  PromotionBatchState = "draft"
	PromDone   PromotionBatchState = "done"
	PromCancel PromotionBatchState = "cancelled"
)

type PromotionResult string

const (
	ResultPass PromotionResult = "pass"
	ResultFail PromotionResult = "fail"
)

type PromotionAction string

const (
	ActPromote PromotionAction = "promote"
	ActRetain  PromotionAction = "retain"
	ActHold    PromotionAction = "hold"
)

type ImportState string

const (
	ImpDraft ImportState = "draft"
	ImpDone  ImportState = "done"
	ImpError ImportState = "error"
)

type ImportRowStatus string

const (
	RowNew       ImportRowStatus = "new"
	RowDuplicate ImportRowStatus = "duplicate"
	RowError     ImportRowStatus = "error"
	RowApplied   ImportRowStatus = "applied"
)

type ImportSource string

const (
	SrcXlsx ImportSource = "xlsx"
	SrcCsv  ImportSource = "csv"
)

// SchoolPromotionBatch groups per-student promotion/retention decisions for a
// year transition.
type SchoolPromotionBatch struct {
	sdk.Model `nexa:"model=school.promotion.batch"`

	Name          sdk.String                         `nexa:"required,size=120,string=Name"`
	FromDate      sdk.Date                           `nexa:"string=Date,default=today"`
	FromYearID    sdk.Many2One[SchoolAcademicYear]   `nexa:"string=From Academic Year"`
	ToYearID      sdk.Many2One[SchoolAcademicYear]   `nexa:"string=To Academic Year"`
	PromotedCount sdk.Integer                        `nexa:"string=Promoted,default=0"`
	RetainedCount sdk.Integer                        `nexa:"string=Retained,default=0"`
	State         sdk.Selection[PromotionBatchState] `nexa:"string=Status,default=draft"`
	CompanyID     sdk.Many2One[CoreCompany]          `nexa:"string=Company"`
	Lines         sdk.One2Many[SchoolPromotionLine]  `nexa:"string=Decisions"`
	Notes         sdk.Text                           `nexa:"string=Notes"`
	Active        sdk.Boolean                        `nexa:"string=Active,default=true"`
}

// SchoolPromotionLine is a single student's promotion/retention decision.
type SchoolPromotionLine struct {
	sdk.Model `nexa:"model=school.promotion.line"`

	Name          sdk.String                         `nexa:"size=120,string=Label"`
	BatchID       sdk.Many2One[SchoolPromotionBatch] `nexa:"required,index,string=Batch"`
	StudentID     sdk.Many2One[SchoolStudent]        `nexa:"required,string=Student"`
	FromClassID   sdk.Many2One[SchoolClass]          `nexa:"string=From Class"`
	FromSectionID sdk.Many2One[SchoolSection]        `nexa:"string=From Section"`
	ToClassID     sdk.Many2One[SchoolClass]          `nexa:"string=To Class"`
	ToSectionID   sdk.Many2One[SchoolSection]        `nexa:"string=To Section"`
	Result        sdk.Selection[PromotionResult]     `nexa:"string=Result,default=pass"`
	Action        sdk.Selection[PromotionAction]     `nexa:"string=Action,default=promote"`
	Notes         sdk.Text                           `nexa:"string=Notes"`
}

// SchoolImportBatch stages a bulk student data import (.xlsx / .csv).
type SchoolImportBatch struct {
	sdk.Model `nexa:"model=school.import.batch"`

	Name           sdk.String                       `nexa:"required,size=120,string=Name"`
	Date           sdk.Date                         `nexa:"string=Date,default=today"`
	Source         sdk.Selection[ImportSource]      `nexa:"string=Source,default=xlsx"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"string=Academic Year"`
	TotalRows      sdk.Integer                      `nexa:"string=Total Rows,default=0"`
	ImportedRows   sdk.Integer                      `nexa:"string=Imported Rows,default=0"`
	ErrorRows      sdk.Integer                      `nexa:"string=Error Rows,default=0"`
	State          sdk.Selection[ImportState]       `nexa:"string=Status,default=draft"`
	CompanyID      sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Lines          sdk.One2Many[SchoolImportLine]   `nexa:"string=Rows"`
	Notes          sdk.Text                         `nexa:"string=Notes"`
	Active         sdk.Boolean                      `nexa:"string=Active,default=true"`
}

// SchoolImportLine is one staged row from a bulk import before/after apply.
type SchoolImportLine struct {
	sdk.Model `nexa:"model=school.import.line"`

	Name       sdk.String                      `nexa:"required,size=120,string=Student Name"`
	BatchID    sdk.Many2One[SchoolImportBatch] `nexa:"required,index,string=Batch"`
	Code       sdk.String                      `nexa:"size=40,string=Student ID"`
	Gender     sdk.String                      `nexa:"size=10,string=Gender"`
	Age        sdk.Integer                     `nexa:"string=Age,default=0"`
	Grade      sdk.String                      `nexa:"size=60,string=Grade/Class"`
	Section    sdk.String                      `nexa:"size=60,string=Section"`
	ParentName sdk.String                      `nexa:"size=120,string=Parent/Guardian"`
	Phone      sdk.String                      `nexa:"size=40,string=Phone"`
	Status     sdk.Selection[ImportRowStatus]  `nexa:"string=Status,default=new"`
	Message    sdk.String                      `nexa:"size=255,string=Message"`
	StudentID  sdk.Many2One[SchoolStudent]     `nexa:"string=Created Student"`
}

func init() {
	sdk.MustRegister("school_promotion",
		&SchoolPromotionBatch{},
		&SchoolPromotionLine{},
		&SchoolImportBatch{},
		&SchoolImportLine{},
	)
}

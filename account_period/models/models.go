package models

import (
	"nexa/core/sdk"
	acctmodels "nexa_addons/account/models"
)

// CoreCompany → core.company
type CoreCompany = acctmodels.CoreCompany

// CoreUser → core.user
type CoreUser = acctmodels.CoreUser

type PeriodCount string

const (
	PeriodCountMonthly   PeriodCount = "monthly"
	PeriodCountQuarterly PeriodCount = "quarterly"
)

type FYState string

const (
	FYStateDraft  FYState = "draft"
	FYStateOpen   FYState = "open"
	FYStateLocked FYState = "locked"
)

type PeriodType string

const (
	PeriodTypeRegular    PeriodType = "regular"
	PeriodTypeAdjustment PeriodType = "adjustment"
	PeriodTypeClosing    PeriodType = "closing"
)

type PeriodState string

const (
	PeriodStateUnopened   PeriodState = "unopened"
	PeriodStateOpen       PeriodState = "open"
	PeriodStateSoftClosed PeriodState = "soft_closed"
	PeriodStateClosed     PeriodState = "closed"
)

type CloseRunState string

const (
	CloseRunDraft      CloseRunState = "draft"
	CloseRunInProgress CloseRunState = "in_progress"
	CloseRunReviewed   CloseRunState = "reviewed"
	CloseRunLocked     CloseRunState = "locked"
)

type CloseTaskCat string

const (
	CloseTaskSalesPurchase CloseTaskCat = "sales_purchase"
	CloseTaskReconcile     CloseTaskCat = "reconcile"
	CloseTaskInventory     CloseTaskCat = "inventory"
	CloseTaskPayroll       CloseTaskCat = "payroll"
	CloseTaskTax           CloseTaskCat = "tax"
	CloseTaskDepreciation  CloseTaskCat = "depreciation"
	CloseTaskAccruals      CloseTaskCat = "accruals"
	CloseTaskFX            CloseTaskCat = "fx"
	CloseTaskTrialBalance  CloseTaskCat = "trial_balance"
	CloseTaskReview        CloseTaskCat = "review"
)

// AccountFiscalYear is an organization's fiscal-year calendar definition.
type AccountFiscalYear struct {
	sdk.Model `nexa:"model=account.fiscal.year"`

	Name            sdk.String                 `nexa:"required,string=Name"`
	Code            sdk.String                 `nexa:"string=Code"`
	CompanyID       sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
	DateFrom        sdk.Date                   `nexa:"required,string=Start Date"`
	DateLast        sdk.Date                   `nexa:"required,string=End Date"`
	PeriodSelection sdk.Selection[PeriodCount] `nexa:"string=Periods,default=monthly"`
	State           sdk.Selection[FYState]     `nexa:"string=Status,default=open"`
	Note            sdk.Text                   `nexa:"string=Notes"`
}

// AccountPeriod is a posting period within a fiscal year with an explicit state.
type AccountPeriod struct {
	sdk.Model `nexa:"model=account.period"`

	FiscalYearID sdk.Many2One[AccountFiscalYear] `nexa:"required,index,string=Fiscal Year"`
	Name         sdk.String                      `nexa:"required,string=Name"`
	Code         sdk.String                      `nexa:"string=Code"`
	DateFrom     sdk.Date                        `nexa:"required,string=Start Date"`
	DateTo       sdk.Date                        `nexa:"required,string=End Date"`
	PeriodType   sdk.Selection[PeriodType]       `nexa:"string=Type,default=regular"`
	State        sdk.Selection[PeriodState]      `nexa:"string=Status,default=unopened"`
	MoveCount    sdk.Integer                     `nexa:"string=Journal Count"`
	Note         sdk.Text                        `nexa:"string=Notes"`
}

// AccountPeriodCloseRun is a controlled period-close workflow with a checklist.
type AccountPeriodCloseRun struct {
	sdk.Model `nexa:"model=account.period.close.run"`

	PeriodID       sdk.Many2One[AccountPeriod]          `nexa:"required,index,string=Period"`
	Name           sdk.String                           `nexa:"required,string=Name"`
	CompanyID      sdk.Many2One[CoreCompany]            `nexa:"string=Company"`
	State          sdk.Selection[CloseRunState]         `nexa:"string=Status,default=draft"`
	StartedBy      sdk.Many2One[CoreUser]               `nexa:"string=Started By"`
	ClosedBy       sdk.Many2One[CoreUser]               `nexa:"string=Closed By"`
	ClosedAt       sdk.DateTime                         `nexa:"string=Closed At"`
	ExceptionCount sdk.Integer                          `nexa:"string=Exceptions"`
	Tasks          sdk.One2Many[AccountPeriodCloseTask] `nexa:"string=Checklist"`
	Note           sdk.Text                             `nexa:"string=Notes"`
}

// AccountPeriodCloseTask is a single checklist item within a close run.
type AccountPeriodCloseTask struct {
	sdk.Model `nexa:"model=account.period.close.task"`

	RunID         sdk.Many2One[AccountPeriodCloseRun] `nexa:"required,index,string=Close Run"`
	Sequence      sdk.Integer                         `nexa:"string=Sequence"`
	Category      sdk.Selection[CloseTaskCat]         `nexa:"string=Category,default=reconcile"`
	Name          sdk.String                          `nexa:"required,string=Task"`
	ResponsibleID sdk.Many2One[CoreUser]              `nexa:"string=Responsible"`
	Done          sdk.Boolean                         `nexa:"string=Done"`
	Evidence      sdk.Text                            `nexa:"string=Evidence"`
	CompletedAt   sdk.DateTime                        `nexa:"string=Completed At"`
}

func init() {
	sdk.MustRegister("account_period",
		&AccountFiscalYear{},
		&AccountPeriod{},
		&AccountPeriodCloseRun{},
		&AccountPeriodCloseTask{},
	)
}

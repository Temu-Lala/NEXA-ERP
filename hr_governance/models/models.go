package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
)

// HR-F12 "Configuration & Governance". It gives HR a declarative control surface —
// settings, a typed parameter registry, approval-policy definitions and integration
// settings — without re-implementing anything the framework already provides. State
// approval itself is still enforced by the core engine (sys.workflow.transition /
// sys.approval.rule) and access is still granted through core.group + sys.access;
// this module lets an HR Administrator author the HR-specific rules those mechanisms
// consume, and keeps every sensitive credential behind the HR Manager group.
//
// Nothing here mutates an employee, a payslip or a ledger. The approval policy is a
// definition (who must approve, above what amount, in how many levels), the setting
// and parameter rows are configuration values, and integration settings only store
// connection metadata — an api key / password column is field-level gated to
// hr.group_hr_manager so it never appears for ordinary HR users.

type (
	HrEmployee   = hrmodels.HrEmployee
	HrDepartment = hrmodels.HrDepartment
	CoreUser     = basemodels.CoreUser
	CoreGroup    = basemodels.CoreGroup
	CoreCompany  = basemodels.CoreCompany
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// ParamType is the value type of a registry parameter (drives validation and
// how a consumer should interpret the stored string).
type ParamType string

const (
	ParamString  ParamType = "string"
	ParamInteger ParamType = "integer"
	ParamBoolean ParamType = "boolean"
	ParamFloat   ParamType = "float"
)

// ParamScope orders resolution precedence: a department value beats a company
// value, which beats a global one.
type ParamScope string

const (
	ScopeGlobal     ParamScope = "global"
	ScopeCompany    ParamScope = "company"
	ScopeDepartment ParamScope = "department"
)

// AppliesTo names the HR transaction an approval policy governs.
type AppliesTo string

const (
	AppliesLeave       AppliesTo = "leave"
	AppliesOvertime    AppliesTo = "overtime"
	AppliesExpense     AppliesTo = "expense"
	AppliesCompChange  AppliesTo = "comp_change"
	AppliesTermination AppliesTo = "termination"
	AppliesHiring      AppliesTo = "hiring"
	AppliesTraining    AppliesTo = "training_request"
	AppliesProcurement AppliesTo = "procurement"
	AppliesPayrollRun  AppliesTo = "payroll_run"
	AppliesOther       AppliesTo = "other"
)

// IntegrationSystem names the external system an integration setting targets.
type IntegrationSystem string

const (
	IntEmail       IntegrationSystem = "email"
	IntAttendance  IntegrationSystem = "attendance_device"
	IntPayrollFile IntegrationSystem = "payroll_file"
	IntBenefits    IntegrationSystem = "benefits_provider"
	IntSSO         IntegrationSystem = "sso"
	IntAccounting  IntegrationSystem = "erp_accounting"
	IntOther       IntegrationSystem = "other"
)

// ---------------------------------------------------------------------------
// hr.setting — company-scoped HR configuration
// ---------------------------------------------------------------------------

// HrSetting collects the HR behavioural switches the companion modules read
// (grace windows, whether overtime needs approval and the auto-approve ceiling,
// leave options, payroll cutoff, self-service editing rights, record retention).
// Values are configuration only; applying them is the responsibility of the module
// that consumes each knob.
type HrSetting struct {
	sdk.Model `nexa:"model=hr.setting"`

	Name      sdk.String                `nexa:"required,string=Name"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company,index"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true,index"`

	AttendanceGraceMinutes   sdk.Integer `nexa:"string=Attendance Grace (min),default=0"`
	OvertimeApprovalRequired sdk.Boolean `nexa:"string=Overtime Requires Approval,default=true"`
	OvertimeAutoApproveBelow sdk.Numeric `nexa:"string=Auto-Approve Overtime Below,precision=18,scale=2,default=0"`
	LeaveHalfDayEnabled      sdk.Boolean `nexa:"string=Allow Half-Day Leave,default=true"`
	LeaveRequiresManager     sdk.Boolean `nexa:"string=Leave Requires Manager Approval,default=true"`
	PayrollCutoffDay         sdk.Integer `nexa:"string=Payroll Cutoff Day,default=5"`
	SelfServiceEditBank      sdk.Boolean `nexa:"string=Self-Service Can Edit Bank,default=false"`
	DataRetentionMonths      sdk.Integer `nexa:"string=Data Retention (months),default=84"`
	RequireApprovalAbove     sdk.Numeric `nexa:"string=Default Approval Threshold,precision=18,scale=2,default=0"`
	Note                     sdk.Text    `nexa:"string=Note"`
}

// ---------------------------------------------------------------------------
// hr.parameter — typed configuration registry
// ---------------------------------------------------------------------------

// HrParameter is a named key/value configuration item resolved by scope. A
// consumer asks for a key and the most specific applicable active row wins
// (department > company > global). Values are stored as text and typed so the
// reader knows how to coerce them; nothing here drives a side effect on its own.
type HrParameter struct {
	sdk.Model `nexa:"model=hr.parameter"`

	Key          sdk.String                 `nexa:"required,string=Key,index"`
	Value        sdk.String                 `nexa:"string=Value"`
	Type         sdk.Selection[ParamType]   `nexa:"string=Type,default=string,selection=string:Text,integer:Integer,boolean:Boolean,float:Float"`
	Scope        sdk.Selection[ParamScope]  `nexa:"string=Scope,default=global,selection=global:Global,company:Company,department:Department"`
	CompanyID    sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
	DepartmentID sdk.Many2One[HrDepartment] `nexa:"string=Department"`
	Description  sdk.String                 `nexa:"string=Description"`
	Active       sdk.Boolean                `nexa:"string=Active,default=true,index"`
}

// ---------------------------------------------------------------------------
// hr.approval.policy + hr.approval.level — approval-workflow definitions
// ---------------------------------------------------------------------------

// HrApprovalPolicy declares, for an HR transaction type, the monetary threshold
// above which approval is required and an ordered set of approver levels. It is a
// definition the operational modules (or the core workflow engine) can consult;
// creating a policy here does not itself approve or move any record.
type HrApprovalPolicy struct {
	sdk.Model `nexa:"model=hr.approval.policy"`

	Name            sdk.String                `nexa:"required,string=Name"`
	AppliesTo       sdk.Selection[AppliesTo]  `nexa:"string=Applies To,default=leave,selection=leave:Leave,overtime:Overtime,expense:Expense,comp_change:Compensation Change,termination:Termination,hiring:Hiring,training_request:Training Request,procurement:Procurement,payroll_run:Payroll Run,other:Other"`
	AmountThreshold sdk.Numeric               `nexa:"string=Approval Threshold,precision=18,scale=2,default=0,help=Requests at or below this amount need no approval; 0 means always approve."`
	Active          sdk.Boolean               `nexa:"string=Active,default=true,index"`
	CompanyID       sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Description     sdk.Text                  `nexa:"string=Description"`

	Lines      sdk.One2Many[HrApprovalLevel] `nexa:"string=Approval Levels,inverse=policy_id"`
	LevelCount sdk.Integer                   `nexa:"string=Levels,compute=compute_policy_levels"`
}

// HrApprovalLevel is one step in a policy's chain: an approver group (and/or a
// specific user), the amount at/above which that step becomes required, and its
// order. Approver groups reference core.group — the same groups sys.access uses —
// so a policy never invents a parallel permission system.
type HrApprovalLevel struct {
	sdk.Model `nexa:"model=hr.approval.level"`

	PolicyID        sdk.Many2One[HrApprovalPolicy] `nexa:"required,string=Policy,index"`
	Sequence        sdk.Integer                    `nexa:"string=Step,default=10"`
	ApproverGroupID sdk.Many2One[CoreGroup]        `nexa:"string=Approver Group"`
	ApproverUserID  sdk.Many2One[CoreUser]         `nexa:"string=Approver User"`
	ThresholdAmount sdk.Numeric                    `nexa:"string=Required At/Above,precision=18,scale=2,default=0"`
	Mandatory       sdk.Boolean                    `nexa:"string=Mandatory,default=true"`
}

// ---------------------------------------------------------------------------
// hr.integration.setting — integration endpoints
// ---------------------------------------------------------------------------

// HrIntegrationSetting stores connection metadata for external systems HR talks to
// (email gateway, attendance device, payroll bank file, benefits provider, SSO ...).
// The credential columns are field-level gated to the HR Manager group so they are
// invisible to ordinary HR users; enabling an integration here is a bookkeeping
// flag, not an automated outbound call.
type HrIntegrationSetting struct {
	sdk.Model `nexa:"model=hr.integration.setting"`

	Name         sdk.String                       `nexa:"required,string=Name"`
	System       sdk.Selection[IntegrationSystem] `nexa:"string=System,default=other,selection=email:Email,attendance_device:Attendance Device,payroll_file:Payroll File / Bank,benefits_provider:Benefits Provider,sso:Single Sign-On,erp_accounting:Accounting / ERP,other:Other"`
	BaseUrl      sdk.String                       `nexa:"string=Base URL"`
	Username     sdk.String                       `nexa:"string=Username"`
	ApiKey       sdk.String                       `nexa:"string=API Key,groups=hr.group_hr_manager"`
	Password     sdk.String                       `nexa:"string=Password,groups=hr.group_hr_manager"`
	Enabled      sdk.Boolean                      `nexa:"string=Enabled,default=false,index"`
	SyncEveryMin sdk.Integer                      `nexa:"string=Sync Every (minutes),default=0"`
	LastSync     sdk.DateTime                     `nexa:"string=Last Sync"`
	CompanyID    sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Note         sdk.Text                         `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("hr_governance",
		&HrSetting{},
		&HrParameter{},
		&HrApprovalPolicy{},
		&HrApprovalLevel{},
		&HrIntegrationSetting{},
	)
}

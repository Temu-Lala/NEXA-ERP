package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Cross-module references kept loose (comodel) to respect ownership:
// product.product (Inventory/Products), account.budget (Accounting),
// approval.request (Approval engine), project.project (Projects),
// maintenance.work.order (Maintenance).

type RequisitionState string

const (
	ReqStateDraft            RequisitionState = "draft"
	ReqStateSubmitted        RequisitionState = "submitted"
	ReqStateBudgetValidation RequisitionState = "budget_validation"
	ReqStateApproval         RequisitionState = "approval"
	ReqStateUnderReview      RequisitionState = "under_review"
	ReqStateApproved         RequisitionState = "approved"
	ReqStateSourcing         RequisitionState = "sourcing"
	ReqStateOrdered          RequisitionState = "ordered"
	ReqStatePartial          RequisitionState = "partial"
	ReqStateFulfilled        RequisitionState = "fulfilled"
	ReqStateDone             RequisitionState = "done"
	ReqStateClosed           RequisitionState = "closed"
	ReqStateRejected         RequisitionState = "rejected"
	ReqStateCancelled        RequisitionState = "cancelled"
)

// BudgetPolicy classifies how strictly a budget breach stops the requisition
// (blueprint §5). It is recorded on the override so the governing rule is
// preserved even if the policy setting later changes.
type BudgetPolicy string

const (
	PolicyNoBudget    BudgetPolicy = "no_budget"
	PolicySoft        BudgetPolicy = "soft"
	PolicyConditional BudgetPolicy = "conditional"
	PolicyHard        BudgetPolicy = "hard"
)

type RequisitionType string

const (
	ReqTypeStock     RequisitionType = "stock_replenishment"
	ReqTypeDirect    RequisitionType = "direct"
	ReqTypeService   RequisitionType = "service"
	ReqTypeProject   RequisitionType = "project"
	ReqTypeMaint     RequisitionType = "maintenance"
	ReqTypeCapex     RequisitionType = "capex"
	ReqTypeEmergency RequisitionType = "emergency"
)

type ReqPriority string

const (
	ReqPriLow    ReqPriority = "low"
	ReqPriNormal ReqPriority = "normal"
	ReqPriHigh   ReqPriority = "high"
	ReqPriUrgent ReqPriority = "urgent"
)

type ProcMethod string

const (
	MethodDirect  ProcMethod = "direct"
	MethodOneQRQ  ProcMethod = "single_quote"
	MethodRFQ     ProcMethod = "rfq"
	MethodBlanket ProcMethod = "blanket_order"
	MethodTender  ProcMethod = "tender"
)

type BudgetResult string

const (
	BudgetNA      BudgetResult = "not_checked"
	BudgetOK      BudgetResult = "within_budget"
	BudgetWarning BudgetResult = "warning"
	BudgetExceed  BudgetResult = "exceeded"
)

type CoreUserAlias = basemodels.CoreUser

// PurchaseRequisition is an internal request to obtain goods or services.
// It is not yet a commitment to a supplier.
type PurchaseRequisition struct {
	sdk.Model `nexa:"model=purchase.requisition"`

	Name             sdk.String                               `nexa:"required,index,string=Reference"`
	DateReq          sdk.Date                                 `nexa:"string=Date,default=today"`
	RequesterID      sdk.Many2One[CoreUserAlias]              `nexa:"string=Requested By"`
	Department       sdk.String                               `nexa:"size=120,string=Department"`
	CostCenter       sdk.String                               `nexa:"size=60,string=Cost Center"`
	Branch           sdk.String                               `nexa:"size=120,string=Branch"`
	ReqType          sdk.Selection[RequisitionType]           `nexa:"string=Type,default=direct"`
	RequiredDate     sdk.Date                                 `nexa:"string=Required By"`
	DeliveryLocation sdk.String                               `nexa:"size=200,string=Delivery Location"`
	Category         sdk.String                               `nexa:"size=120,string=Purchase Category"`
	Justification    sdk.Text                                 `nexa:"string=Business Justification"`
	ProjectRef       sdk.Many2One[sdk.Any]                    `nexa:"string=Project,comodel=project.project"`
	MaintOrderRef    sdk.Many2One[sdk.Any]                    `nexa:"string=Work Order,comodel=maintenance.work.order"`
	PreferredSuppID  sdk.Many2One[sdk.Any]                    `nexa:"string=Preferred Supplier,comodel=core.partner"`
	Method           sdk.Selection[ProcMethod]                `nexa:"string=Procurement Method,default=rfq"`
	Priority         sdk.Selection[ReqPriority]               `nexa:"string=Priority,default=normal"`
	SingleSource     sdk.Boolean                              `nexa:"string=Single Source Justified,default=false"`
	BudgetID         sdk.Many2One[sdk.Any]                    `nexa:"string=Budget,comodel=account.budget"`
	BudgetPolicy     sdk.Selection[BudgetPolicy]              `nexa:"string=Budget Policy,default=conditional"`
	BudgetResult     sdk.Selection[BudgetResult]              `nexa:"string=Budget Check,default=not_checked"`
	ApprovalRef      sdk.Many2One[sdk.Any]                    `nexa:"string=Approval,comodel=approval.request"`
	ApprovalInstance sdk.Many2One[PurchaseApprovalInstance]   `nexa:"string=Procurement Approval"`
	EstTotal         sdk.Numeric                              `nexa:"string=Estimated Total,precision=18,scale=2,default=0"`
	Currency         sdk.String                               `nexa:"size=3,string=Currency,default=USD"`
	State            sdk.Selection[RequisitionState]          `nexa:"string=Status,default=draft"`
	Lines            sdk.One2Many[PurchaseRequisitionLine]    `nexa:"string=Lines"`
	History          sdk.One2Many[PurchaseRequisitionHistory] `nexa:"string=Status History"`
	Notes            sdk.Text                                 `nexa:"string=Notes"`
	Active           sdk.Boolean                              `nexa:"string=Active,default=true"`
}

// PurchaseRequisitionLine is a requested product or service on a requisition.
type PurchaseRequisitionLine struct {
	sdk.Model `nexa:"model=purchase.requisition.line"`

	RequisitionID sdk.Many2One[PurchaseRequisition] `nexa:"required,index,string=Requisition"`
	Sequence      sdk.Integer                       `nexa:"string=Sequence,default=10"`
	ProductID     sdk.Many2One[sdk.Any]             `nexa:"string=Product,comodel=product.product"`
	Name          sdk.String                        `nexa:"required,size=255,string=Description"`
	Quantity      sdk.Numeric                       `nexa:"string=Quantity,precision=18,scale=3,default=1"`
	UoM           sdk.String                        `nexa:"size=30,string=Unit"`
	EstPrice      sdk.Numeric                       `nexa:"string=Est. Unit Price,precision=18,scale=2,default=0"`
	EstSubtotal   sdk.Numeric                       `nexa:"string=Est. Subtotal,precision=18,scale=2,default=0"`
	ApprovedQty   sdk.Numeric                       `nexa:"string=Approved Qty,precision=18,scale=3,default=0"`
	OrderedQty    sdk.Numeric                       `nexa:"string=Ordered Qty,precision=18,scale=3,default=0"`
	Note          sdk.Text                          `nexa:"string=Note"`
}

// PurchaseRequisitionHistory records requisition state transitions.
type PurchaseRequisitionHistory struct {
	sdk.Model `nexa:"model=purchase.requisition.status.history"`

	RequisitionID sdk.Many2One[PurchaseRequisition] `nexa:"required,index,string=Requisition"`
	FromState     sdk.String                        `nexa:"size=30,string=From"`
	ToState       sdk.String                        `nexa:"required,size=30,string=To"`
	UserID        sdk.Many2One[CoreUserAlias]       `nexa:"string=Changed By"`
	Reason        sdk.String                        `nexa:"size=300,string=Reason"`
	Timestamp     sdk.DateTime                      `nexa:"string=At"`
}

// PurchaseBudgetCheck records the outcome of a budget-availability check
// performed at requisition submission or PO approval. Budget data is owned
// by Accounting; Purchase stores only the check snapshot.
type PurchaseBudgetCheck struct {
	sdk.Model `nexa:"model=purchase.budget.check"`

	Name           sdk.String                        `nexa:"required,size=60,string=Reference"`
	RequisitionID  sdk.Many2One[PurchaseRequisition] `nexa:"string=Requisition"`
	BudgetID       sdk.Many2One[sdk.Any]             `nexa:"string=Budget,comodel=account.budget"`
	CheckDate      sdk.Date                          `nexa:"string=Check Date,default=today"`
	ApprovedBudget sdk.Numeric                       `nexa:"string=Approved Budget,precision=18,scale=2,default=0"`
	ActualSpend    sdk.Numeric                       `nexa:"string=Actual Expenditure,precision=18,scale=2,default=0"`
	Commitments    sdk.Numeric                       `nexa:"string=Existing Commitments,precision=18,scale=2,default=0"`
	ProposedAmt    sdk.Numeric                       `nexa:"string=Proposed Amount,precision=18,scale=2,default=0"`
	Remaining      sdk.Numeric                       `nexa:"string=Remaining,precision=18,scale=2,default=0"`
	Result         sdk.Selection[BudgetResult]       `nexa:"string=Result,default=not_checked"`
	Note           sdk.Text                          `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("purchase_requisition",
		&PurchaseRequisition{},
		&PurchaseRequisitionLine{},
		&PurchaseRequisitionHistory{},
		&PurchaseBudgetCheck{},
	)
}

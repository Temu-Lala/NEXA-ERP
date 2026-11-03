package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	productmodels "nexa_addons/product/models"
	stockmodels "nexa_addons/stock/models"
)

// Stock Quality Control (blueprint §5, §7). Plans scope what must be inspected
// (by product, category or supplier) and how; checks capture the inspection
// result with measured characteristics; holds quarantine non-conforming stock
// behind a separation-of-duties release (the inspector who opened a hold can
// never be the one who releases it). Quality never mutates quants — quarantine
// is expressed through the anchor's lot disposition and internal moves.

type (
	ProductProduct  = productmodels.ProductProduct
	ProductCategory = productmodels.ProductCategory
	CoreUser        = basemodels.CoreUser
	StockLot        = stockmodels.StockLot
	StockPicking    = stockmodels.StockPicking
	StockMove       = stockmodels.StockMove
	StockLocation   = stockmodels.StockLocation
)

// ---- Selection types -------------------------------------------------------

type PlanScope string

const (
	ScopeGeneral  PlanScope = "general"
	ScopeProduct  PlanScope = "product"
	ScopeCategory PlanScope = "category"
	ScopeSupplier PlanScope = "supplier"
)

type CheckState string

const (
	CheckDraft  CheckState = "draft"
	CheckDoing  CheckState = "in_progress"
	CheckDone   CheckState = "done"
	CheckCancel CheckState = "cancelled"
)

type CheckResult string

const (
	ResultPending CheckResult = "pending"
	ResultPass    CheckResult = "pass"
	ResultFail    CheckResult = "fail"
	ResultCond    CheckResult = "conditional"
)

type HoldState string

const (
	HoldOnHold   HoldState = "on_hold"
	HoldReleased HoldState = "released"
	HoldScrapped HoldState = "scrapped"
	HoldReturned HoldState = "returned"
)

// ---- Models ----------------------------------------------------------------

// StockQualityPlan defines what gets inspected, how often, and the pass rule.
type StockQualityPlan struct {
	sdk.Model `nexa:"model=stock.quality.plan"`

	Name             sdk.String                    `nexa:"required,index,string=Name"`
	Scope            sdk.Selection[PlanScope]      `nexa:"required,string=Scope,default=general"`
	ProductID        sdk.Many2One[ProductProduct]  `nexa:"index,string=Product"`
	CategoryID       sdk.Many2One[ProductCategory] `nexa:"index,string=Category"`
	PartnerID        sdk.Many2One[sdk.Any]         `nexa:"comodel=core.partner,index,string=Supplier"`
	SamplePercent    sdk.Numeric                   `nexa:"string=Sample %,precision=18,scale=2,default=100"`
	PassThresholdPct sdk.Numeric                   `nexa:"string=Pass Threshold %,precision=18,scale=2,default=100"`
	AutoQuarantine   sdk.Boolean                   `nexa:"string=Quarantine on Receipt,default=false"`
	Active           sdk.Boolean                   `nexa:"string=Active,default=true"`
	Note             sdk.Text                      `nexa:"string=Instructions"`
}

// StockQualityCheck is one inspection against a receipt move / lot.
type StockQualityCheck struct {
	sdk.Model `nexa:"model=stock.quality.check"`

	Name         sdk.String                     `nexa:"required,index,string=Reference"`
	PlanID       sdk.Many2One[StockQualityPlan] `nexa:"index,string=Plan"`
	ProductID    sdk.Many2One[ProductProduct]   `nexa:"required,index,string=Product"`
	PickingID    sdk.Many2One[StockPicking]     `nexa:"index,string=Transfer"`
	MoveID       sdk.Many2One[StockMove]        `nexa:"string=Move"`
	LotID        sdk.Many2One[StockLot]         `nexa:"index,string=Lot"`
	Quantity     sdk.Numeric                    `nexa:"string=Received Qty,precision=18,scale=3,default=0"`
	SampleQty    sdk.Numeric                    `nexa:"string=Sampled Qty,precision=18,scale=3,default=0"`
	DefectQty    sdk.Numeric                    `nexa:"string=Defect Qty,precision=18,scale=3,default=0"`
	Result       sdk.Selection[CheckResult]     `nexa:"string=Result,default=pending"`
	State        sdk.Selection[CheckState]      `nexa:"string=Status,default=draft"`
	InspectorID  sdk.Many2One[CoreUser]         `nexa:"string=Inspector"`
	Measurements sdk.Text                       `nexa:"string=Measured Characteristics"`
	Note         sdk.Text                       `nexa:"string=Notes"`
}

// StockQualityHold quarantines non-conforming stock; release is a guarded,
// second-person approval (separation of duties).
type StockQualityHold struct {
	sdk.Model `nexa:"model=stock.quality.hold"`

	Name       sdk.String                      `nexa:"required,index,string=Reference"`
	CheckID    sdk.Many2One[StockQualityCheck] `nexa:"index,string=Origin Check"`
	ProductID  sdk.Many2One[ProductProduct]    `nexa:"required,index,string=Product"`
	LotID      sdk.Many2One[StockLot]          `nexa:"index,string=Lot"`
	LocationID sdk.Many2One[StockLocation]     `nexa:"string=Location"`
	Quantity   sdk.Numeric                     `nexa:"string=Held Qty,precision=18,scale=3,default=0"`
	State      sdk.Selection[HoldState]        `nexa:"string=Status,default=on_hold"`
	Reason     sdk.Text                        `nexa:"string=Non-conformity"`
	OpenedBy   sdk.Many2One[CoreUser]          `nexa:"string=Opened By"`
	ReleasedBy sdk.Many2One[CoreUser]          `nexa:"string=Released By"`
	Note       sdk.Text                        `nexa:"string=Disposition Notes"`
}

func init() {
	sdk.MustRegister("stock_quality",
		&StockQualityPlan{},
		&StockQualityCheck{},
		&StockQualityHold{},
	)
}

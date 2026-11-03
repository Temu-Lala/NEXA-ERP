package models

import (
	"nexa/core/sdk"
	productmodels "nexa_addons/product/models"
	stockmodels "nexa_addons/stock/models"
)

// Stock Planning (blueprint §9, §15, §17, §23). Replenishment rules encode
// min/max or reorder-point policies per product and warehouse; the evaluate
// engine computes the inventory position from the ledger (eligible on-hand +
// confirmed incoming − committed demand) and writes deduplicated proposals
// that explain their trigger and assumptions. Approval converts a proposal
// into a draft purchase requisition. Metrics and ABC/XYZ classes are derived
// facts — they never feed back into stock quantities.

type (
	ProductProduct = productmodels.ProductProduct
	StockWarehouse = stockmodels.StockWarehouse
)

// ---- Selection types -------------------------------------------------------

type ReplMethod string

const (
	MethodMinMax ReplMethod = "minmax"
	MethodRop    ReplMethod = "rop"
)

type ProposalTrigger string

const (
	TrigMinMax   ProposalTrigger = "minmax"
	TrigRop      ProposalTrigger = "rop"
	TrigStockout ProposalTrigger = "stockout"
	TrigManual   ProposalTrigger = "manual"
)

type ProposalState string

const (
	PropDraft     ProposalState = "draft"
	PropApproved  ProposalState = "approved"
	PropConverted ProposalState = "converted"
	PropCancelled ProposalState = "cancelled"
)

type AlertReason string

const (
	ReasonStockout    AlertReason = "stockout"
	ReasonInTransit   AlertReason = "in_transit_overdue"
	ReasonReservation AlertReason = "stale_reservation"
)

type AlertState string

const (
	AlertOpen     AlertState = "open"
	AlertAcked    AlertState = "acknowledged"
	AlertResolved AlertState = "resolved"
)

type MetricKind string

const (
	MetricTurnover MetricKind = "turnover"
	MetricDIO      MetricKind = "dio"
	MetricCOGS     MetricKind = "period_cogs"
	MetricAvgValue MetricKind = "avg_value"
)

type ABClass string

const (
	ABCA  ABClass = "a"
	ABCB  ABClass = "b"
	ABCC  ABClass = "c"
	ABCNA ABClass = "unc"
)

type XYZClass string

const (
	XYXX  XYZClass = "x"
	XYXY  XYZClass = "y"
	XYZZ  XYZClass = "z"
	XYZNA XYZClass = "unc"
)

// ---- Models ----------------------------------------------------------------

// StockReplenishmentRule is the per product/warehouse purchasing policy
// (blueprint §9.2). One active rule per product+warehouse pair.
type StockReplenishmentRule struct {
	sdk.Model     `nexa:"model=stock.replenishment.rule"`
	Name          sdk.String                   `nexa:"required,index,string=Reference"`
	ProductID     sdk.Many2One[ProductProduct] `nexa:"required,index,string=Product"`
	WarehouseID   sdk.Many2One[StockWarehouse] `nexa:"required,index,string=Warehouse"`
	Method        sdk.Selection[ReplMethod]    `nexa:"string=Method,default=minmax"`
	MinQty        sdk.Numeric                  `nexa:"string=Min Quantity,precision=18,scale=3,default=0"`
	MaxQty        sdk.Numeric                  `nexa:"string=Max Quantity,precision=18,scale=3,default=0"`
	SafetyStock   sdk.Numeric                  `nexa:"string=Safety Stock,precision=18,scale=3,default=0"`
	LeadTimeDays  sdk.Integer                  `nexa:"string=Lead Time (days),default=7"`
	FixedQty      sdk.Numeric                  `nexa:"string=Fixed Order Qty,precision=18,scale=3,default=0"`
	EoqHint       sdk.Boolean                  `nexa:"string=EOQ Style,default=false"`
	MinOrderQty   sdk.Numeric                  `nexa:"string=MOQ,precision=18,scale=3,default=0"`
	OrderMultiple sdk.Numeric                  `nexa:"string=Order Multiple,precision=18,scale=3,default=1"`
	MaxOrderQty   sdk.Numeric                  `nexa:"string=Max per Order,precision=18,scale=3,default=0"`
	Active        sdk.Boolean                  `nexa:"string=Active,default=true"`
	Note          sdk.Text                     `nexa:"string=Note"`
}

// StockReplenishmentProposal is the engine's explained order suggestion. The
// stable dedup key (product+warehouse+origin) keeps reruns idempotent while
// an earlier proposal is still open (§9.4).
type StockReplenishmentProposal struct {
	sdk.Model     `nexa:"model=stock.replenishment.proposal"`
	Name          sdk.String                           `nexa:"required,index,string=Reference"`
	DedupKey      sdk.String                           `nexa:"required,index,size=80,string=Dedup Key"`
	ProductID     sdk.Many2One[ProductProduct]         `nexa:"required,index,string=Product"`
	WarehouseID   sdk.Many2One[StockWarehouse]         `nexa:"required,index,string=Warehouse"`
	Origin        sdk.String                           `nexa:"string=Origin,size=40,default=cron"`
	RuleID        sdk.Many2One[StockReplenishmentRule] `nexa:"index,string=Rule"`
	OnHand        sdk.Numeric                          `nexa:"string=Eligible On Hand,precision=18,scale=3,default=0"`
	Incoming      sdk.Numeric                          `nexa:"string=Confirmed Incoming,precision=18,scale=3,default=0"`
	Committed     sdk.Numeric                          `nexa:"string=Committed Demand,precision=18,scale=3,default=0"`
	Position      sdk.Numeric                          `nexa:"string=Inventory Position,precision=18,scale=3,default=0"`
	DailyDemand   sdk.Numeric                          `nexa:"string=Daily Demand,precision=18,scale=3,default=0"`
	Trigger       sdk.Selection[ProposalTrigger]       `nexa:"string=Trigger,default=minmax"`
	TargetQty     sdk.Numeric                          `nexa:"string=Target,precision=18,scale=3,default=0"`
	SuggestedQty  sdk.Numeric                          `nexa:"string=Suggested Qty,precision=18,scale=3,default=0"`
	Explanation   sdk.Text                             `nexa:"string=Trigger & Assumptions"`
	State         sdk.Selection[ProposalState]         `nexa:"string=Status,default=draft"`
	RequisitionID sdk.Many2One[sdk.Any]                `nexa:"string=Requisition,comodel=purchase.requisition"`
	Date          sdk.DateTime                         `nexa:"string=Computed At"`
}

// StockStockoutAlert is the exception console: stockouts, overdue inbound
// transfers and released stale reservations (§17.3).
type StockStockoutAlert struct {
	sdk.Model   `nexa:"model=stock.stockout.alert"`
	Name        sdk.String                   `nexa:"required,index,string=Reference"`
	ProductID   sdk.Many2One[ProductProduct] `nexa:"required,index,string=Product"`
	WarehouseID sdk.Many2One[StockWarehouse] `nexa:"index,string=Warehouse"`
	Reason      sdk.Selection[AlertReason]   `nexa:"required,index,string=Reason"`
	Quantity    sdk.Numeric                  `nexa:"string=Quantity at Risk,precision=18,scale=3,default=0"`
	SourceRef   sdk.String                   `nexa:"string=Source,size=80"`
	State       sdk.Selection[AlertState]    `nexa:"string=Status,default=open"`
	Date        sdk.DateTime                 `nexa:"string=Raised At"`
	Note        sdk.Text                     `nexa:"string=Note"`
}

// StockReportMetrics holds derived analysis rows (turnover, days-in-cover).
// The Detail column always records numerator/denominator so money and units
// stay reconcilable (§15 consistency rule).
type StockReportMetrics struct {
	sdk.Model  `nexa:"model=stock.report.metrics"`
	Name       sdk.String                   `nexa:"required,index,string=Reference"`
	Metric     sdk.Selection[MetricKind]    `nexa:"required,index,string=Metric"`
	ProductID  sdk.Many2One[ProductProduct] `nexa:"required,index,string=Product"`
	Value      sdk.Numeric                  `nexa:"string=Value,precision=18,scale=3,default=0"`
	PeriodDays sdk.Integer                  `nexa:"string=Period (days),default=365"`
	DateEnd    sdk.DateTime                 `nexa:"string=Period End"`
	Detail     sdk.Text                     `nexa:"string=Numerator / Denominator"`
}

// StockProductClass stores the ABC/XYZ classification written by the
// consumption-value job (§23).
type StockProductClass struct {
	sdk.Model   `nexa:"model=stock.product.class"`
	Name        sdk.String                   `nexa:"required,index,string=Reference"`
	ProductID   sdk.Many2One[ProductProduct] `nexa:"required,index,string=Product"`
	AbcClass    sdk.Selection[ABClass]       `nexa:"string=ABC,default=unc"`
	XyzClass    sdk.Selection[XYZClass]      `nexa:"string=XYZ,default=unc"`
	AnnualValue sdk.Numeric                  `nexa:"string=Annual Consumption Value,precision=18,scale=2,default=0"`
	Rank        sdk.Integer                  `nexa:"string=Rank,default=0"`
	DemandCv    sdk.Numeric                  `nexa:"string=Demand CV,precision=18,scale=3,default=0"`
	LastRun     sdk.DateTime                 `nexa:"string=Last Computed"`
	Note        sdk.Text                     `nexa:"string=Method Note"`
}

func init() {
	sdk.MustRegister("stock_planning",
		&StockReplenishmentRule{},
		&StockReplenishmentProposal{},
		&StockStockoutAlert{},
		&StockReportMetrics{},
		&StockProductClass{},
	)
}

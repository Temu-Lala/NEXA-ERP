package models

import (
	"nexa/core/sdk"
	productmodels "nexa_addons/product/models"
	stockmodels "nexa_addons/stock/models"
)

// Stock Valuation (blueprint §10). Money lives in integer cents; quantities
// keep the ledger precision (18,3). Layers are append-only facts: posted
// quantity/value/unit cost are never rewritten — consumption only decrements
// the remaining pools, and corrections post compensating layers. Accounting
// is decoupled: inventory publishes valuation events, Accounting owns the
// journals (no auto-posted moves in this platform pass).

type (
	ProductProduct = productmodels.ProductProduct
	StockPicking   = stockmodels.StockPicking
	StockMove      = stockmodels.StockMove
)

// ---- Selection types -------------------------------------------------------

type ValMethod string

const (
	MethodFIFO     ValMethod = "fifo"
	MethodAverage  ValMethod = "average"
	MethodStandard ValMethod = "standard"
)

type LayerSource string

const (
	SrcReceipt    LayerSource = "receipt"
	SrcIssue      LayerSource = "issue"
	SrcAdjustment LayerSource = "adjustment"
	SrcLandedCost LayerSource = "landed_cost"
	SrcReversal   LayerSource = "reversal"
	SrcCorrection LayerSource = "correction"
	SrcInventory  LayerSource = "inventory"
)

type EventType string

const (
	EvtLayerPosted    EventType = "layer_posted"
	EvtConsumption    EventType = "consumption"
	EvtReversal       EventType = "reversal"
	EvtLandedCost     EventType = "landed_cost_posted"
	EvtCOGSCorrection EventType = "cogs_correction"
)

type LandedState string

const (
	LandedDraft     LandedState = "draft"
	LandedConfirmed LandedState = "confirmed"
	LandedPosted    LandedState = "posted"
	LandedCancelled LandedState = "cancelled"
)

type LandedBasis string

const (
	BasisQuantity LandedBasis = "quantity"
	BasisWeight   LandedBasis = "weight"
	BasisVolume   LandedBasis = "volume"
	BasisValue    LandedBasis = "value"
)

// ---- Models ----------------------------------------------------------------

// StockValuationLayer is one posted cost fact: a receipt (+qty,+value), an
// issue (-qty,-cost) or a value-only adjustment. The remaining pools shrink
// as later layers consume them; they are the only mutable columns.
type StockValuationLayer struct {
	sdk.Model           `nexa:"model=stock.valuation.layer"`
	Name                sdk.String                        `nexa:"required,index,string=Reference"`
	ProductID           sdk.Many2One[ProductProduct]      `nexa:"required,index,string=Product"`
	PickingID           sdk.Many2One[StockPicking]        `nexa:"index,string=Transfer"`
	MoveID              sdk.Many2One[StockMove]           `nexa:"index,string=Move"`
	SourceLine          sdk.Integer                       `nexa:"string=Ledger Line,index"`
	Method              sdk.Selection[ValMethod]          `nexa:"string=Cost Method,default=fifo"`
	Source              sdk.Selection[LayerSource]        `nexa:"required,string=Type,default=receipt"`
	Quantity            sdk.Numeric                       `nexa:"string=Quantity,precision=18,scale=3,default=0"`
	UnitValueCents      sdk.Integer                       `nexa:"string=Unit Cost (cents),default=0"`
	ValueCents          sdk.Integer                       `nexa:"string=Value (cents),default=0"`
	RemainingQty        sdk.Numeric                       `nexa:"string=Remaining Qty,precision=18,scale=3,default=0"`
	RemainingValueCents sdk.Integer                       `nexa:"string=Remaining Value (cents),default=0"`
	CurrencyCode        sdk.String                        `nexa:"string=Currency,default=USD"`
	Date                sdk.DateTime                      `nexa:"string=Effective Date"`
	ReversalOfID        sdk.Many2One[StockValuationLayer] `nexa:"index,string=Reversal Of"`
	Reversed            sdk.Boolean                       `nexa:"string=Reversed,default=false"`
	Description         sdk.Text                          `nexa:"string=Notes"`
}

// StockValuationEvent is the publish-only outbox towards Accounting (§10.5).
type StockValuationEvent struct {
	sdk.Model     `nexa:"model=stock.valuation.event"`
	Name          sdk.String                        `nexa:"required,index,string=Reference"`
	EventType     sdk.Selection[EventType]          `nexa:"required,index,string=Event"`
	ProductID     sdk.Many2One[ProductProduct]      `nexa:"index,string=Product"`
	LayerID       sdk.Many2One[StockValuationLayer] `nexa:"index,string=Layer"`
	AmountCents   sdk.Integer                       `nexa:"string=Amount (cents),default=0"`
	AccountMoveID sdk.Integer                       `nexa:"string=Accounting Move Id,default=0"`
	Posted        sdk.Boolean                       `nexa:"string=Consumed by Accounting,default=false"`
	Date          sdk.DateTime                      `nexa:"string=Published At"`
	Message       sdk.Text                          `nexa:"string=Details"`
}

// StockLandedCost capitalizes freight/duty/customs onto received stock.
type StockLandedCost struct {
	sdk.Model `nexa:"model=stock.landed.cost"`
	Name      sdk.String                        `nexa:"required,index,string=Reference"`
	PickingID sdk.Many2One[StockPicking]        `nexa:"required,index,string=Received Transfer"`
	Lines     sdk.One2Many[StockLandedCostLine] `nexa:"string=Cost Lines"`
	Basis     sdk.Selection[LandedBasis]        `nexa:"string=Allocate By,default=quantity"`
	State     sdk.Selection[LandedState]        `nexa:"string=Status,default=draft"`
	Note      sdk.Text                          `nexa:"string=Notes"`
}

// StockLandedCostLine is one capitalized charge (freight, duty, insurance...).
type StockLandedCostLine struct {
	sdk.Model   `nexa:"model=stock.landed.cost.line"`
	CostID      sdk.Many2One[StockLandedCost] `nexa:"required,index"`
	Name        sdk.String                    `nexa:"required,string=Description"`
	AmountCents sdk.Integer                   `nexa:"string=Amount (cents),required,default=0"`
}

func init() {
	sdk.MustRegister("stock_valuation",
		&StockValuationLayer{},
		&StockValuationEvent{},
		&StockLandedCost{},
		&StockLandedCostLine{},
	)
}

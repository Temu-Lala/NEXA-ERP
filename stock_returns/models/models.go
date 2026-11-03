package models

import (
	"nexa/core/sdk"
	productmodels "nexa_addons/product/models"
	stockmodels "nexa_addons/stock/models"
)

// Stock Returns (blueprint §12). RMA (customer) and DTV (vendor-direct)
// return orders reference the originating delivery; each line carries
// condition and lot/serial evidence. Confirmation checks the policy window,
// receipt executes the chosen dispositions through the validated move
// ledger, and restock stays gated on inspection while the policy demands it.

type (
	ProductProduct = productmodels.ProductProduct
	StockPicking   = stockmodels.StockPicking
	StockWarehouse = stockmodels.StockWarehouse
	StockLot       = stockmodels.StockLot
	StockSerial    = stockmodels.StockSerial
)

// ---- Selection types -------------------------------------------------------

type ReturnOrderType string

const (
	TypeRMA ReturnOrderType = "customer"
	TypeDTV ReturnOrderType = "vendor_direct"
)

type ReturnState string

const (
	ReturnDraft     ReturnState = "draft"
	ReturnConfirmed ReturnState = "confirmed"
	ReturnReceived  ReturnState = "received"
	ReturnDone      ReturnState = "done"
	ReturnCancelled ReturnState = "cancelled"
)

type ReturnCondition string

const (
	CondSellable ReturnCondition = "sellable"
	CondDamaged  ReturnCondition = "damaged"
	CondExpired  ReturnCondition = "expired"
	CondUnknown  ReturnCondition = "unknown"
)

type ReturnDisposition string

const (
	DispRestock    ReturnDisposition = "restock"
	DispQuarantine ReturnDisposition = "quarantine"
	DispRepair     ReturnDisposition = "repair"
	DispRTV        ReturnDisposition = "return_vendor"
	DispScrap      ReturnDisposition = "scrap"
	DispReject     ReturnDisposition = "reject"
)

// ---- Models ----------------------------------------------------------------

// StockReturnPolicy encodes the eligibility window and the inspection gate.
// A policy without a product is the fallback for the whole catalogue.
type StockReturnPolicy struct {
	sdk.Model   `nexa:"model=stock.return.policy"`
	Name        sdk.String                   `nexa:"required,index,string=Reference"`
	ProductID   sdk.Many2One[ProductProduct] `nexa:"index,string=Product"`
	WindowDays  sdk.Integer                  `nexa:"string=Window (days),default=30"`
	InspectGate sdk.Boolean                  `nexa:"string=Restock Needs Inspection,default=true"`
	Active      sdk.Boolean                  `nexa:"string=Active,default=true"`
	Note        sdk.Text                     `nexa:"string=Note"`
}

// StockReturnOrder is the RMA/DTV header.
type StockReturnOrder struct {
	sdk.Model       `nexa:"model=stock.return.order"`
	Name            sdk.String                      `nexa:"required,index,string=Reference"`
	Type            sdk.Selection[ReturnOrderType]  `nexa:"string=Type,default=customer"`
	PartnerID       sdk.Many2One[sdk.Any]           `nexa:"index,string=Customer / Vendor,comodel=core.partner"`
	OriginPickingID sdk.Many2One[StockPicking]      `nexa:"index,string=Delivered Transfer"`
	WarehouseID     sdk.Many2One[StockWarehouse]    `nexa:"required,index,string=Return Warehouse"`
	PolicyID        sdk.Many2One[StockReturnPolicy] `nexa:"string=Policy"`
	State           sdk.Selection[ReturnState]      `nexa:"string=Status,default=draft"`
	Date            sdk.DateTime                    `nexa:"string=Raised At"`
	Lines           sdk.One2Many[StockReturnLine]   `nexa:"string=Return Lines"`
	Note            sdk.Text                        `nexa:"string=Notes"`
}

// StockReturnLine is one returned product with condition, evidence and the
// approved disposition. Eligibility is stamped by the confirmation check.
type StockReturnLine struct {
	sdk.Model   `nexa:"model=stock.return.line"`
	OrderID     sdk.Many2One[StockReturnOrder]   `nexa:"required,index,string=Return Order"`
	ProductID   sdk.Many2One[ProductProduct]     `nexa:"required,index,string=Product"`
	Quantity    sdk.Numeric                      `nexa:"string=Quantity,precision=18,scale=3,default=1"`
	LotID       sdk.Many2One[StockLot]           `nexa:"index,string=Lot"`
	SerialID    sdk.Many2One[StockSerial]        `nexa:"index,string=Serial"`
	Condition   sdk.Selection[ReturnCondition]   `nexa:"string=Condition,default=unknown"`
	Disposition sdk.Selection[ReturnDisposition] `nexa:"string=Disposition,default=quarantine"`
	Eligible    sdk.Boolean                      `nexa:"string=Eligible,default=false"`
	PolicyNote  sdk.String                       `nexa:"string=Policy Check,size=200"`
	Inspected   sdk.Boolean                      `nexa:"string=Inspected,default=false"`
	InspectedBy sdk.Integer                      `nexa:"string=Inspected By ID,default=0"`
	Note        sdk.Text                         `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("stock_returns",
		&StockReturnPolicy{},
		&StockReturnOrder{},
		&StockReturnLine{},
	)
}

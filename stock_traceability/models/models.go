package models

import (
	"nexa/core/sdk"
	stockmodels "nexa_addons/stock/models"
)

// Stock Traceability (blueprint §8). The Inventory anchor already owns the
// lot and serial registries and enforces lot/serial capture on the ledger;
// this companion adds the movement-derived traceability chain, the recall /
// quarantine / release disposition workflow on lots, and scheduled expiry
// monitoring with idempotent alerts. It never re-creates anchor tables and
// never edits quants — dispositions act on the anchor's own guarded paths.

type (
	StockLot       = stockmodels.StockLot
	StockSerial    = stockmodels.StockSerial
	StockPicking   = stockmodels.StockPicking
	StockMove      = stockmodels.StockMove
	StockMoveLine  = stockmodels.StockMoveLine
	StockLocation  = stockmodels.StockLocation
	ProductProduct = stockmodels.ProductProduct
)

// ---- Selection types -------------------------------------------------------

type TraceEventType string

const (
	TraceEvtReceipt    TraceEventType = "receipt"
	TraceEvtInternal   TraceEventType = "internal"
	TraceEvtDelivery   TraceEventType = "delivery"
	TraceEvtReturn     TraceEventType = "return"
	TraceEvtScrap      TraceEventType = "scrap"
	TraceEvtRecall     TraceEventType = "recall"
	TraceEvtQuarantine TraceEventType = "quarantine"
	TraceEvtRelease    TraceEventType = "release"
	TraceEvtExpiry     TraceEventType = "expiry"
)

type ExpirySeverity string

const (
	ExpiryWarning  ExpirySeverity = "warning"
	ExpiryCritical ExpirySeverity = "critical"
	ExpiryExpired  ExpirySeverity = "expired"
)

// ---- Models ----------------------------------------------------------------

// StockTraceabilityEvent is one node of the origin/destination chain derived
// from posted ledger operations plus disposition actions (§8.1). Rows sourced
// from operations are produced idempotently by the sync engine keyed on
// SourceLine; workflow rows are written directly by the lot actions.
type StockTraceabilityEvent struct {
	sdk.Model `nexa:"model=stock.traceability.event"`

	Name             sdk.String                    `nexa:"required,index,string=Reference"`
	EventType        sdk.Selection[TraceEventType] `nexa:"required,index,string=Event Type,default=internal"`
	ProductID        sdk.Many2One[ProductProduct]  `nexa:"index,string=Product"`
	LotID            sdk.Many2One[StockLot]        `nexa:"index,string=Lot"`
	SerialID         sdk.Many2One[StockSerial]     `nexa:"index,string=Serial"`
	PickingID        sdk.Many2One[StockPicking]    `nexa:"index,string=Transfer"`
	SourceLocationID sdk.Many2One[StockLocation]   `nexa:"string=From"`
	ResultLocationID sdk.Many2One[StockLocation]   `nexa:"string=To"`
	Quantity         sdk.Numeric                   `nexa:"string=Quantity,precision=18,scale=3,default=0"`
	DateDone         sdk.DateTime                  `nexa:"string=Event Date"`
	SourceLine       sdk.Integer                   `nexa:"string=Origin Operation ID"`
	Description      sdk.Text                      `nexa:"string=Details"`
}

// StockLotExpiryAlert is an idempotent expiry warning for a lot, raised by
// the expiry-monitor cron while the lot sits inside the warning window.
// Resolving (or re-dispositioning) the lot closes the alert.
type StockLotExpiryAlert struct {
	sdk.Model `nexa:"model=stock.lot.expiry.alert"`

	LotID         sdk.Many2One[StockLot]        `nexa:"required,index,string=Lot"`
	ProductID     sdk.Many2One[ProductProduct]  `nexa:"index,string=Product"`
	ExpirationOn  sdk.Date                      `nexa:"string=Expiration Date"`
	DaysRemaining sdk.Integer                   `nexa:"string=Days Left,default=0"`
	Severity      sdk.Selection[ExpirySeverity] `nexa:"string=Severity,default=warning"`
	Open          sdk.Boolean                   `nexa:"string=Open,default=true"`
	Message       sdk.Text                      `nexa:"string=Message"`
}

func init() {
	sdk.MustRegister("stock_traceability",
		&StockTraceabilityEvent{},
		&StockLotExpiryAlert{},
	)
}

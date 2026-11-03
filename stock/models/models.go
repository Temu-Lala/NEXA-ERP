package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	productmodels "nexa_addons/product/models"
)

// ProductProduct → product.product
type ProductProduct = productmodels.ProductProduct

// CoreUser → res.users (base)
type CoreUser = basemodels.CoreUser

type LocationUsage string

const (
	LocationUsageInternal   LocationUsage = "internal"
	LocationUsageView       LocationUsage = "view"
	LocationUsageCustomer   LocationUsage = "customer"
	LocationUsageSupplier   LocationUsage = "supplier"
	LocationUsageInventory  LocationUsage = "inventory"
	LocationUsageTransit    LocationUsage = "transit"
	LocationUsageQuarantine LocationUsage = "quarantine"
	LocationUsageDamaged    LocationUsage = "damaged"
	LocationUsageScrap      LocationUsage = "scrap"
	LocationUsageProduction LocationUsage = "production"
	LocationUsageStaging    LocationUsage = "staging"
	LocationUsageReturns    LocationUsage = "returns"
)

type PickingOperation string

const (
	PickingOperationIncoming PickingOperation = "incoming"
	PickingOperationOutgoing PickingOperation = "outgoing"
	PickingOperationInternal PickingOperation = "internal"
)

type PickingState string

const (
	PickingStateDraft     PickingState = "draft"
	PickingStateConfirmed PickingState = "confirmed"
	PickingStateAssigned  PickingState = "assigned"
	PickingStateDone      PickingState = "done"
	PickingStateCancelled PickingState = "cancelled"
)

type InventoryState string

const (
	InventoryStateDraft     InventoryState = "draft"
	InventoryStateProgress  InventoryState = "in_progress"
	InventoryStateDone      InventoryState = "done"
	InventoryStateCancelled InventoryState = "cancelled"
)

// LotState is the controlled quality disposition of a lot/batch.
type LotState string

const (
	LotStateReleased   LotState = "released"
	LotStateQuarantine LotState = "quarantined"
	LotStateRecalled   LotState = "recalled"
)

// SerialState is the individual-unit lifecycle (blueprint §8.3).
type SerialState string

const (
	SerialStateExpected  SerialState = "expected"
	SerialStateReceived  SerialState = "received"
	SerialStateAvailable SerialState = "available"
	SerialStateReserved  SerialState = "reserved"
	SerialStateDelivered SerialState = "delivered"
	SerialStateReturned  SerialState = "returned"
	SerialStateInRepair  SerialState = "in_repair"
	SerialStateScrapped  SerialState = "scrapped"
	SerialStateLost      SerialState = "lost"
)

type ScrapState string

const (
	ScrapStateDraft     ScrapState = "draft"
	ScrapStateDone      ScrapState = "done"
	ScrapStateCancelled ScrapState = "cancelled"
)

type StockWarehouse struct {
	sdk.Model `nexa:"model=stock.warehouse"`

	Name              sdk.String                  `nexa:"required,string=Warehouse Name"`
	Code              sdk.String                  `nexa:"string=Short Code"`
	Address           sdk.Text                    `nexa:"string=Address"`
	InputLocationID   sdk.Many2One[StockLocation] `nexa:"string=Input Location"`
	OutputLocationID  sdk.Many2One[StockLocation] `nexa:"string=Output Location"`
	ReturnLocationID  sdk.Many2One[StockLocation] `nexa:"string=Return Location"`
	ScrapLocationID   sdk.Many2One[StockLocation] `nexa:"string=Scrap Location"`
	TransitLocationID sdk.Many2One[StockLocation] `nexa:"string=Transit Location"`
	AllowNegative     sdk.Boolean                 `nexa:"string=Allow Negative Stock,default=false"`
	Active            sdk.Boolean                 `nexa:"string=Active,default=true"`
}

type StockLocation struct {
	sdk.Model `nexa:"model=stock.location"`

	Name        sdk.String                   `nexa:"required,string=Location Name"`
	Usage       sdk.Selection[LocationUsage] `nexa:"string=Location Type,default=internal"`
	ParentID    sdk.Many2One[StockLocation]  `nexa:"string=Parent Location"`
	WarehouseID sdk.Many2One[StockWarehouse] `nexa:"string=Warehouse"`
	PosPriority sdk.Integer                  `nexa:"string=Picking Priority,default=0"`
	Blocked     sdk.Boolean                  `nexa:"string=Blocked,default=false"`
	CapacityKG  sdk.Numeric                  `nexa:"string=Max Weight (kg),precision=18,scale=3,default=0"`
	CapacityM3  sdk.Numeric                  `nexa:"string=Max Volume (m3),precision=18,scale=3,default=0"`
	Active      sdk.Boolean                  `nexa:"string=Active,default=true"`
	Note        sdk.Text                     `nexa:"string=Note"`
}

// StockPickingType configures an operation channel (incoming/outgoing/
// internal) with its defaults. It is the controlled vocabulary behind every
// transfer (blueprint §4.3, §6.1, §7).
type StockPickingType struct {
	sdk.Model `nexa:"model=stock.picking.type"`

	Name          sdk.String                      `nexa:"required,string=Name"`
	Code          sdk.String                      `nexa:"required,string=Code"`
	OperationType sdk.Selection[PickingOperation] `nexa:"string=Operation Type,default=incoming"`
	WarehouseID   sdk.Many2One[StockWarehouse]    `nexa:"string=Warehouse"`
	Sequence      sdk.Integer                     `nexa:"string=Sequence,default=10"`
	BackorderOK   sdk.Boolean                     `nexa:"string=Allow Backorders,default=true"`
	ReturnOK      sdk.Boolean                     `nexa:"string=Allow Returns,default=true"`
	Active        sdk.Boolean                     `nexa:"string=Active,default=true"`
	Note          sdk.Text                        `nexa:"string=Note"`
}

// StockProcurementGroup is a scheduling bucket that ties proposals and moves
// back to the demand that triggered them (blueprint §9).
type StockProcurementGroup struct {
	sdk.Model `nexa:"model=stock.procurement.group"`

	Name sdk.String `nexa:"required,string=Name"`
	Note sdk.Text   `nexa:"string=Note"`
}

type StockLot struct {
	sdk.Model `nexa:"model=stock.lot"`

	Name         sdk.String                   `nexa:"required,string=Lot Number"`
	ProductID    sdk.Many2One[ProductProduct] `nexa:"required,string=Product"`
	SupplierRef  sdk.String                   `nexa:"string=Supplier Lot"`
	ProductionOn sdk.Date                     `nexa:"string=Production Date"`
	ExpirationOn sdk.Date                     `nexa:"string=Expiration Date"`
	RemovalOn    sdk.Date                     `nexa:"string=Removal Date"`
	State        sdk.Selection[LotState]      `nexa:"string=Lot Status,default=released"`
	QtyRemaining sdk.Float64                  `nexa:"string=On Hand,compute=compute_lot_qty_remaining"`
	Description  sdk.Text                     `nexa:"string=Notes / Certifications"`
	Active       sdk.Boolean                  `nexa:"string=Active,default=true"`
}

type StockSerial struct {
	sdk.Model `nexa:"model=stock.serial"`

	Name       sdk.String                   `nexa:"required,string=Serial Number"`
	ProductID  sdk.Many2One[ProductProduct] `nexa:"required,string=Product"`
	LotID      sdk.Many2One[StockLot]       `nexa:"string=Lot"`
	State      sdk.Selection[SerialState]   `nexa:"string=Serial Status,default=available"`
	LocationID sdk.Many2One[StockLocation]  `nexa:"string=Current Location"`
	Note       sdk.Text                     `nexa:"string=Notes"`
	Active     sdk.Boolean                  `nexa:"string=Active,default=true"`
}

type StockPicking struct {
	sdk.Model `nexa:"model=stock.picking"`

	Name               sdk.String                          `nexa:"required,string=Reference"`
	PickingTypeID      sdk.Many2One[StockPickingType]      `nexa:"string=Operation Type"`
	Operation          sdk.Selection[PickingOperation]     `nexa:"string=Operation,default=incoming"`
	SourceLocationID   sdk.Many2One[StockLocation]         `nexa:"string=Source Location"`
	DestinationID      sdk.Many2One[StockLocation]         `nexa:"string=Destination Location"`
	State              sdk.Selection[PickingState]         `nexa:"string=Status,default=draft"`
	ScheduledDate      sdk.DateTime                        `nexa:"string=Scheduled Date"`
	DateDone           sdk.DateTime                        `nexa:"string=Done Date"`
	AssignedUserID     sdk.Many2One[CoreUser]              `nexa:"string=Assigned To"`
	Origin             sdk.String                          `nexa:"string=Source Document"`
	ProcurementGroupID sdk.Many2One[StockProcurementGroup] `nexa:"string=Procurement Group"`
	MoveIDs            sdk.One2Many[StockMove]             `nexa:"string=Moves"`
	OperationIDs       sdk.One2Many[StockMoveLine]         `nexa:"string=Operations"`
	Note               sdk.Text                            `nexa:"string=Notes"`
}

type StockMove struct {
	sdk.Model `nexa:"model=stock.move"`

	Name               sdk.String                          `nexa:"string=Description"`
	ProductID          sdk.Many2One[ProductProduct]        `nexa:"required,string=Product"`
	PickingID          sdk.Many2One[StockPicking]          `nexa:"string=Transfer"`
	SourceLocationID   sdk.Many2One[StockLocation]         `nexa:"string=Source Location"`
	DestinationID      sdk.Many2One[StockLocation]         `nexa:"string=Destination Location"`
	Quantity           sdk.Numeric                         `nexa:"string=Demand,precision=18,scale=3"`
	ReservedQuantity   sdk.Numeric                         `nexa:"string=Reserved,precision=18,scale=3,default=0"`
	State              sdk.Selection[PickingState]         `nexa:"string=Status,default=draft"`
	DateDue            sdk.DateTime                        `nexa:"string=Deadline"`
	DateDone           sdk.DateTime                        `nexa:"string=Done Date"`
	LotID              sdk.Many2One[StockLot]              `nexa:"string=Lot"`
	ProcurementGroupID sdk.Many2One[StockProcurementGroup] `nexa:"string=Procurement Group"`
	Origin             sdk.String                          `nexa:"string=Source Document"`
	MoveLines          sdk.One2Many[StockMoveLine]         `nexa:"string=Operations"`
}

// StockMoveLine is a recorded physical operation against a move (which lot,
// which serial, how many, into which bin). Lines are the fulfillment detail
// behind the ledger and are append-only once the move is posted.
type StockMoveLine struct {
	sdk.Model `nexa:"model=stock.move.line"`

	MoveID           sdk.Many2One[StockMove]      `nexa:"required,string=Move"`
	PickingID        sdk.Many2One[StockPicking]   `nexa:"string=Transfer"`
	ProductID        sdk.Many2One[ProductProduct] `nexa:"required,string=Product"`
	LotID            sdk.Many2One[StockLot]       `nexa:"string=Lot"`
	SerialID         sdk.Many2One[StockSerial]    `nexa:"string=Serial"`
	Quantity         sdk.Numeric                  `nexa:"string=Done Quantity,precision=18,scale=3,default=0"`
	SourceLocationID sdk.Many2One[StockLocation]  `nexa:"string=Source Location"`
	ResultLocationID sdk.Many2One[StockLocation]  `nexa:"string=Result Location"`
}

type StockQuant struct {
	sdk.Model `nexa:"model=stock.quant"`

	ProductID         sdk.Many2One[ProductProduct] `nexa:"required,string=Product"`
	LocationID        sdk.Many2One[StockLocation]  `nexa:"required,string=Location"`
	LotID             sdk.Many2One[StockLot]       `nexa:"string=Lot"`
	Quantity          sdk.Numeric                  `nexa:"string=On Hand,precision=18,scale=3,default=0"`
	ReservedQuantity  sdk.Numeric                  `nexa:"string=Reserved,precision=18,scale=3,default=0"`
	InDate            sdk.DateTime                 `nexa:"string=In Date"`
	AvailableQuantity sdk.Numeric                  `nexa:"string=Available,precision=18,scale=3,default=0,compute=compute_quant_available,store"`
}

type StockInventory struct {
	sdk.Model `nexa:"model=stock.inventory"`

	Name          sdk.String                       `nexa:"required,string=Reference"`
	InventoryDate sdk.Date                         `nexa:"string=Inventory Date"`
	LocationID    sdk.Many2One[StockLocation]      `nexa:"string=Count Location"`
	BlindCount    sdk.Boolean                      `nexa:"string=Blind Count,default=false"`
	State         sdk.Selection[InventoryState]    `nexa:"string=Status,default=draft"`
	Lines         sdk.One2Many[StockInventoryLine] `nexa:"string=Lines"`
	Note          sdk.Text                         `nexa:"string=Notes"`
}

type StockInventoryLine struct {
	sdk.Model `nexa:"model=stock.inventory.line"`

	InventoryID    sdk.Many2One[StockInventory] `nexa:"required,string=Inventory"`
	ProductID      sdk.Many2One[ProductProduct] `nexa:"required,string=Product"`
	LocationID     sdk.Many2One[StockLocation]  `nexa:"string=Location"`
	LotID          sdk.Many2One[StockLot]       `nexa:"string=Lot"`
	TheoreticalQty sdk.Numeric                  `nexa:"string=Theoretical Qty,precision=18,scale=3,default=0"`
	CountedQty     sdk.Numeric                  `nexa:"string=Counted Qty,precision=18,scale=3,default=0"`
	DifferenceQty  sdk.Numeric                  `nexa:"string=Difference,precision=18,scale=3,default=0,compute=compute_inventory_line_difference,store"`
	Reason         sdk.String                   `nexa:"string=Reason"`
}

// StockScrap is a controlled write-off of physical stock to a scrap location
// (blueprint §4.3, §13). It posts a real move; it never edits quants.
type StockScrap struct {
	sdk.Model `nexa:"model=stock.scrap"`

	Name            sdk.String                   `nexa:"required,string=Reference"`
	ProductID       sdk.Many2One[ProductProduct] `nexa:"required,string=Product"`
	LotID           sdk.Many2One[StockLot]       `nexa:"string=Lot"`
	Quantity        sdk.Numeric                  `nexa:"required,string=Quantity,precision=18,scale=3,default=0"`
	LocationID      sdk.Many2One[StockLocation]  `nexa:"required,string=Source Location"`
	ScrapLocationID sdk.Many2One[StockLocation]  `nexa:"string=Scrap Location"`
	PickingID       sdk.Many2One[StockPicking]   `nexa:"string=Origin Transfer"`
	Reason          sdk.String                   `nexa:"string=Reason Code"`
	State           sdk.Selection[ScrapState]    `nexa:"string=Status,default=draft"`
	Note            sdk.Text                     `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("stock",
		&StockWarehouse{},
		&StockLocation{},
		&StockPickingType{},
		&StockProcurementGroup{},
		&StockLot{},
		&StockSerial{},
		&StockPicking{},
		&StockMove{},
		&StockMoveLine{},
		&StockQuant{},
		&StockInventory{},
		&StockInventoryLine{},
		&StockScrap{},
	)
}

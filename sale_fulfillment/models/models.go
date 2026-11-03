package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	salemodels "nexa_addons/sale/models"
)

// Sales coordinates fulfillment and returns, but Inventory owns reservations
// and shipments (stock.picking) and Accounting owns credit notes (account.move).
// These models are link/coordination records referencing those authoritative
// documents via comodel — Sales never becomes the source of truth for stock or
// financial balances.

type SaleOrder = salemodels.SaleOrder

type CoreUser = basemodels.CoreUser
type CorePartner = basemodels.CorePartner

type ShipmentState string

const (
	ShipPlanned   ShipmentState = "planned"
	ShipPicking   ShipmentState = "picking"
	ShipPacked    ShipmentState = "packed"
	ShipDelivered ShipmentState = "delivered"
	ShipReturned  ShipmentState = "returned"
	ShipCancelled ShipmentState = "cancelled"
)

type ReturnState string

const (
	RetDraft     ReturnState = "draft"
	RetApproved  ReturnState = "approved"
	RetReceived  ReturnState = "received"
	RetInspected ReturnState = "inspected"
	RetSettled   ReturnState = "settled"
	RetRejected  ReturnState = "rejected"
	RetClosed    ReturnState = "closed"
)

// SaleOrderReservationLink references an Inventory reservation made for an
// order line. Quantities here are a mirrored snapshot for visibility only.
type SaleOrderReservationLink struct {
	sdk.Model `nexa:"model=sale.order.reservation.link"`

	Name         sdk.String              `nexa:"required,size=60,string=Reference"`
	OrderID      sdk.Many2One[SaleOrder] `nexa:"required,index,string=Order"`
	MoveRef      sdk.Many2One[sdk.Any]   `nexa:"string=Stock Move,comodel=stock.move"`
	ProductID    sdk.Many2One[sdk.Any]   `nexa:"string=Product,comodel=product.product"`
	ReservedQty  sdk.Numeric             `nexa:"string=Reserved Qty,precision=18,scale=3,default=0"`
	BackorderQty sdk.Numeric             `nexa:"string=Backordered Qty,precision=18,scale=3,default=0"`
	WarehouseID  sdk.Many2One[sdk.Any]   `nexa:"string=Warehouse,comodel=stock.warehouse"`
	LinkDate     sdk.Date                `nexa:"string=Date,default=today"`
	Note         sdk.Text                `nexa:"string=Note"`
}

// SaleShipmentLink records the shipment/delivery commitment for an order and
// references the authoritative Inventory transfer.
type SaleShipmentLink struct {
	sdk.Model `nexa:"model=sale.shipment.link"`

	Name          sdk.String                   `nexa:"required,index,string=Reference"`
	OrderID       sdk.Many2One[SaleOrder]      `nexa:"required,index,string=Order"`
	PickingRef    sdk.Many2One[sdk.Any]        `nexa:"string=Inventory Transfer,comodel=stock.picking"`
	DeliveryAddr  sdk.Text                     `nexa:"string=Delivery Address"`
	Carrier       sdk.String                   `nexa:"size=120,string=Carrier"`
	TrackingRef   sdk.String                   `nexa:"size=120,string=Tracking Ref"`
	ScheduledDate sdk.Date                     `nexa:"string=Scheduled Date"`
	DeliveredDate sdk.Date                     `nexa:"string=Delivered Date"`
	State         sdk.Selection[ShipmentState] `nexa:"string=Status,default=planned"`
	Notes         sdk.Text                     `nexa:"string=Notes"`
	Active        sdk.Boolean                  `nexa:"string=Active,default=true"`
}

// SaleReturn is the Sales-side return case: the customer request, inspection
// outcome references, and the credit/refund reference issued by Accounting.
type SaleReturn struct {
	sdk.Model `nexa:"model=sale.return"`

	Name           sdk.String                     `nexa:"required,index,string=Reference"`
	OrderID        sdk.Many2One[SaleOrder]        `nexa:"required,index,string=Original Order"`
	ShipmentLinkID sdk.Many2One[SaleShipmentLink] `nexa:"string=Shipment"`
	PartnerID      sdk.Many2One[CorePartner]      `nexa:"string=Customer"`
	ReturnDate     sdk.Date                       `nexa:"string=Date,default=today"`
	Reason         sdk.String                     `nexa:"size=200,string=Reason"`
	InspectionNote sdk.Text                       `nexa:"string=Inspection Outcome"`
	PickingRef     sdk.Many2One[sdk.Any]          `nexa:"string=Inventory Return,comodel=stock.picking"`
	CreditNoteRef  sdk.Many2One[sdk.Any]          `nexa:"string=Credit Note,comodel=account.move"`
	RefundNote     sdk.String                     `nexa:"size=120,string=Refund Reference"`
	State          sdk.Selection[ReturnState]     `nexa:"string=Status,default=draft"`
	Lines          sdk.One2Many[SaleReturnLine]   `nexa:"string=Returned Lines"`
	Notes          sdk.Text                       `nexa:"string=Notes"`
	Active         sdk.Boolean                    `nexa:"string=Active,default=true"`
}

// SaleReturnLine lists returned quantities per order line and the requested
// disposition. Restocking to sellable inventory is a per-line decision made
// after Inventory inspection, never automatic.
type SaleReturnLine struct {
	sdk.Model `nexa:"model=sale.return.line"`

	ReturnID    sdk.Many2One[SaleReturn] `nexa:"required,index,string=Return"`
	ProductID   sdk.Many2One[sdk.Any]    `nexa:"string=Product,comodel=product.product"`
	Description sdk.String               `nexa:"required,size=255,string=Description"`
	Quantity    sdk.Numeric              `nexa:"string=Returned Qty,precision=18,scale=3,default=0"`
	UoM         sdk.String               `nexa:"size=30,string=Unit"`
	Disposition sdk.String               `nexa:"size=60,string=Disposition"`
	Resealable  sdk.Boolean              `nexa:"string=Resellable,default=false"`
	Note        sdk.Text                 `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("sale_fulfillment",
		&SaleOrderReservationLink{},
		&SaleShipmentLink{},
		&SaleReturn{},
		&SaleReturnLine{},
	)
}

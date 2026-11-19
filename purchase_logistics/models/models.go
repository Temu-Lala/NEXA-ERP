package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	purchasemodels "nexa_addons/purchase/models"
	receiptmodels "nexa_addons/purchase_receipt/models"
	suppliermodels "nexa_addons/purchase_supplier/models"
)

// Purchase logistics: shipments, milestone events and freight charges.
// A shipment groups expected inbound deliveries for a purchase order (or a
// receipt when one exists) and tracks carrier milestones until arrival.
// Freight charges feed landed-cost allocation; nothing here posts stock or
// accounting — those stay with stock.picking and the AP flow.

type (
	PurchaseOrder     = purchasemodels.PurchaseOrder
	PurchaseReceipt   = receiptmodels.PurchaseReceipt
	PurchaseSupplier  = suppliermodels.PurchaseSupplier
	CoreUser          = basemodels.CoreUser
)

// PurchaseShipment is one inbound (or inter-branch) shipment.
type PurchaseShipment struct {
	sdk.Model `nexa:"model=purchase.shipment"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name         sdk.String                      `nexa:"required,string=Reference"`
	ProjectRef   sdk.Many2One[sdk.Any]           `nexa:"string=Project,comodel=project.project"`
	OrderID      sdk.Many2One[PurchaseOrder]     `nexa:"index,string=Purchase Order"`
	ReceiptID    sdk.Many2One[PurchaseReceipt]   `nexa:"index,string=Receipt"`
	SupplierID   sdk.Many2One[PurchaseSupplier]  `nexa:"index,string=Supplier"`
	Carrier      sdk.String                      `nexa:"string=Carrier"`
	Mode         sdk.Selection[ShipmentMode]     `nexa:"string=Mode,default=road,selection=road:Road,air:Air,ocean:Ocean,rail:Rail,courier:Courier"`
	State        sdk.Selection[ShipmentState]    `nexa:"string=Status,default=draft,selection=draft:Draft,booked:Booked,in_transit:In Transit,customs:Customs,arrived:Arrived,delivered:Delivered,cancelled:Cancelled"`
	TrackingNo   sdk.String                      `nexa:"string=Tracking Number"`
	Origin       sdk.String                      `nexa:"string=Origin"`
	Destination  sdk.String                      `nexa:"string=Destination"`
	ExpectedDate sdk.Date                        `nexa:"string=Expected Arrival"`
	ActualDate   sdk.Date                        `nexa:"string=Actual Arrival"`
	Incoterm     sdk.String                      `nexa:"string=Incoterm"`
	Description  sdk.Text                        `nexa:"string=Description"`
	Events       sdk.One2Many[PurchaseShipmentEvent] `nexa:"string=Tracking Events"`
	Charges      sdk.One2Many[PurchaseFreightCharge] `nexa:"string=Freight Charges"`
}

type ShipmentMode string

const (
	ShipRoad    ShipmentMode = "road"
	ShipAir     ShipmentMode = "air"
	ShipOcean   ShipmentMode = "ocean"
	ShipRail    ShipmentMode = "rail"
	ShipCourier ShipmentMode = "courier"
)

type ShipmentState string

const (
	ShipDraft     ShipmentState = "draft"
	ShipBooked    ShipmentState = "booked"
	ShipTransit   ShipmentState = "in_transit"
	ShipCustoms   ShipmentState = "customs"
	ShipArrived   ShipmentState = "arrived"
	ShipDelivered ShipmentState = "delivered"
	ShipCancelled ShipmentState = "cancelled"
)

// PurchaseShipmentEvent is one milestone on the tracking timeline.
type PurchaseShipmentEvent struct {
	sdk.Model `nexa:"model=purchase.shipment.event"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	ShipmentID sdk.Many2One[PurchaseShipment] `nexa:"required,index,ondelete=cascade,string=Shipment"`
	Kind       sdk.Selection[EventKind]        `nexa:"required,string=Event,selection=label_created:Label Created,departed:Departed,arrived_hub:Arrived at Hub,customs:Customs,customs_cleared:Customs Cleared,out_for_delivery:Out for Delivery,delivered:Delivered,exception:Exception"`
	Note       sdk.String                      `nexa:"string=Note"`
	OccurredAt sdk.DateTime                    `nexa:"string=Timestamp"`
	Location   sdk.String                      `nexa:"string=Location"`
}

type EventKind string

const (
	EvLabelCreated   EventKind = "label_created"
	EvDeparted       EventKind = "departed"
	EvArrivedHub     EventKind = "arrived_hub"
	EvCustoms        EventKind = "customs"
	EvCustomsCleared EventKind = "customs_cleared"
	EvOutForDelivery EventKind = "out_for_delivery"
	EvDelivered      EventKind = "delivered"
	EvException      EventKind = "exception"
)

// PurchaseFreightCharge is one cost component of a shipment — used for
// landed-cost allocation (freight, insurance, duty, handling, port charges).
type PurchaseFreightCharge struct {
	sdk.Model `nexa:"model=purchase.freight.charge"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	ShipmentID sdk.Many2One[PurchaseShipment] `nexa:"required,index,ondelete=cascade,string=Shipment"`
	Kind       sdk.Selection[ChargeKind]       `nexa:"required,string=Type,selection=freight:Freight,insurance:Insurance,customs_duty:Customs Duty,import_tax:Import Tax,clearing:Clearing,port:Port & Terminal,transport:Local Transport,handling:Handling,other:Other"`
	Amount     sdk.Numeric                     `nexa:"required,string=Amount,precision=18,scale=2,default=0"`
	Currency   sdk.String                      `nexa:"string=Currency,default=ETB"`
	Note       sdk.String                      `nexa:"string=Note"`
}

type ChargeKind string

const (
	ChFreight     ChargeKind = "freight"
	ChInsurance   ChargeKind = "insurance"
	ChCustomsDuty ChargeKind = "customs_duty"
	ChImportTax   ChargeKind = "import_tax"
	ChClearing    ChargeKind = "clearing"
	ChPort        ChargeKind = "port"
	ChTransport   ChargeKind = "transport"
	ChHandling    ChargeKind = "handling"
	ChOther       ChargeKind = "other"
)

func init() {
	sdk.MustRegister("purchase_logistics",
		&PurchaseShipment{},
		&PurchaseShipmentEvent{},
		&PurchaseFreightCharge{},
	)
}

package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	purchasemodels "nexa_addons/purchase/models"
)

type CoreUser = basemodels.CoreUser

// PurchaseOrder / PurchaseOrderLine are owned by the core purchase module.
type (
	PurchaseOrder     = purchasemodels.PurchaseOrder
	PurchaseOrderLine = purchasemodels.PurchaseOrderLine
)

type ScheduleState string

const (
	SchedPlanned    ScheduleState = "planned"
	SchedInProgress ScheduleState = "in_progress"
	SchedReceived   ScheduleState = "received"
	SchedCancelled  ScheduleState = "cancelled"
)

type ReceiptState string

const (
	ReceiptDraft      ReceiptState = "draft"
	ReceiptInspection ReceiptState = "inspection"
	ReceiptAccepted   ReceiptState = "accepted"
	ReceiptPartially  ReceiptState = "partially_accepted"
	ReceiptRejected   ReceiptState = "rejected"
	ReceiptService    ReceiptState = "service_accepted"
)

type ReturnState string

const (
	RetDraft      ReturnState = "draft"
	RetAuthorized ReturnState = "authorized"
	RetShipped    ReturnState = "shipped"
	RetCredited   ReturnState = "credited"
	RetClosed     ReturnState = "closed"
)

type DiscrepancyType string

const (
	DiscShort     DiscrepancyType = "short"
	DiscExcess    DiscrepancyType = "excess"
	DiscDamaged   DiscrepancyType = "damaged"
	DiscWrong     DiscrepancyType = "wrong"
	DiscNonconfom DiscrepancyType = "nonconforming"
)

type DiscrepancyState string

const (
	DiscOpen     DiscrepancyState = "open"
	DiscHold     DiscrepancyState = "on_hold"
	DiscResolved DiscrepancyState = "resolved"
	DiscClosed   DiscrepancyState = "closed"
)

type ClaimState string

const (
	ClaimOpen      ClaimState = "open"
	ClaimSubmitted ClaimState = "submitted"
	ClaimAccepted  ClaimState = "accepted"
	ClaimRejected  ClaimState = "rejected"
	ClaimSettled   ClaimState = "settled"
)

// PurchaseDeliverySchedule is a purchasing commitment for a future delivery.
// The actual stock movement is an Inventory transaction, kept separate.
type PurchaseDeliverySchedule struct {
	sdk.Model `nexa:"model=purchase.delivery.schedule"`

	Name          sdk.String                      `nexa:"required,index,string=Reference"`
	OrderID       sdk.Many2One[PurchaseOrder]     `nexa:"required,index,string=Purchase Order"`
	LineID        sdk.Many2One[PurchaseOrderLine] `nexa:"string=PO Line"`
	ProductID     sdk.Many2One[sdk.Any]           `nexa:"string=Product,comodel=product.product"`
	ScheduledDate sdk.Date                        `nexa:"string=Scheduled Date"`
	Quantity      sdk.Numeric                     `nexa:"string=Planned Qty,precision=18,scale=3,default=0"`
	UoM           sdk.String                      `nexa:"size=30,string=Unit"`
	Location      sdk.String                      `nexa:"size=200,string=Delivery Location"`
	WarehouseID   sdk.Many2One[sdk.Any]           `nexa:"string=Warehouse,comodel=stock.warehouse"`
	ReceivedDate  sdk.Date                        `nexa:"string=Received Date"`
	State         sdk.Selection[ScheduleState]    `nexa:"string=Status,default=planned"`
	Note          sdk.Text                        `nexa:"string=Note"`
}

// PurchaseReceipt coordinates the receiving of goods/services against a PO.
// For stocked items it references the authoritative Inventory receipt
// (stock.picking); it does not re-implement stock movements.
type PurchaseReceipt struct {
	sdk.Model `nexa:"model=purchase.receipt"`

	Name             sdk.String                        `nexa:"required,index,string=Reference"`
	OrderID          sdk.Many2One[PurchaseOrder]       `nexa:"required,index,string=Purchase Order"`
	ReceiptType      sdk.String                        `nexa:"size=30,string=Kind,default=goods"`
	ReceiptDate      sdk.Date                          `nexa:"string=Date,default=today"`
	Location         sdk.String                        `nexa:"size=200,string=Receiving Location"`
	WarehouseID      sdk.Many2One[sdk.Any]             `nexa:"string=Warehouse,comodel=stock.warehouse"`
	ReceiverID       sdk.Many2One[CoreUser]            `nexa:"string=Received By"`
	PickingRef       sdk.Many2One[sdk.Any]             `nexa:"string=Inventory Receipt,comodel=stock.picking"`
	SupplierNote     sdk.String                        `nexa:"size=120,string=Delivery Note Ref"`
	PackingList      sdk.String                        `nexa:"size=120,string=Packing List Ref"`
	InspectionResult sdk.Text                          `nexa:"string=Inspection Result"`
	DiscrepancyNote  sdk.Text                          `nexa:"string=Discrepancies"`
	State            sdk.Selection[ReceiptState]       `nexa:"string=Status,default=draft"`
	Lines            sdk.One2Many[PurchaseReceiptLine] `nexa:"string=Received Lines"`
	Notes            sdk.Text                          `nexa:"string=Notes"`
	Active           sdk.Boolean                       `nexa:"string=Active,default=true"`
}

// PurchaseReceiptLine records what arrived per PO line. Received quantities
// are reconciled with the authoritative Inventory receipt, not treated as an
// independent source of truth.
type PurchaseReceiptLine struct {
	sdk.Model `nexa:"model=purchase.receipt.line"`

	ReceiptID    sdk.Many2One[PurchaseReceipt]   `nexa:"required,index,string=Receipt"`
	OrderLineID  sdk.Many2One[PurchaseOrderLine] `nexa:"string=PO Line"`
	ProductID    sdk.Many2One[sdk.Any]           `nexa:"string=Product,comodel=product.product"`
	Description  sdk.String                      `nexa:"required,size=255,string=Description"`
	OrderedQty   sdk.Numeric                     `nexa:"string=Ordered,precision=18,scale=3,default=0"`
	DeliveredQty sdk.Numeric                     `nexa:"string=Delivered,precision=18,scale=3,default=0"`
	AcceptedQty  sdk.Numeric                     `nexa:"string=Accepted,precision=18,scale=3,default=0"`
	RejectedQty  sdk.Numeric                     `nexa:"string=Rejected,precision=18,scale=3,default=0"`
	DamagedQty   sdk.Numeric                     `nexa:"string=Damaged,precision=18,scale=3,default=0"`
	Lot          sdk.String                      `nexa:"size=80,string=Lot"`
	SerialNumber sdk.String                      `nexa:"size=80,string=Serial"`
	ExpiryDate   sdk.Date                        `nexa:"string=Expiry"`
	Inspection   sdk.String                      `nexa:"size=200,string=Inspection"`
	Note         sdk.Text                        `nexa:"string=Note"`
}

// PurchaseSupplierReturn coordinates a return to supplier and the open claim.
// Stock returns are executed by Inventory; credit notes by Accounting.
type PurchaseSupplierReturn struct {
	sdk.Model `nexa:"model=purchase.supplier.return"`

	Name          sdk.String                               `nexa:"required,index,string=Reference"`
	OrderID       sdk.Many2One[PurchaseOrder]              `nexa:"required,index,string=Purchase Order"`
	ReceiptID     sdk.Many2One[PurchaseReceipt]            `nexa:"string=Original Receipt"`
	PartnerID     sdk.Many2One[sdk.Any]                    `nexa:"string=Supplier,comodel=core.partner"`
	ReturnDate    sdk.Date                                 `nexa:"string=Date,default=today"`
	ReturnAuthRef sdk.String                               `nexa:"size=120,string=Return Authorization"`
	PickingRef    sdk.Many2One[sdk.Any]                    `nexa:"string=Stock Return,comodel=stock.picking"`
	CreditNoteRef sdk.Many2One[sdk.Any]                    `nexa:"string=Credit Note,comodel=account.move"`
	ClaimStatus   sdk.String                               `nexa:"size=60,string=Claim Status"`
	State         sdk.Selection[ReturnState]               `nexa:"string=Status,default=draft"`
	Lines         sdk.One2Many[PurchaseSupplierReturnLine] `nexa:"string=Returned Lines"`
	Reason        sdk.Text                                 `nexa:"string=Reason"`
	Notes         sdk.Text                                 `nexa:"string=Notes"`
	Active        sdk.Boolean                              `nexa:"string=Active,default=true"`
}

// PurchaseSupplierReturnLine lists returned quantities and reasons.
type PurchaseSupplierReturnLine struct {
	sdk.Model `nexa:"model=purchase.supplier.return.line"`

	ReturnID      sdk.Many2One[PurchaseSupplierReturn] `nexa:"required,index,string=Return"`
	ReceiptLineID sdk.Many2One[PurchaseReceiptLine]    `nexa:"string=Receipt Line"`
	ProductID     sdk.Many2One[sdk.Any]                `nexa:"string=Product,comodel=product.product"`
	Description   sdk.String                           `nexa:"required,size=255,string=Description"`
	Quantity      sdk.Numeric                          `nexa:"string=Returned Qty,precision=18,scale=3,default=0"`
	UoM           sdk.String                           `nexa:"size=30,string=Unit"`
	Reason        sdk.String                           `nexa:"size=200,string=Reason"`
	Disposition   sdk.String                           `nexa:"size=60,string=Disposition"`
	Note          sdk.Text                             `nexa:"string=Note"`
}

// PurchaseDiscrepancyCase records a delivery problem (short/excess/damaged/
// wrong/nonconforming) against a receipt line. Physical holds remain owned by
// Inventory; this record coordinates the commercial disposition and evidence.
type PurchaseDiscrepancyCase struct {
	sdk.Model `nexa:"model=purchase.discrepancy.case"`

	Name          sdk.String                          `nexa:"required,index,string=Reference"`
	ReceiptID     sdk.Many2One[PurchaseReceipt]       `nexa:"required,index,string=Receipt"`
	ReceiptLineID sdk.Many2One[PurchaseReceiptLine]   `nexa:"string=Receipt Line"`
	OrderID       sdk.Many2One[PurchaseOrder]         `nexa:"string=Purchase Order"`
	ProductID     sdk.Many2One[sdk.Any]               `nexa:"string=Product,comodel=product.product"`
	IssueType     sdk.Selection[DiscrepancyType]      `nexa:"string=Issue,default=short"`
	Quantity      sdk.Numeric                         `nexa:"string=Affected Qty,precision=18,scale=3,default=0"`
	Lot           sdk.String                          `nexa:"size=80,string=Lot"`
	SerialNumber  sdk.String                          `nexa:"size=80,string=Serial"`
	Evidence      sdk.Text                            `nexa:"string=Evidence"`
	OnHold        sdk.Boolean                         `nexa:"string=Hold In Inventory,default=false"`
	Disposition   sdk.String                          `nexa:"size=60,string=Disposition"`
	State         sdk.Selection[DiscrepancyState]     `nexa:"string=Status,default=open"`
	ClaimID       sdk.Many2One[PurchaseSupplierClaim] `nexa:"string=Related Claim"`
	Resolution    sdk.Text                            `nexa:"string=Resolution"`
	History       sdk.Text                            `nexa:"string=History"`
	Active        sdk.Boolean                         `nexa:"string=Active,default=true"`
}

// PurchaseSupplierClaim is the commercial claim raised to a supplier for a
// discrepancy (replacement, credit or return). Credit notes are owned by
// Accounting and only referenced here.
type PurchaseSupplierClaim struct {
	sdk.Model `nexa:"model=purchase.supplier.claim"`

	Name           sdk.String                    `nexa:"required,index,string=Reference"`
	PartnerID      sdk.Many2One[sdk.Any]         `nexa:"required,string=Supplier,comodel=core.partner"`
	OrderID        sdk.Many2One[PurchaseOrder]   `nexa:"string=Purchase Order"`
	ReceiptID      sdk.Many2One[PurchaseReceipt] `nexa:"string=Receipt"`
	ClaimType      sdk.String                    `nexa:"size=60,string=Claim Type"`
	AmountClaimed  sdk.Numeric                   `nexa:"string=Amount Claimed,precision=18,scale=2,default=0"`
	State          sdk.Selection[ClaimState]     `nexa:"string=Status,default=open"`
	OpenedOn       sdk.Date                      `nexa:"string=Opened,default=today"`
	ResolvedOn     sdk.Date                      `nexa:"string=Resolved"`
	ResolutionNote sdk.Text                      `nexa:"string=Resolution"`
	Active         sdk.Boolean                   `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("purchase_receipt",
		&PurchaseDeliverySchedule{},
		&PurchaseReceipt{},
		&PurchaseReceiptLine{},
		&PurchaseSupplierReturn{},
		&PurchaseSupplierReturnLine{},
		&PurchaseDiscrepancyCase{},
		&PurchaseSupplierClaim{},
	)
}

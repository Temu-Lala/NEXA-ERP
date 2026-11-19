package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Point of Sale: cashier sessions (shifts), orders and payment tenders.
// A session is one cashier's shift with opening/closing cash; orders belong
// to a session; tenders record how each order was paid (split payments).
// Stock deduction goes through stock.picking in a later phase.

type CoreUser = basemodels.CoreUser
type CoreCompany = basemodels.CoreCompany

// PosSession is a cashier shift.
type PosSession struct {
	sdk.Model `nexa:"model=pos.session"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name         sdk.String                  `nexa:"required,string=Session"`
	UserID       sdk.Many2One[CoreUser]      `nexa:"required,index,string=Cashier"`
	ConfigName   sdk.String                  `nexa:"string=Register/Config"`
	State        sdk.Selection[SessionState] `nexa:"string=Status,default=open,selection=open:Open,closing:Closing,closed:Closed"`
	StartAt      sdk.DateTime                `nexa:"string=Opened At"`
	StopAt       sdk.DateTime                `nexa:"string=Closed At"`
	OpeningCash  sdk.Numeric                 `nexa:"string=Opening Cash,precision=18,scale=2,default=0"`
	ClosingCash  sdk.Numeric                 `nexa:"string=Counted Cash,precision=18,scale=2,default=0"`
	ExpectedCash sdk.Numeric                 `nexa:"string=Expected Cash,precision=18,scale=2,default=0"`
	CashVariance sdk.Numeric                 `nexa:"string=Variance,precision=18,scale=2,default=0"`
	Orders       sdk.One2Many[PosOrder]      `nexa:"string=Orders"`
}

type SessionState string

const (
	SessOpen    SessionState = "open"
	SessClosing SessionState = "closing"
	SessClosed  SessionState = "closed"
)

// PosOrder is one receipt.
type PosOrder struct {
	sdk.Model `nexa:"model=pos.order"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name        sdk.String                 `nexa:"required,string=Receipt"`
	SessionID   sdk.Many2One[PosSession]   `nexa:"required,index,string=Session"`
	PartnerID   sdk.Many2One[sdk.Any]      `nexa:"string=Customer,comodel=core.partner"`
	UserID      sdk.Many2One[CoreUser]     `nexa:"string=Cashier"`
	State       sdk.Selection[OrderState]  `nexa:"string=Status,default=draft,selection=draft:Draft,held:Held,layaway:Layaway,paid:Paid,done:Done,void:Void,refunded:Refunded"`
	TableID     sdk.Many2One[sdk.Any]      `nexa:"index,string=Table,comodel=pos.table"`
	Course      sdk.Integer                `nexa:"string=Course,default=1"`
	DateOrder   sdk.DateTime               `nexa:"string=Date"`
	AmountTotal sdk.Numeric                `nexa:"string=Total,precision=18,scale=2,default=0"`
	AmountTax   sdk.Numeric                `nexa:"string=Tax,precision=18,scale=2,default=0"`
	Discount    sdk.Numeric                `nexa:"string=Discount,precision=18,scale=2,default=0"`
	Note        sdk.Text                   `nexa:"string=Note"`
	Lines       sdk.One2Many[PosOrderLine] `nexa:"string=Lines"`
	Payments    sdk.One2Many[PosPayment]   `nexa:"string=Payments"`
}

type OrderState string

const (
	OrdDraft    OrderState = "draft"
	OrdPaid     OrderState = "paid"
	OrdDone     OrderState = "done"
	OrdVoid     OrderState = "void"
	OrdRefunded OrderState = "refunded"
)

// PosOrderLine is one sold item.
type PosOrderLine struct {
	sdk.Model `nexa:"model=pos.order.line"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	OrderID       sdk.Many2One[PosOrder] `nexa:"required,index,ondelete=cascade,string=Order"`
	ProductID     sdk.Many2One[sdk.Any]  `nexa:"required,index,string=Product,comodel=product.product"`
	Name          sdk.String             `nexa:"required,string=Description"`
	Qty           sdk.Numeric            `nexa:"string=Quantity,precision=18,scale=3,default=1"`
	PriceUnit     sdk.Numeric            `nexa:"string=Unit Price,precision=18,scale=2,default=0"`
	DiscountPct   sdk.Numeric            `nexa:"string=Discount %,precision=18,scale=2,default=0"`
	PriceSubtotal sdk.Numeric            `nexa:"string=Subtotal,precision=18,scale=2,default=0"`
	Note          sdk.String             `nexa:"string=Note"`
	Serial        sdk.String             `nexa:"string=Serial/Batch"`
	Course        sdk.Integer            `nexa:"string=Course,default=1"`
	SentToKitchen sdk.Boolean            `nexa:"string=Sent to Kitchen,default=false"`
	Prepared      sdk.Boolean            `nexa:"string=Prepared,default=false"`
	Sequence      sdk.Integer            `nexa:"string=Sequence,default=10"`
}

// PosPayment is one tender against an order — split payments supported.
type PosPayment struct {
	sdk.Model `nexa:"model=pos.payment"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	OrderID   sdk.Many2One[PosOrder]   `nexa:"required,index,ondelete=cascade,string=Order"`
	Method    sdk.Selection[PayMethod] `nexa:"required,string=Method,selection=cash:Cash,card:Card,mobile:Mobile,wallet:Wallet,credit:Store Credit,other:Other"`
	Amount    sdk.Numeric              `nexa:"required,string=Amount,precision=18,scale=2,default=0"`
	Reference sdk.String               `nexa:"string=Reference"`
	Note      sdk.String               `nexa:"string=Note"`
}

type PayMethod string

const (
	PayCash   PayMethod = "cash"
	PayCard   PayMethod = "card"
	PayMobile PayMethod = "mobile"
	PayWallet PayMethod = "wallet"
	PayCredit PayMethod = "credit"
	PayOther  PayMethod = "other"
)

func init() {
	sdk.MustRegister("pos",
		&PosSession{},
		&PosOrder{},
		&PosOrderLine{},
		&PosPayment{},
		&PosReturn{},
		&PosStore{},
		&PosRegister{},
		&PosPromotion{},
		&PosGiftCard{},
		&PosCashMovement{},
		&PosDevice{},
		&PosSyncEvent{},
		&PosPriceList{},
		&PosPriceListItem{},
		&PosLoyaltyTxn{},
		&PosAuditEvent{},
		&PosTable{},
		&PosPrinter{},
	)
}

// PosReturn is a return/exchange against a paid order.
type PosReturn struct {
	sdk.Model `nexa:"model=pos.return"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name          sdk.String               `nexa:"required,string=Return Reference"`
	OrderID       sdk.Many2One[PosOrder]   `nexa:"required,index,string=Original Order"`
	SessionID     sdk.Many2One[PosSession] `nexa:"string=Session"`
	Reason        sdk.String               `nexa:"string=Reason"`
	AmountTotal   sdk.Numeric              `nexa:"string=Refund Total,precision=18,scale=2,default=0"`
	State         sdk.Selection[RetState]  `nexa:"string=Status,default=requested,selection=requested:Requested,approved:Approved,refunded:Refunded,rejected:Rejected"`
	RefundMethod  sdk.Selection[PayMethod] `nexa:"string=Refund Method,selection=cash:Cash,card:Card,mobile:Mobile,wallet:Wallet,credit:Store Credit,other:Other"`
	RequestedByID sdk.Many2One[CoreUser]   `nexa:"string=Requested By"`
	ApprovedByID  sdk.Many2One[CoreUser]   `nexa:"string=Approved By"`
	Note          sdk.Text                 `nexa:"string=Note"`
}

type RetState string

const (
	RetRequested RetState = "requested"
	RetApproved  RetState = "approved"
	RetRefunded  RetState = "refunded"
	RetRejected  RetState = "rejected"
)

// ---------------------------------------------------------------------------
// Advanced POS: stores/registers, promotions, gift cards, cash movements.
// ---------------------------------------------------------------------------

// PosStore is a retail branch.
type PosStore struct {
	sdk.Model `nexa:"model=pos.store"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name      sdk.String                `nexa:"required,string=Store"`
	Code      sdk.String                `nexa:"string=Store Code"`
	Address   sdk.Text                  `nexa:"string=Address"`
	Phone     sdk.String                `nexa:"string=Phone"`
	ManagerID sdk.Many2One[CoreUser]    `nexa:"string=Manager"`
	Timezone  sdk.String                `nexa:"string=Timezone,default=UTC"`
	State     sdk.Selection[StoreState] `nexa:"string=Status,default=active,selection=active:Active,inactive:Inactive"`
	Registers sdk.One2Many[PosRegister] `nexa:"string=Registers"`
}

type StoreState string

const (
	StoreActive   StoreState = "active"
	StoreInactive StoreState = "inactive"
)

// PosRegister is a physical/virtual POS terminal in a store.
type PosRegister struct {
	sdk.Model `nexa:"model=pos.register"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name       sdk.String               `nexa:"required,string=Register"`
	Code       sdk.String               `nexa:"string=Register ID"`
	StoreID    sdk.Many2One[PosStore]   `nexa:"required,index,string=Store"`
	DeviceID   sdk.String               `nexa:"string=Device/Terminal ID"`
	CashDrawer sdk.String               `nexa:"string=Cash Drawer Config"`
	State      sdk.Selection[RegState]  `nexa:"string=Status,default=active,selection=active:Active,disabled:Disabled"`
	Sessions   sdk.One2Many[PosSession] `nexa:"string=Sessions"`
}

type RegState string

const (
	RegActive   RegState = "active"
	RegDisabled RegState = "disabled"
)

// PosPromotion is a discount rule applied at checkout.
type PosPromotion struct {
	sdk.Model `nexa:"model=pos.promotion"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name       sdk.String               `nexa:"required,string=Promotion"`
	Code       sdk.String               `nexa:"string=Coupon Code"`
	Kind       sdk.Selection[PromoKind] `nexa:"string=Type,default=percent,selection=percent:Percent,fixed:Fixed_amount,buy_x_get_y:Buy X Get Y,coupon:Coupon"`
	Value      sdk.Numeric              `nexa:"string=Value,precision=18,scale=2,default=0"`
	MinBasket  sdk.Numeric              `nexa:"string=Min Basket,precision=18,scale=2,default=0"`
	BuyQty     sdk.Integer              `nexa:"string=Buy Qty,default=0"`
	GetQty     sdk.Integer              `nexa:"string=Get Qty,default=0"`
	ProductID  sdk.Many2One[sdk.Any]    `nexa:"string=Product,comodel=product.product"`
	CategoryID sdk.Many2One[sdk.Any]    `nexa:"string=Category,comodel=product.category"`
	StartAt    sdk.DateTime             `nexa:"string=Starts"`
	EndAt      sdk.DateTime             `nexa:"string=Ends"`
	UsageLimit sdk.Integer              `nexa:"string=Usage Limit,default=0"`
	UsedCount  sdk.Integer              `nexa:"string=Used,default=0"`
	Active     sdk.Boolean              `nexa:"string=Active,default=true"`
}

type PromoKind string

const (
	PromoPercent PromoKind = "percent"
	PromoFixed   PromoKind = "fixed"
	PromoBogo    PromoKind = "buy_x_get_y"
	PromoCoupon  PromoKind = "coupon"
)

// PosGiftCard is a stored-value card.
type PosGiftCard struct {
	sdk.Model `nexa:"model=pos.giftcard"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Code       sdk.String             `nexa:"required,index,string=Card Code"`
	Initial    sdk.Numeric            `nexa:"string=Initial Value,precision=18,scale=2,default=0"`
	Balance    sdk.Numeric            `nexa:"string=Balance,precision=18,scale=2,default=0"`
	State      sdk.Selection[GCState] `nexa:"string=Status,default=active,selection=active:Active,depleted:Depleted,cancelled:Cancelled"`
	IssuedToID sdk.Many2One[sdk.Any]  `nexa:"string=Issued To,comodel=core.partner"`
	ExpiresAt  sdk.Date               `nexa:"string=Expires"`
}

type GCState string

const (
	GCActive    GCState = "active"
	GCDepleted  GCState = "depleted"
	GCCancelled GCState = "cancelled"
)

// PosCashMovement is a cash-in/out event on a session (drops, petty cash).
type PosCashMovement struct {
	sdk.Model `nexa:"model=pos.cash.movement"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	SessionID sdk.Many2One[PosSession] `nexa:"required,index,string=Session"`
	Kind      sdk.Selection[CMKind]    `nexa:"string=Type,default=in,selection=in:Cash In,out:Cash Out"`
	Amount    sdk.Numeric              `nexa:"string=Amount,precision=18,scale=2,default=0"`
	Reason    sdk.String               `nexa:"string=Reason"`
	UserID    sdk.Many2One[CoreUser]   `nexa:"string=By"`
	At        sdk.DateTime             `nexa:"string=At"`
}

type CMKind string

const (
	CMIn  CMKind = "in"
	CMOut CMKind = "out"
)

// ---------------------------------------------------------------------------
// Offline-first sync (spec §10): registered devices push queued transactions;
// every event carries a globally unique client_txn_id for idempotent apply.
// ---------------------------------------------------------------------------

// PosDevice is a registered POS terminal allowed to sync offline sales.
type PosDevice struct {
	sdk.Model `nexa:"model=pos.device"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name       sdk.String                 `nexa:"required,string=Device"`
	DeviceID   sdk.String                 `nexa:"required,index,string=Device ID"`
	StoreID    sdk.Many2One[PosStore]     `nexa:"string=Store"`
	RegisterID sdk.Many2One[PosRegister]  `nexa:"string=Register"`
	UserID     sdk.Many2One[CoreUser]     `nexa:"string=Assigned Cashier"`
	LastSyncAt sdk.DateTime               `nexa:"string=Last Sync"`
	Active     sdk.Boolean                `nexa:"string=Active,default=true"`
	Events     sdk.One2Many[PosSyncEvent] `nexa:"string=Sync Events"`
}

// PosSyncEvent is one offline transaction queued for server application.
// client_txn_id is the terminal-generated UUID — the idempotency key.
type PosSyncEvent struct {
	sdk.Model `nexa:"model=pos.sync.event"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	DeviceID      sdk.Many2One[PosDevice]  `nexa:"required,index,string=Device"`
	ClientTxnID   sdk.String               `nexa:"required,index,string=Client Txn ID"`
	Kind          sdk.Selection[SyncKind]  `nexa:"required,string=Type,selection=sale:Sale,payment:Payment,return:Return"`
	Payload       sdk.Text                 `nexa:"string=Payload (JSON lines/payments)"`
	State         sdk.Selection[SyncState] `nexa:"string=Status,default=pending,selection=pending:Pending,applied:Applied,conflict:Conflict,rejected:Rejected"`
	ServerOrderID sdk.Many2One[sdk.Any]    `nexa:"string=Server Order,comodel=pos.order"`
	Error         sdk.String               `nexa:"string=Error"`
	DeviceTime    sdk.DateTime             `nexa:"string=Device Time"`
	ReceivedAt    sdk.DateTime             `nexa:"string=Received At"`
	AppliedAt     sdk.DateTime             `nexa:"string=Applied At"`
}

type SyncKind string

const (
	SyncSale    SyncKind = "sale"
	SyncPayment SyncKind = "payment"
	SyncReturn  SyncKind = "return"
)

type SyncState string

const (
	SyncPending  SyncState = "pending"
	SyncApplied  SyncState = "applied"
	SyncConflict SyncState = "conflict"
	SyncRejected SyncState = "rejected"
)

// ---------------------------------------------------------------------------
// Price lists (spec §4.1): branch/customer/segment pricing rules.
// ---------------------------------------------------------------------------

// PosPriceList is a named price list (retail, wholesale, VIP, branch X...).
type PosPriceList struct {
	sdk.Model `nexa:"model=pos.price.list"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name       sdk.String                     `nexa:"required,string=Price List"`
	Code       sdk.String                     `nexa:"string=Code"`
	Kind       sdk.Selection[PLKind]          `nexa:"string=Type,default=percent,selection=percent:Percent,fixed:Fixed_price"`
	StoreID    sdk.Many2One[PosStore]         `nexa:"string=Store (blank=all)"`
	PartnerID  sdk.Many2One[sdk.Any]          `nexa:"string=Customer,comodel=core.partner"`
	CurrencyID sdk.Many2One[sdk.Any]          `nexa:"string=Currency,comodel=currency.currency"`
	Active     sdk.Boolean                    `nexa:"string=Active,default=true"`
	Items      sdk.One2Many[PosPriceListItem] `nexa:"string=Items"`
}

type PLKind string

const (
	PLPercent PLKind = "percent"
	PLFixed   PLKind = "fixed"
)

// PosPriceListItem is one product/category price rule inside a list.
type PosPriceListItem struct {
	sdk.Model `nexa:"model=pos.price.list.item"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	PriceListID sdk.Many2One[PosPriceList] `nexa:"required,index,ondelete=cascade,string=Price List"`
	ProductID   sdk.Many2One[sdk.Any]      `nexa:"index,string=Product,comodel=product.product"`
	CategoryID  sdk.Many2One[sdk.Any]      `nexa:"index,string=Category,comodel=product.category"`
	MinQty      sdk.Numeric                `nexa:"string=Min Qty,precision=18,scale=3,default=1"`
	Price       sdk.Numeric                `nexa:"string=Price,precision=18,scale=2,default=0"`
	DiscountPct sdk.Numeric                `nexa:"string=Discount %,precision=18,scale=2,default=0"`
}

// ---------------------------------------------------------------------------
// Loyalty ledger + audit events (spec §8, §15).
// ---------------------------------------------------------------------------

// PosLoyaltyTxn is an immutable points ledger entry — never overwrite balance.
type PosLoyaltyTxn struct {
	sdk.Model `nexa:"model=pos.loyalty.txn"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	PartnerID    sdk.Many2One[sdk.Any]  `nexa:"required,index,string=Customer,comodel=core.partner"`
	OrderID      sdk.Many2One[PosOrder] `nexa:"index,string=Order"`
	Kind         sdk.Selection[LoyKind] `nexa:"required,string=Type,selection=earn:Earn,redeem:Redeem,expire:Expire,adjust:Adjust"`
	Points       sdk.Integer            `nexa:"string=Points (+/-),default=0"`
	BalanceAfter sdk.Integer            `nexa:"string=Balance After,default=0"`
	Note         sdk.String             `nexa:"string=Note"`
	UserID       sdk.Many2One[CoreUser] `nexa:"string=By"`
	At           sdk.DateTime           `nexa:"string=At"`
}

type LoyKind string

const (
	LoyEarn   LoyKind = "earn"
	LoyRedeem LoyKind = "redeem"
	LoyExpire LoyKind = "expire"
	LoyAdjust LoyKind = "adjust"
)

// PosAuditEvent is an immutable record of sensitive POS actions.
type PosAuditEvent struct {
	sdk.Model `nexa:"model=pos.audit.event"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	ModelName sdk.String             `nexa:"required,string=Model"`
	RecordID  sdk.Integer            `nexa:"string=Record ID"`
	Action    sdk.String             `nexa:"required,string=Action"`
	UserID    sdk.Many2One[CoreUser] `nexa:"string=User"`
	Detail    sdk.Text               `nexa:"string=Detail"`
	At        sdk.DateTime           `nexa:"string=At"`
}

// PosTable is a restaurant table mapped to a store (spec: restaurant mode).
type PosTable struct {
	sdk.Model `nexa:"model=pos.table"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name    sdk.String                `nexa:"required,string=Table"`
	StoreID sdk.Many2One[sdk.Any]     `nexa:"index,string=Store,comodel=pos.store"`
	Seats   sdk.Integer               `nexa:"string=Seats,default=4"`
	Section sdk.String                `nexa:"string=Section"`
	State   sdk.Selection[TableState] `nexa:"string=Status,default=free,selection=free:Free,occupied:Occupied,billed:Billed"`
	Active  sdk.Boolean               `nexa:"string=Active,default=true"`
}

type TableState string

const (
	TableFree     TableState = "free"
	TableOccupied TableState = "occupied"
	TableBilled   TableState = "billed"
)

// PosPrinter is a network receipt printer (ESC/POS over TCP :9100).
type PosPrinter struct {
	sdk.Model `nexa:"model=pos.printer"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name      sdk.String            `nexa:"required,string=Printer"`
	IpAddress sdk.String            `nexa:"string=IP Address"`
	Port      sdk.Integer           `nexa:"string=Port,default=9100"`
	StoreID   sdk.Many2One[sdk.Any] `nexa:"string=Store,comodel=pos.store"`
	Active    sdk.Boolean           `nexa:"string=Active,default=true"`
}

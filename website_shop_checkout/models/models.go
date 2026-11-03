package models

import (
	"nexa/core/sdk"
	shopmodels "nexa_addons/website_shop/models"
	cartmodels "nexa_addons/website_shop_cart/models"
)

// E-commerce checkout, payment and order-projection companion.
// Checkout converts cart intent into a validated purchase. The authoritative
// confirmed order lives in Sales (sale.order); ecom.order is the storefront
// projection/attribution reference to it. Payment status is tracked separately
// from order/fulfillment/return status so state combinations stay meaningful.

type (
	EcomStore   = shopmodels.EcomStore
	EcomCart    = cartmodels.EcomCart
	CorePartner = shopmodels.CorePartner
	CoreCompany = shopmodels.CoreCompany
)

type CheckoutState string

const (
	ChkDraft     CheckoutState = "draft"
	ChkReviewing CheckoutState = "reviewing"
	ChkPlaced    CheckoutState = "placed"
	ChkCancelled CheckoutState = "cancelled"
	ChkExpired   CheckoutState = "expired"
)

type PaymentMethodSel string

const (
	PayCard   PaymentMethodSel = "card"
	PayBank   PaymentMethodSel = "bank_transfer"
	PayMobile PaymentMethodSel = "mobile_money"
	PayCOD    PaymentMethodSel = "cod"
	PayPickup PaymentMethodSel = "pickup"
	PayCredit PaymentMethodSel = "credit"
)

type ShipMethod string

const (
	ShipFlat        ShipMethod = "flat"
	ShipWeight      ShipMethod = "weight"
	ShipOrderValue  ShipMethod = "order_value"
	ShipPickup      ShipMethod = "pickup"
	ShipCarrierFree ShipMethod = "carrier_free"
)

type PaymentProvider string

const (
	ProvStripe   PaymentProvider = "stripe"
	ProvPaypal   PaymentProvider = "paypal"
	ProvTelebirr PaymentProvider = "telebirr"
	ProvCbeat    PaymentProvider = "cbeat"
	ProvBank     PaymentProvider = "bank_transfer"
	ProvCOD      PaymentProvider = "cod"
)

type PaymentStatus string

const (
	PmtPending    PaymentStatus = "pending"
	PmtAuthorized PaymentStatus = "authorized"
	PmtCaptured   PaymentStatus = "captured"
	PmtFailed     PaymentStatus = "failed"
	PmtRefunded   PaymentStatus = "refunded"
	PmtCancelled  PaymentStatus = "cancelled"
)

type OrderState string

const (
	OrdPending    OrderState = "pending"
	OrdConfirmed  OrderState = "confirmed"
	OrdProcessing OrderState = "processing"
	OrdFulfilled  OrderState = "fulfilled"
	OrdCompleted  OrderState = "completed"
	OrdCancelled  OrderState = "cancelled"
)

type OrderPayStatus string

const (
	OPSNotPaid    OrderPayStatus = "not_paid"
	OPSAwaiting   OrderPayStatus = "awaiting"
	OPSAuthorized OrderPayStatus = "authorized"
	OPSPaid       OrderPayStatus = "paid"
	OPSPartialRef OrderPayStatus = "partially_refunded"
	OPSRefunded   OrderPayStatus = "refunded"
)

type OrderFulfilStatus string

const (
	OFSNone     OrderFulfilStatus = "none"
	OFSPartial  OrderFulfilStatus = "partial"
	OFSComplete OrderFulfilStatus = "complete"
)

type OrderReturnStatus string

const (
	ORSNone      OrderReturnStatus = "none"
	ORSRequested OrderReturnStatus = "requested"
	ORSPartial   OrderReturnStatus = "partial"
	ORSComplete  OrderReturnStatus = "complete"
)

// EcomCheckout is a checkout session derived from a cart.
type EcomCheckout struct {
	sdk.Model `nexa:"model=ecom.checkout"`

	Name             sdk.String                       `nexa:"size=60,string=Reference"`
	StoreID          sdk.Many2One[EcomStore]          `nexa:"required,string=Store"`
	CartID           sdk.Many2One[EcomCart]           `nexa:"string=Cart"`
	PartnerID        sdk.Many2One[CorePartner]        `nexa:"string=Customer"`
	Email            sdk.String                       `nexa:"size=120,string=Email"`
	State            sdk.Selection[CheckoutState]     `nexa:"string=State,default=draft"`
	ShippingMethodID sdk.Many2One[EcomShippingMethod] `nexa:"string=Shipping Method"`
	PaymentMethod    sdk.Selection[PaymentMethodSel]  `nexa:"string=Payment Method,default=card"`
	Subtotal         sdk.Numeric                      `nexa:"string=Subtotal,precision=18,scale=2,default=0"`
	DiscountTotal    sdk.Numeric                      `nexa:"string=Discount,precision=18,scale=2,default=0"`
	TaxTotal         sdk.Numeric                      `nexa:"string=Tax,precision=18,scale=2,default=0"`
	ShippingCost     sdk.Numeric                      `nexa:"string=Shipping,precision=18,scale=2,default=0"`
	GrandTotal       sdk.Numeric                      `nexa:"string=Total,precision=18,scale=2,default=0"`
	TermsAccepted    sdk.Boolean                      `nexa:"string=Terms Accepted,default=false"`
	IdempotencyKey   sdk.String                       `nexa:"size=64,string=Idempotency Key"`
	OrderID          sdk.Many2One[EcomOrder]          `nexa:"string=Order"`
	PlacedAt         sdk.DateTime                     `nexa:"string=Placed At"`
	Note             sdk.Text                         `nexa:"string=Note"`
}

// EcomShippingMethod is a configurable delivery option offered at checkout.
type EcomShippingMethod struct {
	sdk.Model `nexa:"model=ecom.shipping.method"`

	StoreID       sdk.Many2One[EcomStore]   `nexa:"string=Store"`
	Name          sdk.String                `nexa:"required,size=120,string=Name"`
	Method        sdk.Selection[ShipMethod] `nexa:"string=Method,default=flat"`
	Price         sdk.Numeric               `nexa:"string=Price,precision=18,scale=2,default=0"`
	FreeThreshold sdk.Numeric               `nexa:"string=Free Over,precision=18,scale=2,default=0"`
	MinDays       sdk.Integer               `nexa:"string=Min Days,default=0"`
	MaxDays       sdk.Integer               `nexa:"string=Max Days,default=0"`
	Active        sdk.Boolean               `nexa:"string=Active,default=true"`
	Description   sdk.Text                  `nexa:"string=Description"`
}

// EcomPaymentSession tracks a payment-provider attempt for a checkout.
type EcomPaymentSession struct {
	sdk.Model `nexa:"model=ecom.payment.session"`

	CheckoutID      sdk.Many2One[EcomCheckout]     `nexa:"required,index,string=Checkout"`
	StoreID         sdk.Many2One[EcomStore]        `nexa:"string=Store"`
	Provider        sdk.Selection[PaymentProvider] `nexa:"string=Provider,default=stripe"`
	ProviderRef     sdk.String                     `nexa:"size=120,string=Provider Ref"`
	Amount          sdk.Numeric                    `nexa:"string=Amount,precision=18,scale=2,default=0"`
	CurrencyID      sdk.Many2One[sdk.Any]          `nexa:"string=Currency,comodel=core.currency"`
	Status          sdk.Selection[PaymentStatus]   `nexa:"string=Status,default=pending"`
	AttemptCount    sdk.Integer                    `nexa:"string=Attempts,default=0"`
	StartedAt       sdk.DateTime                   `nexa:"string=Started At"`
	VerifiedAt      sdk.DateTime                   `nexa:"string=Verified At"`
	WebhookVerified sdk.Boolean                    `nexa:"string=Webhook Verified,default=false"`
	Note            sdk.Text                       `nexa:"string=Note"`
}

// EcomOrder is the storefront projection/attribution reference to a Sales order.
type EcomOrder struct {
	sdk.Model `nexa:"model=ecom.order"`

	Name              sdk.String                       `nexa:"required,size=60,string=Order Reference"`
	Number            sdk.String                       `nexa:"index,size=40,string=Order Number"`
	StoreID           sdk.Many2One[EcomStore]          `nexa:"required,index,string=Store"`
	SaleOrderID       sdk.Many2One[sdk.Any]            `nexa:"string=Sales Order,comodel=sale.order"`
	CheckoutID        sdk.Many2One[EcomCheckout]       `nexa:"string=Checkout"`
	PartnerID         sdk.Many2One[CorePartner]        `nexa:"index,string=Customer"`
	CurrencyID        sdk.Many2One[sdk.Any]            `nexa:"string=Currency,comodel=core.currency"`
	State             sdk.Selection[OrderState]        `nexa:"string=Order Status,default=pending"`
	PaymentStatus     sdk.Selection[OrderPayStatus]    `nexa:"string=Payment Status,default=not_paid"`
	FulfillmentStatus sdk.Selection[OrderFulfilStatus] `nexa:"string=Fulfillment,default=none"`
	ReturnStatus      sdk.Selection[OrderReturnStatus] `nexa:"string=Return Status,default=none"`
	AmountUntaxed     sdk.Numeric                      `nexa:"string=Subtotal,precision=18,scale=2,default=0"`
	AmountTax         sdk.Numeric                      `nexa:"string=Tax,precision=18,scale=2,default=0"`
	AmountTotal       sdk.Numeric                      `nexa:"string=Total,precision=18,scale=2,default=0"`
	AttributionSource sdk.String                       `nexa:"size=120,string=Source"`
	CampaignName      sdk.String                       `nexa:"size=120,string=Campaign"`
	Note              sdk.Text                         `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("website_shop_checkout",
		&EcomCheckout{},
		&EcomShippingMethod{},
		&EcomPaymentSession{},
		&EcomOrder{},
	)
}

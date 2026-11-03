package models

import (
	"nexa/core/sdk"
	shopmodels "nexa_addons/website_shop/models"
)

// E-commerce shopping cart companion.
// The cart captures purchase intent (guest or registered) before an order is
// confirmed. It is NOT the authoritative Sales order. Prices/totals are stored
// as session snapshots; the backend is the only trusted source of amounts.

type (
	EcomStore   = shopmodels.EcomStore
	CorePartner = shopmodels.CorePartner
	CoreCompany = shopmodels.CoreCompany
)

type CartState string

const (
	CartOpen      CartState = "open"
	CartConverted CartState = "converted"
	CartMerged    CartState = "merged"
	CartAbandoned CartState = "abandoned"
	CartExpired   CartState = "expired"
)

type AddressType string

const (
	AddrBilling  AddressType = "billing"
	AddrShipping AddressType = "shipping"
)

type CartEventType string

const (
	CevCreated     CartEventType = "created"
	CevUpdated     CartEventType = "updated"
	CevPriceChange CartEventType = "price_change"
	CevAbandoned   CartEventType = "abandoned"
	CevRecovered   CartEventType = "recovered"
	CevConverted   CartEventType = "converted"
)

// EcomCart is a shopping cart owned by a guest session or a customer.
type EcomCart struct {
	sdk.Model `nexa:"model=ecom.cart"`

	Name          sdk.String                 `nexa:"size=60,string=Reference"`
	StoreID       sdk.Many2One[EcomStore]    `nexa:"required,index,string=Store"`
	PartnerID     sdk.Many2One[CorePartner]  `nexa:"index,string=Customer"`
	SessionToken  sdk.String                 `nexa:"size=64,string=Session Token"`
	CurrencyID    sdk.Many2One[sdk.Any]      `nexa:"string=Currency,comodel=core.currency"`
	State         sdk.Selection[CartState]   `nexa:"string=State,default=open"`
	Email         sdk.String                 `nexa:"size=120,string=Email"`
	OrderCount    sdk.Integer                `nexa:"string=Item Count,default=0"`
	TotalUntaxed  sdk.Numeric                `nexa:"string=Subtotal,precision=18,scale=2,default=0"`
	TotalDiscount sdk.Numeric                `nexa:"string=Discount,precision=18,scale=2,default=0"`
	TotalTax      sdk.Numeric                `nexa:"string=Tax,precision=18,scale=2,default=0"`
	TotalGrand    sdk.Numeric                `nexa:"string=Total,precision=18,scale=2,default=0"`
	ExpiresAt     sdk.DateTime               `nexa:"string=Expires At"`
	MergedIntoID  sdk.Many2One[EcomCart]     `nexa:"string=Merged Into"`
	Note          sdk.Text                   `nexa:"string=Note"`
	Lines         sdk.One2Many[EcomCartLine] `nexa:"string=Lines"`
}

// EcomCartLine is a product line in a cart, with a session price snapshot.
type EcomCartLine struct {
	sdk.Model `nexa:"model=ecom.cart.line"`

	CartID        sdk.Many2One[EcomCart] `nexa:"required,index,string=Cart"`
	ProductID     sdk.Many2One[sdk.Any]  `nexa:"required,index,string=Product,comodel=product.product"`
	VariantName   sdk.String             `nexa:"size=120,string=Variant"`
	Quantity      sdk.Numeric            `nexa:"string=Quantity,precision=18,scale=4,default=1"`
	UnitPrice     sdk.Numeric            `nexa:"string=Unit Price,precision=18,scale=2,default=0"`
	Discount      sdk.Numeric            `nexa:"string=Discount,precision=18,scale=2,default=0"`
	PriceSubtotal sdk.Numeric            `nexa:"string=Subtotal,precision=18,scale=2,default=0"`
	PriceChanged  sdk.Boolean            `nexa:"string=Price Changed,default=false"`
	Description   sdk.Text               `nexa:"string=Description"`
	AddedAt       sdk.DateTime           `nexa:"string=Added At"`
}

// EcomCartAddress is a billing/shipping address snapshot on a cart.
type EcomCartAddress struct {
	sdk.Model `nexa:"model=ecom.cart.address"`

	CartID      sdk.Many2One[EcomCart]     `nexa:"required,index,string=Cart"`
	Type        sdk.Selection[AddressType] `nexa:"string=Type,default=shipping"`
	PartnerID   sdk.Many2One[CorePartner]  `nexa:"string=Contact"`
	ContactName sdk.String                 `nexa:"size=120,string=Contact Name"`
	Phone       sdk.String                 `nexa:"size=40,string=Phone"`
	Email       sdk.String                 `nexa:"size=120,string=Email"`
	Street      sdk.String                 `nexa:"size=200,string=Street"`
	Street2     sdk.String                 `nexa:"size=200,string=Street 2"`
	City        sdk.String                 `nexa:"size=100,string=City"`
	StateName   sdk.String                 `nexa:"size=100,string=State"`
	Zip         sdk.String                 `nexa:"size=20,string=ZIP"`
	Country     sdk.String                 `nexa:"size=80,string=Country"`
}

// EcomCartEvent records cart lifecycle and analytics events.
type EcomCartEvent struct {
	sdk.Model `nexa:"model=ecom.cart.event"`

	CartID    sdk.Many2One[EcomCart]       `nexa:"required,index,string=Cart"`
	Name      sdk.String                   `nexa:"size=120,string=Event"`
	EventType sdk.Selection[CartEventType] `nexa:"string=Type,default=updated"`
	Date      sdk.DateTime                 `nexa:"string=Date"`
	Message   sdk.Text                     `nexa:"string=Message"`
}

func init() {
	sdk.MustRegister("website_shop_cart",
		&EcomCart{},
		&EcomCartLine{},
		&EcomCartAddress{},
		&EcomCartEvent{},
	)
}

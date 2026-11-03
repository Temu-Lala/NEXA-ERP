package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// E-commerce — application anchor.
// website_shop owns the online storefront context: store/channel configuration,
// domains, per-store policies/settings, and product publication (how a product
// appears in a given store). Canonical SKU identity stays with Product; the
// confirmed commercial order stays with Sales (sale.order); invoices/journal
// entries stay with Accounting; stock balances stay with Inventory. E-commerce
// references those authorities via soft comodel pointers only.

type (
	CoreUser    = basemodels.CoreUser
	CorePartner = basemodels.CorePartner
	CoreCompany = basemodels.CoreCompany
)

type StoreStatus string

const (
	StoreDraft     StoreStatus = "draft"
	StoreActive    StoreStatus = "active"
	StoreSuspended StoreStatus = "suspended"
	StoreArchived  StoreStatus = "archived"
)

type DomainType string

const (
	DomainMain      DomainType = "domain"
	DomainSubdomain DomainType = "subdomain"
)

type DomainState string

const (
	DomPending  DomainState = "pending"
	DomVerified DomainState = "verified"
	DomFailed   DomainState = "failed"
)

type PublicationState string

const (
	PubDraft       PublicationState = "draft"
	PubPublished   PublicationState = "published"
	PubUnpublished PublicationState = "unpublished"
	PubArchived    PublicationState = "archived"
)

type SaleAvailability string

const (
	AvailInStock     SaleAvailability = "in_stock"
	AvailAlways      SaleAvailability = "always"
	AvailPreorder    SaleAvailability = "preorder"
	AvailBackorder   SaleAvailability = "backorder"
	AvailUnavailable SaleAvailability = "unavailable"
)

// EcomStore is an online storefront/channel owned by a company.
type EcomStore struct {
	sdk.Model `nexa:"model=ecom.store"`

	Name               sdk.String                    `nexa:"required,size=120,string=Store Name"`
	Code               sdk.String                    `nexa:"size=40,string=Code"`
	CompanyID          sdk.Many2One[CoreCompany]     `nexa:"string=Company"`
	Status             sdk.Selection[StoreStatus]    `nexa:"string=Status,default=draft"`
	Subdomain          sdk.String                    `nexa:"size=120,string=Subdomain"`
	DomainName         sdk.String                    `nexa:"size=120,string=Domain"`
	BaseURL            sdk.String                    `nexa:"size=200,string=Storefront URL"`
	CurrencyID         sdk.Many2One[sdk.Any]         `nexa:"string=Currency,comodel=core.currency"`
	DefaultPricelistID sdk.Many2One[sdk.Any]         `nexa:"string=Default Price List,comodel=product.pricelist"`
	DefaultWarehouseID sdk.Many2One[sdk.Any]         `nexa:"string=Default Warehouse,comodel=stock.warehouse"`
	Timezone           sdk.String                    `nexa:"size=60,string=Timezone"`
	Language           sdk.String                    `nexa:"size=20,string=Language"`
	AllowGuestCheckout sdk.Boolean                   `nexa:"string=Allow Guest Checkout,default=true"`
	MaintenanceMode    sdk.Boolean                   `nexa:"string=Maintenance Mode,default=false"`
	Active             sdk.Boolean                   `nexa:"string=Active,default=true"`
	Description        sdk.Text                      `nexa:"string=Description"`
	Note               sdk.Text                      `nexa:"string=Note"`
	Domains            sdk.One2Many[EcomStoreDomain] `nexa:"string=Domains"`
}

// EcomStoreDomain is a domain/subdomain attached to a store.
type EcomStoreDomain struct {
	sdk.Model `nexa:"model=ecom.store.domain"`

	StoreID   sdk.Many2One[EcomStore]    `nexa:"required,index,string=Store"`
	Name      sdk.String                 `nexa:"required,size=180,string=Domain Name"`
	Type      sdk.Selection[DomainType]  `nexa:"string=Type,default=subdomain"`
	IsPrimary sdk.Boolean                `nexa:"string=Primary,default=false"`
	Verified  sdk.Boolean                `nexa:"string=Verified,default=false"`
	State     sdk.Selection[DomainState] `nexa:"string=State,default=pending"`
}

// EcomStoreSettings holds per-store SEO, policy and checkout policy content.
type EcomStoreSettings struct {
	sdk.Model `nexa:"model=ecom.store.settings"`

	StoreID               sdk.Many2One[EcomStore] `nexa:"required,index,string=Store"`
	SeoTitle              sdk.String              `nexa:"size=180,string=SEO Title"`
	SeoDescription        sdk.Text                `nexa:"string=SEO Description"`
	TermsPolicy           sdk.Text                `nexa:"string=Terms & Conditions"`
	PrivacyPolicy         sdk.Text                `nexa:"string=Privacy Policy"`
	ReturnsPolicy         sdk.Text                `nexa:"string=Returns Policy"`
	ShippingPolicy        sdk.Text                `nexa:"string=Shipping Policy"`
	ContactEmail          sdk.String              `nexa:"size=120,string=Contact Email"`
	ContactPhone          sdk.String              `nexa:"size=40,string=Contact Phone"`
	SupportHours          sdk.String              `nexa:"size=120,string=Support Hours"`
	MinOrderAmount        sdk.Numeric             `nexa:"string=Minimum Order Amount,precision=18,scale=2,default=0"`
	FreeShippingThreshold sdk.Numeric             `nexa:"string=Free Shipping Threshold,precision=18,scale=2,default=0"`
}

// EcomProductPublication controls how a product is presented in a store.
type EcomProductPublication struct {
	sdk.Model `nexa:"model=ecom.product.publication"`

	StoreID             sdk.Many2One[EcomStore]         `nexa:"required,index,string=Store"`
	ProductID           sdk.Many2One[sdk.Any]           `nexa:"required,index,string=Product,comodel=product.product"`
	State               sdk.Selection[PublicationState] `nexa:"string=State,default=draft"`
	CatalogName         sdk.String                      `nexa:"size=180,string=Catalog Name"`
	CatalogDescription  sdk.Text                        `nexa:"string=Catalog Description"`
	SeoTitle            sdk.String                      `nexa:"size=180,string=SEO Title"`
	SeoDescription      sdk.Text                        `nexa:"string=SEO Description"`
	Slug                sdk.String                      `nexa:"size=180,string=URL Slug"`
	Price               sdk.Numeric                     `nexa:"string=Price,precision=18,scale=2,default=0"`
	ListPrice           sdk.Numeric                     `nexa:"string=List Price,precision=18,scale=2,default=0"`
	DiscountedPrice     sdk.Numeric                     `nexa:"string=Discounted Price,precision=18,scale=2,default=0"`
	Featured            sdk.Boolean                     `nexa:"string=Featured,default=false"`
	SaleAvailability    sdk.Selection[SaleAvailability] `nexa:"string=Availability,default=in_stock"`
	AvailabilityMessage sdk.String                      `nexa:"size=120,string=Availability Message"`
	ShowQuantity        sdk.Boolean                     `nexa:"string=Show Quantity,default=false"`
	Sequence            sdk.Integer                     `nexa:"string=Sequence,default=10"`
	DateFrom            sdk.Date                        `nexa:"string=Publish From"`
	DateTo              sdk.Date                        `nexa:"string=Publish To"`
	CompanyID           sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("website_shop",
		&EcomStore{},
		&EcomStoreDomain{},
		&EcomStoreSettings{},
		&EcomProductPublication{},
	)
}

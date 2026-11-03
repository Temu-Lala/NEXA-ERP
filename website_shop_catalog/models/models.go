package models

import (
	"nexa/core/sdk"
	shopmodels "nexa_addons/website_shop/models"
)

// E-commerce catalog & merchandising companion.
// Store-facing categories, curated/rule collections, and wishlists. Canonical
// product identity stays in Product (product.product); collections only group
// product references for storefront presentation.

type (
	EcomStore              = shopmodels.EcomStore
	EcomProductPublication = shopmodels.EcomProductPublication
	CorePartner            = shopmodels.CorePartner
	CoreCompany            = shopmodels.CoreCompany
)

type CollectionType string

const (
	CollManual   CollectionType = "manual"
	CollRule     CollectionType = "rule"
	CollFeatured CollectionType = "featured"
	CollSeasonal CollectionType = "seasonal"
)

type WishlistState string

const (
	WishActive   WishlistState = "active"
	WishArchived WishlistState = "archived"
)

// EcomCategory is a store-facing hierarchical navigation category.
type EcomCategory struct {
	sdk.Model `nexa:"model=ecom.category"`

	StoreID      sdk.Many2One[EcomStore]    `nexa:"index,string=Store"`
	Name         sdk.String                 `nexa:"required,size=120,string=Name"`
	ParentID     sdk.Many2One[EcomCategory] `nexa:"string=Parent Category"`
	CompleteName sdk.String                 `nexa:"size=200,string=Category"`
	Sequence     sdk.Integer                `nexa:"string=Sequence,default=10"`
	Description  sdk.Text                   `nexa:"string=Description"`
	ImageUrl     sdk.String                 `nexa:"size=200,string=Image URL"`
	Slug         sdk.String                 `nexa:"size=120,string=Slug"`
	Published    sdk.Boolean                `nexa:"string=Published,default=false"`
	Active       sdk.Boolean                `nexa:"string=Active,default=true"`
	CompanyID    sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
}

// EcomCollection groups products for a storefront landing page.
type EcomCollection struct {
	sdk.Model `nexa:"model=ecom.collection"`

	StoreID     sdk.Many2One[EcomStore]          `nexa:"index,string=Store"`
	Name        sdk.String                       `nexa:"required,size=120,string=Name"`
	Code        sdk.String                       `nexa:"size=40,string=Code"`
	Type        sdk.Selection[CollectionType]    `nexa:"string=Type,default=manual"`
	Description sdk.Text                         `nexa:"string=Description"`
	Banner      sdk.String                       `nexa:"size=200,string=Banner"`
	Published   sdk.Boolean                      `nexa:"string=Published,default=false"`
	Sequence    sdk.Integer                      `nexa:"string=Sequence,default=10"`
	RuleDomain  sdk.Text                         `nexa:"string=Rule (Domain)"`
	CompanyID   sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Items       sdk.One2Many[EcomCollectionItem] `nexa:"string=Products"`
}

// EcomCollectionItem is an explicit membership row in a collection.
type EcomCollectionItem struct {
	sdk.Model `nexa:"model=ecom.collection.item"`

	CollectionID  sdk.Many2One[EcomCollection]         `nexa:"required,index,string=Collection"`
	ProductID     sdk.Many2One[sdk.Any]                `nexa:"string=Product,comodel=product.product"`
	PublicationID sdk.Many2One[EcomProductPublication] `nexa:"string=Publication"`
	Sequence      sdk.Integer                          `nexa:"string=Sequence,default=10"`
}

// EcomWishlist is a customer's saved-items list.
type EcomWishlist struct {
	sdk.Model `nexa:"model=ecom.wishlist"`

	StoreID     sdk.Many2One[EcomStore]      `nexa:"index,string=Store"`
	PartnerID   sdk.Many2One[CorePartner]    `nexa:"string=Customer"`
	Name        sdk.String                   `nexa:"required,size=120,string=Name"`
	IsPublic    sdk.Boolean                  `nexa:"string=Public,default=false"`
	ShareToken  sdk.String                   `nexa:"size=60,string=Share Token"`
	State       sdk.Selection[WishlistState] `nexa:"string=State,default=active"`
	CreatedDate sdk.Date                     `nexa:"string=Created,default=today"`
	CompanyID   sdk.Many2One[CoreCompany]    `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("website_shop_catalog",
		&EcomCategory{},
		&EcomCollection{},
		&EcomCollectionItem{},
		&EcomWishlist{},
	)
}

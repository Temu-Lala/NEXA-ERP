package models

import (
	"nexa/core/sdk"
	shopmodels "nexa_addons/website_shop/models"
)

// E-commerce promotions, coupons and discounts companion.
// Controlled campaigns without touching product master data. Cart/checkout are
// referenced softly (comodel) so redemption can track usage without a hard
// dependency on where the discount was applied.

type (
	EcomStore   = shopmodels.EcomStore
	CorePartner = shopmodels.CorePartner
	CoreCompany = shopmodels.CoreCompany
)

type PromotionType string

const (
	promoPercentage    PromotionType = "percentage"
	promoFixed         PromotionType = "fixed"
	promoProduct       PromotionType = "product"
	promoCategory      PromotionType = "category"
	promoQtyTier       PromotionType = "qty_tier"
	promoMinOrder      PromotionType = "min_order"
	promoFreeShipping  PromotionType = "free_shipping"
	promoCustGroup     PromotionType = "customer_group"
	promoFirstPurchase PromotionType = "first_purchase"
)

type PromotionState string

const (
	PromoDraft    PromotionState = "draft"
	PromoActive   PromotionState = "active"
	PromoExpired  PromotionState = "expired"
	PromoArchived PromotionState = "archived"
)

type StackingRule string

const (
	StackExclusive StackingRule = "exclusive"
	StackStackable StackingRule = "stackable"
)

type RuleField string

const (
	RuleProduct    RuleField = "product"
	RuleCategory   RuleField = "category"
	RuleCollection RuleField = "collection"
	RuleCustGroup  RuleField = "customer_group"
	RuleTag        RuleField = "tag"
	RulePrice      RuleField = "price"
	RuleQuantity   RuleField = "quantity"
)

type RuleOperator string

const (
	OpIs      RuleOperator = "is"
	OpIsNot   RuleOperator = "is_not"
	OpIn      RuleOperator = "in"
	OpNotIn   RuleOperator = "not_in"
	OpGreater RuleOperator = "greater"
	OpLess    RuleOperator = "less"
)

type RedemptionState string

const (
	RedApplied  RedemptionState = "applied"
	RedReversed RedemptionState = "reversed"
)

// EcomPromotion is a discount campaign definition with eligibility and limits.
type EcomPromotion struct {
	sdk.Model `nexa:"model=ecom.promotion"`

	Name             sdk.String                      `nexa:"required,size=120,string=Name"`
	Code             sdk.String                      `nexa:"size=40,string=Code"`
	StoreID          sdk.Many2One[EcomStore]         `nexa:"index,string=Store"`
	CompanyID        sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
	Type             sdk.Selection[PromotionType]    `nexa:"string=Type,default=percentage"`
	StartDate        sdk.DateTime                    `nexa:"string=Start"`
	EndDate          sdk.DateTime                    `nexa:"string=End"`
	State            sdk.Selection[PromotionState]   `nexa:"string=State,default=draft"`
	Discount         sdk.Numeric                     `nexa:"string=Discount,precision=18,scale=2,default=0"`
	MaxDiscount      sdk.Numeric                     `nexa:"string=Max Discount,precision=18,scale=2,default=0"`
	MinOrderAmount   sdk.Numeric                     `nexa:"string=Min Order Amount,precision=18,scale=2,default=0"`
	MinQty           sdk.Integer                     `nexa:"string=Min Quantity,default=0"`
	StackingRule     sdk.Selection[StackingRule]     `nexa:"string=Stacking,default=exclusive"`
	Priority         sdk.Integer                     `nexa:"string=Priority,default=10"`
	UsageLimit       sdk.Integer                     `nexa:"string=Usage Limit,default=0"`
	UsedCount        sdk.Integer                     `nexa:"string=Used Count,default=0"`
	PerCustomerLimit sdk.Integer                     `nexa:"string=Per-Customer Limit,default=0"`
	Note             sdk.Text                        `nexa:"string=Note"`
	Rules            sdk.One2Many[EcomPromotionRule] `nexa:"string=Rules"`
}

// EcomPromotionRule is an eligibility condition row for a promotion.
type EcomPromotionRule struct {
	sdk.Model `nexa:"model=ecom.promotion.rule"`

	PromotionID sdk.Many2One[EcomPromotion] `nexa:"required,index,string=Promotion"`
	Sequence    sdk.Integer                 `nexa:"string=Sequence,default=1"`
	Field       sdk.Selection[RuleField]    `nexa:"string=Field,default=product"`
	Operator    sdk.Selection[RuleOperator] `nexa:"string=Operator,default=is"`
	Value       sdk.String                  `nexa:"size=200,string=Value"`
	Amount      sdk.Numeric                 `nexa:"string=Amount,precision=18,scale=2,default=0"`
}

// EcomCoupon is a redeemable code bound to a promotion.
type EcomCoupon struct {
	sdk.Model `nexa:"model=ecom.coupon"`

	PromotionID      sdk.Many2One[EcomPromotion]        `nexa:"required,index,string=Promotion"`
	Code             sdk.String                         `nexa:"required,index,size=60,string=Code"`
	StoreID          sdk.Many2One[EcomStore]            `nexa:"string=Store"`
	Active           sdk.Boolean                        `nexa:"string=Active,default=true"`
	MaxUses          sdk.Integer                        `nexa:"string=Max Uses,default=0"`
	Uses             sdk.Integer                        `nexa:"string=Uses,default=0"`
	PerCustomerLimit sdk.Integer                        `nexa:"string=Per-Customer Limit,default=0"`
	ExpirationDate   sdk.Date                           `nexa:"string=Expiration Date"`
	Description      sdk.Text                           `nexa:"string=Description"`
	Redemptions      sdk.One2Many[EcomCouponRedemption] `nexa:"string=Redemptions"`
}

// EcomCouponRedemption records a coupon usage against a cart/checkout.
type EcomCouponRedemption struct {
	sdk.Model `nexa:"model=ecom.coupon.redemption"`

	CouponID         sdk.Many2One[EcomCoupon]       `nexa:"required,index,string=Coupon"`
	CartID           sdk.Many2One[sdk.Any]          `nexa:"string=Cart,comodel=ecom.cart"`
	CheckoutID       sdk.Many2One[sdk.Any]          `nexa:"string=Checkout,comodel=ecom.checkout"`
	PartnerID        sdk.Many2One[CorePartner]      `nexa:"index,string=Customer"`
	Date             sdk.DateTime                   `nexa:"string=Date"`
	AmountDiscounted sdk.Numeric                    `nexa:"string=Discount,precision=18,scale=2,default=0"`
	State            sdk.Selection[RedemptionState] `nexa:"string=State,default=applied"`
}

func init() {
	sdk.MustRegister("website_shop_promotion",
		&EcomPromotion{},
		&EcomPromotionRule{},
		&EcomCoupon{},
		&EcomCouponRedemption{},
	)
}

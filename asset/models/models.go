package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// CorePartner → core.partner
type CorePartner = basemodels.CorePartner

// CoreUser → core.user
type CoreUser = basemodels.CoreUser

type AssetState string

const (
	AssetStateDraft    AssetState = "draft"
	AssetStateActive   AssetState = "active"
	AssetStateDisposed AssetState = "disposed"
)

type AssetCategory struct {
	sdk.Model `nexa:"model=asset.category"`

	Name   sdk.String  `nexa:"required,string=Category Name"`
	Code   sdk.String  `nexa:"string=Code"`
	Notes  sdk.Text    `nexa:"string=Notes"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}

type FixedAsset struct {
	sdk.Model `nexa:"model=asset.asset"`

	Name          sdk.String                  `nexa:"required,string=Asset Name"`
	Code          sdk.String                  `nexa:"string=Reference"`
	CategoryID    sdk.Many2One[AssetCategory] `nexa:"string=Category"`
	PartnerID     sdk.Many2One[CorePartner]   `nexa:"string=Vendor"`
	PurchaseDate  sdk.Date                    `nexa:"string=Purchase Date"`
	PurchaseValue sdk.Numeric                 `nexa:"string=Purchase Value,precision=18,scale=2"`
	ResidualValue sdk.Numeric                 `nexa:"string=Residual Value,precision=18,scale=2"`
	UsefulLife    sdk.Integer                 `nexa:"string=Useful Life (months)"`
	AssignedTo    sdk.Many2One[CoreUser]      `nexa:"string=Assigned To"`
	State         sdk.Selection[AssetState]   `nexa:"string=Status,default=draft"`
	Notes         sdk.Text                    `nexa:"string=Notes"`
	Active        sdk.Boolean                 `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("asset",
		&AssetCategory{},
		&FixedAsset{},
	)
}

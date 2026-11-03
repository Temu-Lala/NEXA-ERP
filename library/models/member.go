package models

import (
	"nexa/core/sdk"
)

type LibraryMember struct {
	sdk.Model `nexa:"model=library.member"`

	Name      sdk.String                `nexa:"required,string=Name"`
	Code      sdk.String                `nexa:"string=Member Number"`
	PartnerID sdk.Many2One[CorePartner] `nexa:"string=Related Partner"`
	Email     sdk.Email                 `nexa:"string=Email"`
	Phone     sdk.Phone                 `nexa:"string=Phone"`
	JoinDate  sdk.Date                  `nexa:"string=Membership Start"`
	MaxLoans  sdk.Integer               `nexa:"string=Max Loans,default=5"`
	Notes     sdk.Text                  `nexa:"string=Notes"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

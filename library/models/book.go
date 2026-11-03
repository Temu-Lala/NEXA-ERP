package models

import (
	"nexa/core/sdk"
)

type LibraryBook struct {
	sdk.Model `nexa:"model=library.book"`

	Name            sdk.String                    `nexa:"required,string=Title"`
	Reference       sdk.String                    `nexa:"string=Reference"`
	Author          sdk.String                    `nexa:"string=Author"`
	Publisher       sdk.String                    `nexa:"string=Publisher"`
	CategoryID      sdk.Many2One[LibraryCategory] `nexa:"string=Category"`
	PublicationDate sdk.Date                      `nexa:"string=Publication Date"`
	Copies          sdk.Integer                   `nexa:"string=Copies,default=1"`
	Available       sdk.Integer                   `nexa:"string=Available Copies,default=1"`
	Price           sdk.Numeric                   `nexa:"string=Price,precision=18,scale=2"`
	Summary         sdk.Text                      `nexa:"string=Description"`
	Active          sdk.Boolean                   `nexa:"string=Active,default=true"`
}

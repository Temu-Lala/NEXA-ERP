package models

import (
	"nexa/core/sdk"
)

type LibraryLoan struct {
	sdk.Model `nexa:"model=library.loan"`

	Name       sdk.String                  `nexa:"string=Number"`
	MemberID   sdk.Many2One[LibraryMember] `nexa:"required,string=Member"`
	BookID     sdk.Many2One[LibraryBook]   `nexa:"required,string=Book"`
	LoanDate   sdk.Date                    `nexa:"string=Loan Date"`
	DueDate    sdk.Date                    `nexa:"string=Due Date"`
	ReturnDate sdk.Date                    `nexa:"string=Return Date"`
	State      sdk.Selection[LoanState]    `nexa:"string=Status,default=draft"`
	Fine       sdk.Numeric                 `nexa:"string=Fine,precision=18,scale=2"`
	Notes      sdk.Text                    `nexa:"string=Notes"`
}

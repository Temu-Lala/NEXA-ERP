package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// CoreUser → core.user
type CoreUser = basemodels.CoreUser

type ExpenseState string

const (
	ExpenseStateDraft     ExpenseState = "draft"
	ExpenseStateSubmitted ExpenseState = "submitted"
	ExpenseStateApproved  ExpenseState = "approved"
	ExpenseStateRefused   ExpenseState = "refused"
	ExpenseStatePaid      ExpenseState = "paid"
)

type ExpenseCategory struct {
	sdk.Model `nexa:"model=expense.category"`

	Name   sdk.String  `nexa:"required,string=Category Name"`
	Code   sdk.String  `nexa:"string=Code"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}

type ExpenseLine struct {
	sdk.Model `nexa:"model=expense.expense"`

	Name        sdk.String                    `nexa:"required,string=Description"`
	CategoryID  sdk.Many2One[ExpenseCategory] `nexa:"string=Category"`
	UserID      sdk.Many2One[CoreUser]        `nexa:"string=Submitted By"`
	Amount      sdk.Numeric                   `nexa:"string=Amount,precision=18,scale=2"`
	ExpenseDate sdk.Date                      `nexa:"string=Expense Date"`
	State       sdk.Selection[ExpenseState]   `nexa:"string=Status,default=draft"`
	Notes       sdk.Text                      `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("expense",
		&ExpenseCategory{},
		&ExpenseLine{},
	)
}

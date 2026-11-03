package models

import (
	"nexa/core/sdk"
	acctmodels "nexa_addons/account/models"
)

// AccountAsset → account.asset
type AccountAsset = acctmodels.AccountAsset

// AccountAccount → account.account
type AccountAccount = acctmodels.AccountAccount

// AccountJournal → account.journal
type AccountJournal = acctmodels.AccountJournal

type DepMethod string

const (
	DepMethodStraightLine DepMethod = "straight_line"
	DepMethodDeclining    DepMethod = "declining_balance"
	DepMethodUnits        DepMethod = "units_of_production"
)

type Convention string

const (
	ConventionFullMonth Convention = "full_month"
	ConventionHalfYear  Convention = "half_year"
	ConventionMidMonth  Convention = "mid_month"
)

type BookType string

const (
	BookTypeFinancial BookType = "financial_book"
	BookTypeTax       BookType = "tax_book"
)

type BookState string

const (
	BookStateDraft BookState = "draft"
	BookStateOpen  BookState = "open"
	BookStateClose BookState = "close"
)

type DepRunState string

const (
	DepRunStateDraft     DepRunState = "draft"
	DepRunStatePosted    DepRunState = "posted"
	DepRunStateCancelled DepRunState = "cancelled"
)

type DisposalMethod string

const (
	DisposalSell    DisposalMethod = "sell"
	DisposalScrap   DisposalMethod = "scrap"
	DisposalDonate  DisposalMethod = "donate"
	DisposalTradeIn DisposalMethod = "trade_in"
)

type DisposalState string

const (
	DisposalStateDraft  DisposalState = "draft"
	DisposalStatePosted DisposalState = "posted"
)

// AccountAssetBook is a depreciation book (financial or tax) for an asset.
type AccountAssetBook struct {
	sdk.Model `nexa:"model=account.asset.book"`

	Name                  sdk.String                                 `nexa:"required,string=Name"`
	AssetID               sdk.Many2One[AccountAsset]                 `nexa:"required,index,string=Asset"`
	Method                sdk.Selection[DepMethod]                   `nexa:"string=Method,default=straight_line"`
	BookType              sdk.Selection[BookType]                    `nexa:"string=Book Type,default=financial_book"`
	Convention            sdk.Selection[Convention]                  `nexa:"string=Convention,default=full_month"`
	Cost                  sdk.Numeric                                `nexa:"string=Cost,precision=18,scale=2,default=0"`
	ResidualValue         sdk.Numeric                                `nexa:"string=Residual Value,precision=18,scale=2,default=0"`
	UsefulLife            sdk.Integer                                `nexa:"string=Useful Life (months)"`
	StartDate             sdk.Date                                   `nexa:"string=Start Date"`
	AccumulatedDep        sdk.Numeric                                `nexa:"string=Accumulated Depreciation,precision=18,scale=2,default=0"`
	NetBookValue          sdk.Numeric                                `nexa:"string=Net Book Value,precision=18,scale=2,default=0"`
	JournalID             sdk.Many2One[AccountJournal]               `nexa:"string=Journal"`
	ExpenseAccountID      sdk.Many2One[AccountAccount]               `nexa:"string=Depreciation Expense Account"`
	DepreciationAccountID sdk.Many2One[AccountAccount]               `nexa:"string=Accumulated Depreciation Account"`
	Lines                 sdk.One2Many[AccountAssetDepreciationLine] `nexa:"string=Depreciation Lines"`
	State                 sdk.Selection[BookState]                   `nexa:"string=Status,default=open"`
}

// AccountAssetDepreciationLine is one scheduled depreciation entry within a book.
type AccountAssetDepreciationLine struct {
	sdk.Model `nexa:"model=account.asset.depreciation.line"`

	BookID      sdk.Many2One[AccountAssetBook] `nexa:"required,index,string=Book"`
	Sequence    sdk.Integer                    `nexa:"string=Sequence"`
	Date        sdk.Date                       `nexa:"string=Depreciation Date"`
	Amount      sdk.Numeric                    `nexa:"string=Amount,precision=18,scale=2,default=0"`
	Depreciated sdk.Boolean                    `nexa:"string=Posted"`
	MoveLineID  sdk.Many2One[sdk.Any]          `nexa:"string=Journal Item,comodel=account.move.line"`
}

// AccountAssetDepreciationRun posts depreciation for a set of books over a period.
type AccountAssetDepreciationRun struct {
	sdk.Model `nexa:"model=account.asset.depreciation.run"`

	Name           sdk.String                 `nexa:"required,string=Name"`
	PeriodID       sdk.Many2One[sdk.Any]      `nexa:"string=Period,comodel=account.period"`
	DateFrom       sdk.Date                   `nexa:"string=From"`
	DateTo         sdk.Date                   `nexa:"string=To"`
	State          sdk.Selection[DepRunState] `nexa:"string=Status,default=draft"`
	BookScope      sdk.Text                   `nexa:"string=Books"`
	TotalAmount    sdk.Numeric                `nexa:"string=Total,precision=18,scale=2,default=0"`
	JournalEntryID sdk.Many2One[sdk.Any]      `nexa:"string=Journal Entry,comodel=account.move"`
	Note           sdk.Text                   `nexa:"string=Notes"`
}

// AccountAssetDisposal records the removal of an asset from the register.
type AccountAssetDisposal struct {
	sdk.Model `nexa:"model=account.asset.disposal"`

	AssetID            sdk.Many2One[AccountAsset]     `nexa:"required,index,string=Asset"`
	BookID             sdk.Many2One[AccountAssetBook] `nexa:"string=Book"`
	Date               sdk.Date                       `nexa:"required,string=Disposal Date"`
	Method             sdk.Selection[DisposalMethod]  `nexa:"string=Method,default=sell"`
	Proceeds           sdk.Numeric                    `nexa:"string=Proceeds,precision=18,scale=2,default=0"`
	CostRemoved        sdk.Numeric                    `nexa:"string=Cost Removed,precision=18,scale=2,default=0"`
	AccumulatedRemoved sdk.Numeric                    `nexa:"string=Accumulated Removed,precision=18,scale=2,default=0"`
	GainLoss           sdk.Numeric                    `nexa:"string=Gain/Loss,precision=18,scale=2,default=0"`
	JournalEntryID     sdk.Many2One[sdk.Any]          `nexa:"string=Journal Entry,comodel=account.move"`
	State              sdk.Selection[DisposalState]   `nexa:"string=Status,default=draft"`
	Note               sdk.Text                       `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("account_asset",
		&AccountAssetBook{},
		&AccountAssetDepreciationLine{},
		&AccountAssetDepreciationRun{},
		&AccountAssetDisposal{},
	)
}

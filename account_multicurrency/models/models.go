package models

import (
	"nexa/core/sdk"
	acctmodels "nexa_addons/account/models"
)

// CoreCompany → core.company
type CoreCompany = acctmodels.CoreCompany

// CoreCurrency → core.currency
type CoreCurrency = acctmodels.CoreCurrency

// CorePartner → core.partner
type CorePartner = acctmodels.CorePartner

// AccountAccount → account.account
type AccountAccount = acctmodels.AccountAccount

// AccountJournal → account.journal
type AccountJournal = acctmodels.AccountJournal

type RateSource string

const (
	RateSourceManual RateSource = "manual"
	RateSourceECB    RateSource = "ecb"
	RateSourceServer RateSource = "server"
)

type FXMode string

const (
	FXModeSplit    FXMode = "realized_unrealized_split"
	FXModeCombined FXMode = "combined"
)

type RevalState string

const (
	RevalStateDraft     RevalState = "draft"
	RevalStatePosted    RevalState = "posted"
	RevalStateCancelled RevalState = "cancelled"
)

type FXType string

const (
	FXTypeRealized   FXType = "realized"
	FXTypeUnrealized FXType = "unrealized"
)

// AccountExchangeRate is an effective-dated currency rate.
type AccountExchangeRate struct {
	sdk.Model `nexa:"model=account.exchange.rate"`

	CurrencyID sdk.Many2One[CoreCurrency] `nexa:"required,index,string=Currency"`
	CompanyID  sdk.Many2One[CoreCompany]  `nexa:"string=Company"`
	Date       sdk.Date                   `nexa:"required,index,string=Date"`
	Rate       sdk.Numeric                `nexa:"required,string=Rate,precision=18,scale=8"`
	Source     sdk.Selection[RateSource]  `nexa:"string=Source,default=manual"`
	Note       sdk.Text                   `nexa:"string=Notes"`
}

// AccountRevaluationConfig holds gain/loss accounts and journal for FX revaluation.
type AccountRevaluationConfig struct {
	sdk.Model `nexa:"model=account.revaluation.config"`

	CompanyID     sdk.Many2One[CoreCompany]    `nexa:"required,string=Company"`
	CurrencyID    sdk.Many2One[CoreCurrency]   `nexa:"string=Currency"`
	GainAccountID sdk.Many2One[AccountAccount] `nexa:"string=Gain Account"`
	LossAccountID sdk.Many2One[AccountAccount] `nexa:"string=Loss Account"`
	JournalID     sdk.Many2One[AccountJournal] `nexa:"string=Journal"`
	FXAccountMode sdk.Selection[FXMode]        `nexa:"string=FX Treatment,default=combined"`
	Note          sdk.Text                     `nexa:"string=Notes"`
}

// AccountRevaluationRun is a period-end foreign-currency revaluation.
type AccountRevaluationRun struct {
	sdk.Model `nexa:"model=account.revaluation.run"`

	Name           sdk.String                             `nexa:"required,string=Name"`
	CompanyID      sdk.Many2One[CoreCompany]              `nexa:"required,string=Company"`
	Date           sdk.Date                               `nexa:"required,string=Revaluation Date"`
	State          sdk.Selection[RevalState]              `nexa:"string=Status,default=draft"`
	ConfigID       sdk.Many2One[AccountRevaluationConfig] `nexa:"string=Configuration"`
	Lines          sdk.One2Many[AccountRevaluationLine]   `nexa:"string=Lines"`
	JournalEntryID sdk.Many2One[sdk.Any]                  `nexa:"string=Journal Entry,comodel=account.move"`
	TotalGainLoss  sdk.Numeric                            `nexa:"string=Total Gain/Loss,precision=18,scale=2,default=0"`
	Note           sdk.Text                               `nexa:"string=Notes"`
}

// AccountRevaluationLine is one revalued balance within a run.
type AccountRevaluationLine struct {
	sdk.Model `nexa:"model=account.revaluation.line"`

	RunID          sdk.Many2One[AccountRevaluationRun] `nexa:"required,index,string=Run"`
	AccountID      sdk.Many2One[AccountAccount]        `nexa:"string=Account"`
	PartnerID      sdk.Many2One[CorePartner]           `nexa:"string=Partner"`
	CurrencyID     sdk.Many2One[CoreCurrency]          `nexa:"string=Currency"`
	BalanceForeign sdk.Numeric                         `nexa:"string=Foreign Balance,precision=18,scale=2,default=0"`
	BookAmount     sdk.Numeric                         `nexa:"string=Book Amount,precision=18,scale=2,default=0"`
	RevaluedAmount sdk.Numeric                         `nexa:"string=Revalued Amount,precision=18,scale=2,default=0"`
	GainLoss       sdk.Numeric                         `nexa:"string=Gain/Loss,precision=18,scale=2,default=0"`
	Type           sdk.Selection[FXType]               `nexa:"string=Type,default=unrealized"`
}

func init() {
	sdk.MustRegister("account_multicurrency",
		&AccountExchangeRate{},
		&AccountRevaluationConfig{},
		&AccountRevaluationRun{},
		&AccountRevaluationLine{},
	)
}

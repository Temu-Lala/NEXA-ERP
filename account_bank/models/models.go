package models

import (
	"nexa/core/sdk"
	acctmodels "nexa_addons/account/models"
)

// CoreCompany → core.company
type CoreCompany = acctmodels.CoreCompany

// CoreCurrency → core.currency
type CoreCurrency = acctmodels.CoreCurrency

// CoreUser → core.user
type CoreUser = acctmodels.CoreUser

// AccountAccount → account.account
type AccountAccount = acctmodels.AccountAccount

// AccountJournal → account.journal
type AccountJournal = acctmodels.AccountJournal

type CashSessionState string

const (
	CashSessionOpen   CashSessionState = "open"
	CashSessionClosed CashSessionState = "closed"
)

type CashMoveType string

const (
	CashMoveIn         CashMoveType = "in"
	CashMoveOut        CashMoveType = "out"
	CashMoveTransfer   CashMoveType = "transfer"
	CashMoveAdjustment CashMoveType = "adjustment"
)

type CashMethod string

const (
	CashMethodCash         CashMethod = "cash"
	CashMethodCard         CashMethod = "card"
	CashMethodBankTransfer CashMethod = "bank_transfer"
	CashMethodMobileMoney  CashMethod = "mobile_money"
)

type StmtFormat string

const (
	StmtFormatCSV  StmtFormat = "csv"
	StmtFormatOFX  StmtFormat = "ofx"
	StmtFormatQIF  StmtFormat = "qif"
	StmtFormatCAMT StmtFormat = "camt"
)

type ImportState string

const (
	ImportStateDraft    ImportState = "draft"
	ImportStateReviewed ImportState = "reviewed"
	ImportStatePosted   ImportState = "posted"
)

type Provider string

const (
	ProviderStripe Provider = "stripe"
	ProviderPaypal Provider = "paypal"
	ProviderBank   Provider = "bank"
)

type SettleState string

const (
	SettleStatePending    SettleState = "pending"
	SettleStateReconciled SettleState = "reconciled"
	SettleStatePosted     SettleState = "posted"
)

// AccountCashSession is a cashier/till session with opening and counted balances.
type AccountCashSession struct {
	sdk.Model `nexa:"model=account.cash.session"`

	Name            sdk.String                      `nexa:"required,string=Name"`
	JournalID       sdk.Many2One[AccountJournal]    `nexa:"required,index,string=Journal"`
	CompanyID       sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
	CashierID       sdk.Many2One[CoreUser]          `nexa:"string=Cashier"`
	OpeningDate     sdk.DateTime                    `nexa:"string=Opening Date"`
	ClosingDate     sdk.DateTime                    `nexa:"string=Closing Date"`
	State           sdk.Selection[CashSessionState] `nexa:"string=Status,default=open"`
	OpeningBalance  sdk.Numeric                     `nexa:"string=Opening Balance,precision=18,scale=2,default=0"`
	ComputedBalance sdk.Numeric                     `nexa:"string=Computed Balance,precision=18,scale=2,default=0"`
	CountedBalance  sdk.Numeric                     `nexa:"string=Counted Balance,precision=18,scale=2,default=0"`
	Difference      sdk.Numeric                     `nexa:"string=Difference,precision=18,scale=2,default=0"`
	Moves           sdk.One2Many[AccountCashMove]   `nexa:"string=Moves"`
	Note            sdk.Text                        `nexa:"string=Notes"`
}

// AccountCashMove is a single movement within a cash session.
type AccountCashMove struct {
	sdk.Model `nexa:"model=account.cash.move"`

	SessionID             sdk.Many2One[AccountCashSession] `nexa:"required,index,string=Session"`
	Date                  sdk.DateTime                     `nexa:"required,string=Date"`
	Type                  sdk.Selection[CashMoveType]      `nexa:"string=Type,default=in"`
	Amount                sdk.Numeric                      `nexa:"string=Amount,precision=18,scale=2,default=0"`
	CounterpartyAccountID sdk.Many2One[AccountAccount]     `nexa:"string=Counterparty Account"`
	PaymentMethod         sdk.Selection[CashMethod]        `nexa:"string=Payment Method,default=cash"`
	Reference             sdk.String                       `nexa:"string=Reference"`
	Description           sdk.Text                         `nexa:"string=Description"`
	MoveLineID            sdk.Many2One[sdk.Any]            `nexa:"string=Journal Item,comodel=account.move.line"`
}

// AccountBankStatementImport is a staged bank statement file import with match stats.
type AccountBankStatementImport struct {
	sdk.Model `nexa:"model=account.bank.statement.import"`

	Name           sdk.String                   `nexa:"required,string=Name"`
	BankJournalID  sdk.Many2One[AccountJournal] `nexa:"required,index,string=Bank Journal"`
	Format         sdk.Selection[StmtFormat]    `nexa:"string=Format,default=csv"`
	ImportedAt     sdk.DateTime                 `nexa:"string=Imported At"`
	LineCount      sdk.Integer                  `nexa:"string=Lines"`
	MatchedCount   sdk.Integer                  `nexa:"string=Matched"`
	UnmatchedCount sdk.Integer                  `nexa:"string=Unmatched"`
	State          sdk.Selection[ImportState]   `nexa:"string=Status,default=draft"`
	StatementID    sdk.Many2One[sdk.Any]        `nexa:"string=Statement,comodel=account.bank.statement"`
	RawData        sdk.Text                     `nexa:"string=Raw Data"`
}

// AccountProviderSettlement reconciles a payment-provider payout into the bank.
type AccountProviderSettlement struct {
	sdk.Model `nexa:"model=account.provider.settlement"`

	Name              sdk.String                   `nexa:"required,string=Name"`
	Provider          sdk.Selection[Provider]      `nexa:"string=Provider,default=bank"`
	PeriodDateFrom    sdk.Date                     `nexa:"string=Period Start"`
	PeriodDateTo      sdk.Date                     `nexa:"string=Period End"`
	GrossAmount       sdk.Numeric                  `nexa:"string=Gross,precision=18,scale=2,default=0"`
	FeeAmount         sdk.Numeric                  `nexa:"string=Fees,precision=18,scale=2,default=0"`
	NetAmount         sdk.Numeric                  `nexa:"string=Net,precision=18,scale=2,default=0"`
	CurrencyID        sdk.Many2One[CoreCurrency]   `nexa:"string=Currency"`
	ClearingAccountID sdk.Many2One[AccountAccount] `nexa:"string=Clearing Account"`
	BankAccountID     sdk.Many2One[AccountAccount] `nexa:"string=Bank Account"`
	State             sdk.Selection[SettleState]   `nexa:"string=Status,default=pending"`
	JournalEntryID    sdk.Many2One[sdk.Any]        `nexa:"string=Journal Entry,comodel=account.move"`
	Note              sdk.Text                     `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("account_bank",
		&AccountCashSession{},
		&AccountCashMove{},
		&AccountBankStatementImport{},
		&AccountProviderSettlement{},
	)
}

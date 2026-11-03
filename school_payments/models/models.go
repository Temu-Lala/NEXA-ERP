package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	schoolfeesmodels "nexa_addons/school_fees/models"
)

// Payment side of the school system: periodic fee bills, recorded payments and
// receipts, accumulated penalties, and privileged penalty waivers. Amounts are
// snapshots owned by the school; posting to the accounting ledger is out of
// scope here. Penalty waivers are restricted to the Manager/Director role.

type (
	CoreUser           = basemodels.CoreUser
	CoreCompany        = basemodels.CoreCompany
	SchoolStudent      = schoolmodels.SchoolStudent
	SchoolFeeCategory  = schoolmodels.SchoolFeeCategory
	SchoolAcademicYear = schoolmodels.SchoolAcademicYear
	SchoolFeeStructure = schoolfeesmodels.SchoolFeeStructure
	SchoolPenaltyRule  = schoolfeesmodels.SchoolPenaltyRule
)

type BillStatus string

const (
	BillUnpaid  BillStatus = "unpaid"
	BillPartial BillStatus = "partial"
	BillPaid    BillStatus = "paid"
	BillVoid    BillStatus = "void"
)

type BillDiscountType string

const (
	DiscNone    BillDiscountType = "none"
	DiscFamily  BillDiscountType = "family"
	DiscTeacher BillDiscountType = "teacher"
)

type PaymentMethod string

const (
	PayCash   PaymentMethod = "cash"
	PayBank   PaymentMethod = "bank"
	PayMobile PaymentMethod = "mobile"
	PayCheck  PaymentMethod = "check"
)

type WaiverType string

const (
	WaiveFull    WaiverType = "full"
	WaivePartial WaiverType = "partial"
)

// SchoolFeeBill is a periodic charge issued to a student for one fee category.
type SchoolFeeBill struct {
	sdk.Model `nexa:"model=school.fee.bill"`

	Name           sdk.String                       `nexa:"required,index,size=60,string=Bill No"`
	StudentID      sdk.Many2One[SchoolStudent]      `nexa:"required,index,string=Student"`
	FeeStructureID sdk.Many2One[SchoolFeeStructure] `nexa:"string=Fee Structure"`
	FeeCategoryID  sdk.Many2One[SchoolFeeCategory]  `nexa:"string=Fee Category"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"string=Academic Year"`
	PeriodLabel    sdk.String                       `nexa:"size=20,string=Period"`
	AmountDue      sdk.Numeric                      `nexa:"string=Amount Due,precision=18,scale=2,default=0"`
	DiscountType   sdk.Selection[BillDiscountType]  `nexa:"string=Discount Type,default=none"`
	DiscountPct    sdk.Numeric                      `nexa:"string=Discount %,precision=5,scale=2,default=0"`
	DiscountAmount sdk.Numeric                      `nexa:"string=Discount Amount,precision=18,scale=2,default=0"`
	PenaltyAmount  sdk.Numeric                      `nexa:"string=Penalty,precision=18,scale=2,default=0"`
	TotalAmount    sdk.Numeric                      `nexa:"string=Total Payable,precision=18,scale=2,default=0"`
	AmountPaid     sdk.Numeric                      `nexa:"string=Amount Paid,precision=18,scale=2,default=0"`
	Balance        sdk.Numeric                      `nexa:"string=Balance,precision=18,scale=2,default=0"`
	DueDate        sdk.Date                         `nexa:"string=Due Date"`
	Status         sdk.Selection[BillStatus]        `nexa:"string=Status,default=unpaid"`
	Currency       sdk.String                       `nexa:"size=3,string=Currency,default=USD"`
	CompanyID      sdk.Many2One[CoreCompany]        `nexa:"string=Company"`
	Notes          sdk.Text                         `nexa:"string=Notes"`
	Active         sdk.Boolean                      `nexa:"string=Active,default=true"`
}

// SchoolPayment records a payment received against a student/bill and issues a
// receipt number.
type SchoolPayment struct {
	sdk.Model `nexa:"model=school.payment"`

	Name          sdk.String                      `nexa:"required,index,size=60,string=Receipt No"`
	StudentID     sdk.Many2One[SchoolStudent]     `nexa:"required,index,string=Student"`
	BillID        sdk.Many2One[SchoolFeeBill]     `nexa:"string=Bill"`
	FeeCategoryID sdk.Many2One[SchoolFeeCategory] `nexa:"string=Fee Category"`
	PaymentDate   sdk.Date                        `nexa:"string=Payment Date,default=today"`
	PeriodLabel   sdk.String                      `nexa:"size=20,string=Period"`
	Amount        sdk.Numeric                     `nexa:"required,string=Amount,precision=18,scale=2,default=0"`
	PenaltyPaid   sdk.Numeric                     `nexa:"string=Penalty Paid,precision=18,scale=2,default=0"`
	Method        sdk.Selection[PaymentMethod]    `nexa:"string=Method,default=cash"`
	Reference     sdk.String                      `nexa:"size=120,string=Reference"`
	CashierID     sdk.Many2One[CoreUser]          `nexa:"string=Cashier"`
	CompanyID     sdk.Many2One[CoreCompany]       `nexa:"string=Company"`
	Notes         sdk.Text                        `nexa:"string=Notes"`
	Active        sdk.Boolean                     `nexa:"string=Active,default=true"`
}

// SchoolPenalty is an accumulated late-payment penalty computed against a bill.
type SchoolPenalty struct {
	sdk.Model `nexa:"model=school.penalty"`

	Name          sdk.String                      `nexa:"required,size=60,string=Reference"`
	BillID        sdk.Many2One[SchoolFeeBill]     `nexa:"required,index,string=Bill"`
	StudentID     sdk.Many2One[SchoolStudent]     `nexa:"index,string=Student"`
	PenaltyRuleID sdk.Many2One[SchoolPenaltyRule] `nexa:"string=Penalty Rule"`
	Amount        sdk.Numeric                     `nexa:"string=Penalty Amount,precision=18,scale=2,default=0"`
	WaivedAmount  sdk.Numeric                     `nexa:"string=Waived Amount,precision=18,scale=2,default=0"`
	DaysLate      sdk.Integer                     `nexa:"string=Days Late,default=0"`
	DateFrom      sdk.Date                        `nexa:"string=From"`
	DateTo        sdk.Date                        `nexa:"string=To"`
	Notes         sdk.Text                        `nexa:"string=Notes"`
	Active        sdk.Boolean                     `nexa:"string=Active,default=true"`
}

// SchoolPenaltyWaiver forgives part or all of an accumulated penalty. This is a
// restricted privilege (Manager/Director only) and records the approver/reason.
type SchoolPenaltyWaiver struct {
	sdk.Model `nexa:"model=school.penalty.waiver"`

	Name         sdk.String                  `nexa:"required,size=60,string=Reference"`
	PenaltyID    sdk.Many2One[SchoolPenalty] `nexa:"required,index,string=Penalty"`
	StudentID    sdk.Many2One[SchoolStudent] `nexa:"string=Student"`
	WaiverType   sdk.Selection[WaiverType]   `nexa:"string=Waiver Type,default=partial"`
	AmountWaived sdk.Numeric                 `nexa:"required,string=Amount Waived,precision=18,scale=2,default=0"`
	Reason       sdk.String                  `nexa:"required,size=255,string=Reason"`
	WaivedBy     sdk.Many2One[CoreUser]      `nexa:"string=Approved By"`
	WaiverDate   sdk.Date                    `nexa:"string=Waiver Date,default=today"`
	Notes        sdk.Text                    `nexa:"string=Notes"`
	Active       sdk.Boolean                 `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("school_payments",
		&SchoolFeeBill{},
		&SchoolPayment{},
		&SchoolPenalty{},
		&SchoolPenaltyWaiver{},
	)
}

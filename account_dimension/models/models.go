package models

import (
	"nexa/core/sdk"
	acctmodels "nexa_addons/account/models"
)

// CoreCompany → core.company
type CoreCompany = acctmodels.CoreCompany

// AccountMoveLine → account.move.line
type AccountMoveLine = acctmodels.AccountMoveLine

type DimCategory string

const (
	DimCategoryBranch      DimCategory = "branch"
	DimCategoryDepartment  DimCategory = "department"
	DimCategoryCostCenter  DimCategory = "cost_center"
	DimCategoryProject     DimCategory = "project"
	DimCategoryProductLine DimCategory = "product_line"
	DimCategoryTerritory   DimCategory = "territory"
	DimCategoryGrant       DimCategory = "grant"
)

type AllocBasis string

const (
	AllocBasisEqual      AllocBasis = "equal"
	AllocBasisPercentage AllocBasis = "percentage"
	AllocBasisFixed      AllocBasis = "fixed"
)

// AccountAnalyticDimension defines a structured analytic dimension for management reporting.
type AccountAnalyticDimension struct {
	sdk.Model `nexa:"model=account.analytic.dimension"`

	Name        sdk.String                                  `nexa:"required,string=Name"`
	Code        sdk.String                                  `nexa:"string=Code"`
	CompanyID   sdk.Many2One[CoreCompany]                   `nexa:"string=Company"`
	Category    sdk.Selection[DimCategory]                  `nexa:"string=Category,default=department"`
	Required    sdk.Boolean                                 `nexa:"string=Required on Posting"`
	Active      sdk.Boolean                                 `nexa:"string=Active,default=true"`
	Description sdk.Text                                    `nexa:"string=Description"`
	Values      sdk.One2Many[AccountAnalyticDimensionValue] `nexa:"string=Values"`
}

// AccountAnalyticDimensionValue is a valid value within a dimension.
type AccountAnalyticDimensionValue struct {
	sdk.Model `nexa:"model=account.analytic.dimension.value"`

	DimensionID       sdk.Many2One[AccountAnalyticDimension]      `nexa:"required,index,string=Dimension"`
	Name              sdk.String                                  `nexa:"required,string=Name"`
	Code              sdk.String                                  `nexa:"string=Code"`
	ParentID          sdk.Many2One[AccountAnalyticDimensionValue] `nexa:"string=Parent"`
	AnalyticAccountID sdk.Many2One[sdk.Any]                       `nexa:"string=Analytic Account,comodel=account.analytic.account"`
	Active            sdk.Boolean                                 `nexa:"string=Active,default=true"`
}

// AccountMoveLineDimension assigns (or allocates) a dimension value onto a journal item.
type AccountMoveLineDimension struct {
	sdk.Model `nexa:"model=account.move.line.dimension"`

	MoveLineID   sdk.Many2One[AccountMoveLine]               `nexa:"required,index,string=Journal Item"`
	AllocationID sdk.Many2One[AccountAnalyticAllocation]     `nexa:"string=Allocation"`
	DimensionID  sdk.Many2One[AccountAnalyticDimension]      `nexa:"required,string=Dimension"`
	ValueID      sdk.Many2One[AccountAnalyticDimensionValue] `nexa:"required,string=Value"`
	Amount       sdk.Numeric                                 `nexa:"string=Allocated Amount,precision=18,scale=2,default=0"`
	Percentage   sdk.Float                                   `nexa:"string=Percentage"`
}

// AccountAnalyticAllocation splits one journal item's amount across dimension values.
type AccountAnalyticAllocation struct {
	sdk.Model `nexa:"model=account.analytic.allocation"`

	SourceMoveLineID sdk.Many2One[AccountMoveLine]          `nexa:"index,string=Source Journal Item"`
	Name             sdk.String                             `nexa:"required,string=Name"`
	Basis            sdk.Selection[AllocBasis]              `nexa:"string=Basis,default=percentage"`
	TotalAmount      sdk.Numeric                            `nexa:"string=Total Amount,precision=18,scale=2,default=0"`
	Lines            sdk.One2Many[AccountMoveLineDimension] `nexa:"string=Allocations"`
	Note             sdk.Text                               `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("account_dimension",
		&AccountAnalyticDimension{},
		&AccountAnalyticDimensionValue{},
		&AccountMoveLineDimension{},
		&AccountAnalyticAllocation{},
	)
}

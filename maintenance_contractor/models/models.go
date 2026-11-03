package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
	relimodels "nexa_addons/maintenance_reliability/models"
)

// Cross-module reference aliases.
type (
	CoreUser                   = basemodels.CoreUser
	CorePartner                = basemodels.CorePartner
	MaintenanceAsset           = maintmodels.MaintenanceAsset
	MaintenanceWorkOrder       = maintmodels.MaintenanceWorkOrder
	MaintenanceWarranty        = relimodels.MaintenanceWarranty
	MaintenanceServiceContract = relimodels.MaintenanceServiceContract
)

// ---- Selection types -------------------------------------------------------

type DispatchState string

const (
	DispatchDraft        DispatchState = "draft"
	DispatchSent         DispatchState = "dispatched"
	DispatchAcknowledged DispatchState = "acknowledged"
	DispatchOnSite       DispatchState = "on_site"
	DispatchWorkDone     DispatchState = "work_done"
	DispatchVerified     DispatchState = "verified"
	DispatchInvoiced     DispatchState = "invoiced"
	DispatchRejected     DispatchState = "rejected"
	DispatchCancelled    DispatchState = "cancelled"
)

type PricingType string

const (
	PricingFixedPrice PricingType = "fixed_price"
	PricingTimeMaterial PricingType = "time_material"
	PricingCoveredContract PricingType = "contract_included"
	PricingWarrantyCovered PricingType = "warranty_covered"
)

type ClaimStatus string

const (
	ClaimStatusDraft        ClaimStatus = "draft"
	ClaimStatusSubmitted    ClaimStatus = "submitted"
	ClaimStatusAcknowledged ClaimStatus = "acknowledged"
	ClaimStatusApproved     ClaimStatus = "approved"
	ClaimStatusPartiallyApp ClaimStatus = "partially_approved"
	ClaimStatusRejected     ClaimStatus = "rejected"
	ClaimStatusReimbursed   ClaimStatus = "reimbursed"
	ClaimStatusClosed       ClaimStatus = "closed"
)

// ---- Models ----------------------------------------------------------------

// MaintContractorDispatch manages work order dispatches to third-party maintenance contractors.
type MaintContractorDispatch struct {
	sdk.Model `nexa:"model=maintenance.contractor.dispatch"`

	Name                 sdk.String                                 `nexa:"required,index,string=Dispatch Reference"`
	WorkOrderID          sdk.Many2One[MaintenanceWorkOrder]         `nexa:"required,index,string=Work Order"`
	ContractorID         sdk.Many2One[CorePartner]                  `nexa:"required,index,string=Contractor / Supplier"`
	ServiceContractID    sdk.Many2One[MaintenanceServiceContract]   `nexa:"string=Service Contract Reference"`
	State                sdk.Selection[DispatchState]               `nexa:"required,string=Status,default=draft"`
	PricingType          sdk.Selection[PricingType]                 `nexa:"required,string=Pricing Basis,default=fixed_price"`
	DispatchedAt         sdk.DateTime                               `nexa:"string=Dispatched At"`
	ExpectedArrival      sdk.DateTime                               `nexa:"string=Expected On-Site"`
	ActualArrival        sdk.DateTime                               `nexa:"string=Actual Arrival"`
	CompletedAt          sdk.DateTime                               `nexa:"string=Completed At"`
	ScopeOfWork          sdk.Text                                   `nexa:"required,string=Scope of Work"`
	EstimatedAmount      sdk.Numeric                                `nexa:"string=Estimated Amount ($),precision=18,scale=2,default=0"`
	ActualInvoicedAmount sdk.Numeric                                `nexa:"string=Invoiced Amount ($),precision=18,scale=2,default=0"`
	PurchaseOrderRef     sdk.String                                 `nexa:"string=Purchase Order Reference"`
	VendorInvoiceRef     sdk.String                                 `nexa:"string=Vendor Invoice Reference"`
	ContractorTechnician sdk.String                                 `nexa:"string=Contractor Technician Name"`
	QualityScore         sdk.Integer                                `nexa:"string=Quality Rating (1-5),default=5"`
	QualityVerifiedBy    sdk.Many2One[CoreUser]                     `nexa:"string=Verified By"`
	QualityVerifiedAt    sdk.DateTime                               `nexa:"string=Verified At"`
	WorkAcceptanceNotes  sdk.Text                                   `nexa:"string=Work Acceptance Notes"`
	Notes                sdk.Text                                   `nexa:"string=Notes"`
}

// MaintWarrantyClaimV2 manages manufacturer warranty claims, RMA authorization and cost recovery.
type MaintWarrantyClaimV2 struct {
	sdk.Model `nexa:"model=maintenance.warranty.claim"`

	Name                  sdk.String                       `nexa:"required,index,string=Claim Reference"`
	AssetID               sdk.Many2One[MaintenanceAsset]   `nexa:"required,index,string=Asset"`
	WarrantyID            sdk.Many2One[MaintenanceWarranty]`nexa:"string=Warranty Policy"`
	WorkOrderID           sdk.Many2One[MaintenanceWorkOrder]`nexa:"string=Related Work Order"`
	ManufacturerID        sdk.Many2One[CorePartner]        `nexa:"string=Manufacturer / Vendor"`
	State                 sdk.Selection[ClaimStatus]       `nexa:"required,string=Claim Status,default=draft"`
	ClaimDate             sdk.Date                         `nexa:"required,string=Claim Date"`
	SubmissionDate        sdk.Date                         `nexa:"string=Submission Date"`
	ResolutionDate        sdk.Date                         `nexa:"string=Resolution Date"`
	ClaimedAmount         sdk.Numeric                      `nexa:"string=Claimed Amount ($),precision=18,scale=2,default=0"`
	ApprovedAmount        sdk.Numeric                      `nexa:"string=Approved Amount ($),precision=18,scale=2,default=0"`
	ReimbursedAmount      sdk.Numeric                      `nexa:"string=Reimbursed Amount ($),precision=18,scale=2,default=0"`
	RMANumber             sdk.String                       `nexa:"string=RMA Number (Return Auth)"`
	ReplacementPartIssued sdk.Boolean                      `nexa:"string=Free Replacement Part Provided,default=false"`
	FailureDescription    sdk.Text                         `nexa:"required,string=Failure Description & Defect Evidence"`
	SettlementDetails     sdk.Text                         `nexa:"string=Settlement / Credit Note Details"`
	PreparedBy            sdk.Many2One[CoreUser]           `nexa:"string=Prepared By"`
	Notes                 sdk.Text                         `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("maintenance_contractor",
		&MaintContractorDispatch{},
		&MaintWarrantyClaimV2{},
	)
}

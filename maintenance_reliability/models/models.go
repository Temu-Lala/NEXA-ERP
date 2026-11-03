package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
)

// Cross-module reference aliases.
type (
	MaintenanceAsset     = maintmodels.MaintenanceAsset
	MaintenanceWorkOrder = maintmodels.MaintenanceWorkOrder
	CoreUser             = basemodels.CoreUser
	CorePartner          = basemodels.CorePartner
)

type DowntimeType string

const (
	DowntimePlanned   DowntimeType = "planned"
	DowntimeUnplanned DowntimeType = "unplanned"
)

type ContractState string

const (
	ContractDraft     ContractState = "draft"
	ContractActive    ContractState = "active"
	ContractExpired   ContractState = "expired"
	ContractRenewed   ContractState = "renewed"
	ContractCancelled ContractState = "cancelled"
)

type ClaimState string

const (
	ClaimNone      ClaimState = "none"
	ClaimPending   ClaimState = "pending"
	ClaimSubmitted ClaimState = "submitted"
	ClaimApproved  ClaimState = "approved"
	ClaimRejected  ClaimState = "rejected"
)

// MaintenanceFailureCode is a standardized failure mode, cause and action.
type MaintenanceFailureCode struct {
	sdk.Model `nexa:"model=maintenance.failure.code"`

	Code         sdk.String  `nexa:"required,index,string=Code"`
	Name         sdk.String  `nexa:"required,string=Description"`
	FailureMode  sdk.String  `nexa:"string=Failure Mode"`
	FailureCause sdk.String  `nexa:"string=Failure Cause"`
	Detection    sdk.String  `nexa:"string=Detection Method"`
	RepairAction sdk.String  `nexa:"string=Repair Action"`
	Active       sdk.Boolean `nexa:"string=Active,default=true"`
}

// MaintenanceDowntimeEvent records equipment downtime and return to service.
type MaintenanceDowntimeEvent struct {
	sdk.Model `nexa:"model=maintenance.downtime.event"`

	Name             sdk.String                         `nexa:"required,string=Reference"`
	AssetID          sdk.Many2One[MaintenanceAsset]     `nexa:"required,index,string=Asset"`
	WorkOrderID      sdk.Many2One[MaintenanceWorkOrder] `nexa:"string=Work Order"`
	DowntimeType     sdk.Selection[DowntimeType]        `nexa:"string=Type,default=unplanned"`
	FailureCode      sdk.String                         `nexa:"string=Failure Code"`
	StartTime        sdk.DateTime                       `nexa:"string=Downtime Start"`
	EndTime          sdk.DateTime                       `nexa:"string=Return to Service"`
	DurationHours    sdk.Numeric                        `nexa:"string=Duration (hours),precision=18,scale=2,default=0"`
	Impact           sdk.Text                           `nexa:"string=Production / Service Impact"`
	RootCause        sdk.Text                           `nexa:"string=Root Cause"`
	CorrectiveAction sdk.Text                           `nexa:"string=Corrective Action"`
	FollowUpDate     sdk.Date                           `nexa:"string=Follow-Up Date"`
}

// MaintenanceWarranty records warranty periods, coverage and claim references.
type MaintenanceWarranty struct {
	sdk.Model `nexa:"model=maintenance.warranty"`

	Name       sdk.String                     `nexa:"required,string=Warranty"`
	AssetID    sdk.Many2One[MaintenanceAsset] `nexa:"required,index,string=Asset"`
	ProviderID sdk.Many2One[CorePartner]      `nexa:"string=Provider / Supplier"`
	StartDate  sdk.Date                       `nexa:"string=Start Date"`
	EndDate    sdk.Date                       `nexa:"string=Expiration Date"`
	Coverage   sdk.Text                       `nexa:"string=Coverage"`
	Exclusions sdk.Text                       `nexa:"string=Exclusions"`
	ClaimRef   sdk.String                     `nexa:"string=Claim Reference"`
	ClaimState sdk.Selection[ClaimState]      `nexa:"string=Claim Status,default=none"`
	Active     sdk.Boolean                    `nexa:"string=Active,default=true"`
}

// MaintenanceServiceContract is a maintenance agreement with coverage terms and SLAs.
type MaintenanceServiceContract struct {
	sdk.Model `nexa:"model=maintenance.service.contract"`

	Name                  sdk.String                     `nexa:"required,index,string=Contract"`
	AssetID               sdk.Many2One[MaintenanceAsset] `nexa:"index,string=Asset"`
	ProviderID            sdk.Many2One[CorePartner]      `nexa:"string=Service Provider"`
	ResponsibleID         sdk.Many2One[CoreUser]         `nexa:"string=Internal Contact"`
	StartDate             sdk.Date                       `nexa:"string=Start Date"`
	EndDate               sdk.Date                       `nexa:"string=End Date"`
	State                 sdk.Selection[ContractState]   `nexa:"string=Status,default=draft"`
	Coverage              sdk.Text                       `nexa:"string=Coverage"`
	ResponseTargetHours   sdk.Integer                    `nexa:"string=Response Target (hours),default=0"`
	ResolutionTargetHours sdk.Integer                    `nexa:"string=Resolution Target (hours),default=0"`
	ContractValue         sdk.Numeric                    `nexa:"string=Contract Value,precision=18,scale=2,default=0"`
	RenewalReminderDate   sdk.Date                       `nexa:"string=Renewal Reminder"`
	Notes                 sdk.Text                       `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("maintenance_reliability",
		&MaintenanceFailureCode{},
		&MaintenanceDowntimeEvent{},
		&MaintenanceWarranty{},
		&MaintenanceServiceContract{},
	)
}

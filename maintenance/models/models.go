package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Cross-module reference aliases.
type (
	CoreUser    = basemodels.CoreUser
	CorePartner = basemodels.CorePartner
)

// ---- Selection types -------------------------------------------------------

type AssetState string

const (
	AssetDraft            AssetState = "draft"
	AssetActive           AssetState = "active"
	AssetUnderMaintenance AssetState = "under_maintenance"
	AssetOutOfService     AssetState = "out_of_service"
	AssetRetired          AssetState = "retired"
	AssetArchived         AssetState = "archived"
)

type Criticality string

const (
	CritLow      Criticality = "low"
	CritNormal   Criticality = "normal"
	CritHigh     Criticality = "high"
	CritCritical Criticality = "critical"
)

type Ownership string

const (
	OwnOwned    Ownership = "owned"
	OwnLeased   Ownership = "leased"
	OwnRented   Ownership = "rented"
	OwnOnBehalf Ownership = "customer"
)

type MaintStrategy string

const (
	StratPreventive MaintStrategy = "preventive"
	StratCorrective MaintStrategy = "corrective"
	StratCondition  MaintStrategy = "condition_based"
	StratPredictive MaintStrategy = "predictive"
	StratEmergency  MaintStrategy = "emergency"
)

type RequestState string

const (
	ReqSubmitted RequestState = "submitted"
	ReqTriage    RequestState = "triage"
	ReqApproved  RequestState = "approved"
	ReqConverted RequestState = "converted"
	ReqRejected  RequestState = "rejected"
	ReqCancelled RequestState = "cancelled"
	ReqDuplicate RequestState = "duplicate"
)

type WorkOrderState string

const (
	WoDraft      WorkOrderState = "draft"
	WoPlanned    WorkOrderState = "planned"
	WoReady      WorkOrderState = "ready"
	WoInProgress WorkOrderState = "in_progress"
	WoOnHold     WorkOrderState = "on_hold"
	WoCompleted  WorkOrderState = "completed"
	WoVerified   WorkOrderState = "verified"
	WoClosed     WorkOrderState = "closed"
	WoCancelled  WorkOrderState = "cancelled"
)

type LineType string

const (
	LineTask    LineType = "task"
	LinePart    LineType = "part"
	LineSafety  LineType = "safety"
	LineInspect LineType = "inspection"
)

type LineState string

const (
	LinePending LineState = "pending"
	LineDone    LineState = "done"
	LineSkipped LineState = "skipped"
)

type MaintPriority string

const (
	PrioLow    MaintPriority = "low"
	PrioNormal MaintPriority = "normal"
	PrioHigh   MaintPriority = "high"
	PrioUrgent MaintPriority = "urgent"
)

type CostType string

const (
	CostLabor    CostType = "labor"
	CostParts    CostType = "parts"
	CostExternal CostType = "external_service"
	CostRental   CostType = "rental"
	CostOther    CostType = "other"
)

// ---- Models ----------------------------------------------------------------

// MaintenanceAssetCategory is a hierarchical asset classification.
type MaintenanceAssetCategory struct {
	sdk.Model `nexa:"model=maintenance.asset.category"`

	Name     sdk.String                             `nexa:"required,index,string=Category"`
	Code     sdk.String                             `nexa:"string=Code"`
	ParentID sdk.Many2One[MaintenanceAssetCategory] `nexa:"string=Parent Category"`
	Active   sdk.Boolean                            `nexa:"string=Active,default=true"`
	Notes    sdk.Text                               `nexa:"string=Notes"`
}

// MaintenanceAssetLocation is a site, building, room or GPS position.
type MaintenanceAssetLocation struct {
	sdk.Model `nexa:"model=maintenance.asset.location"`

	Name     sdk.String                             `nexa:"required,index,string=Location"`
	Code     sdk.String                             `nexa:"string=Code"`
	ParentID sdk.Many2One[MaintenanceAssetLocation] `nexa:"string=Parent Location"`
	Site     sdk.String                             `nexa:"string=Site"`
	Room     sdk.String                             `nexa:"string=Room"`
	GPS      sdk.String                             `nexa:"string=GPS Coordinate"`
	Active   sdk.Boolean                            `nexa:"string=Active,default=true"`
}

// MaintenanceAsset is the canonical maintainable equipment record.
type MaintenanceAsset struct {
	sdk.Model `nexa:"model=maintenance.asset"`

	Name              sdk.String                             `nexa:"required,index,string=Asset Name"`
	Code              sdk.String                             `nexa:"index,string=Asset Code"`
	SerialNumber      sdk.String                             `nexa:"string=Serial Number"`
	Barcode           sdk.String                             `nexa:"string=Barcode / QR"`
	CategoryID        sdk.Many2One[MaintenanceAssetCategory] `nexa:"string=Category"`
	LocationID        sdk.Many2One[MaintenanceAssetLocation] `nexa:"string=Location"`
	ParentAssetID     sdk.Many2One[MaintenanceAsset]         `nexa:"string=Parent Asset"`
	Manufacturer      sdk.String                             `nexa:"string=Manufacturer"`
	ModelRef          sdk.String                             `nexa:"string=Model"`
	Specifications    sdk.Text                               `nexa:"string=Specifications"`
	Criticality       sdk.Selection[Criticality]             `nexa:"string=Criticality,default=normal"`
	State             sdk.Selection[AssetState]              `nexa:"string=Status,default=draft"`
	Ownership         sdk.Selection[Ownership]               `nexa:"string=Ownership,default=owned"`
	Strategy          sdk.Selection[MaintStrategy]           `nexa:"string=Maintenance Strategy,default=preventive"`
	ResponsibleID     sdk.Many2One[CoreUser]                 `nexa:"string=Responsible"`
	OwnerPartnerID    sdk.Many2One[CorePartner]              `nexa:"string=Asset Owner"`
	CustomerID        sdk.Many2One[sdk.Any]                  `nexa:"comodel=core.partner,string=Customer"`
	SupplierID        sdk.Many2One[sdk.Any]                  `nexa:"comodel=core.partner,string=Supplier"`
	AcquisitionDate   sdk.Date                               `nexa:"string=Acquisition Date"`
	InstallDate       sdk.Date                               `nexa:"string=Installation Date"`
	CommissionedDate  sdk.Date                               `nexa:"string=Commissioned Date"`
	RetiredDate       sdk.Date                               `nexa:"string=Retired Date"`
	ExpectedLifeYears sdk.Integer                            `nexa:"string=Expected Service Life (yrs),default=0"`
	AcquisitionCost   sdk.Numeric                            `nexa:"string=Acquisition Cost,precision=18,scale=2,default=0"`
	WarrantyEnd       sdk.Date                               `nexa:"string=Warranty End"`
	UsageMeterName    sdk.String                             `nexa:"string=Usage Meter"`
	CurrentReading    sdk.Numeric                            `nexa:"string=Current Reading,precision=18,scale=2,default=0"`
	NextDueDate       sdk.Date                               `nexa:"string=Next Due Maintenance"`
	Description       sdk.Text                               `nexa:"string=Description"`
	Notes             sdk.Text                               `nexa:"string=Notes"`
	Active            sdk.Boolean                            `nexa:"string=Active,default=true"`
}

// MaintenanceRequest is a reported fault or service request.
type MaintenanceRequest struct {
	sdk.Model `nexa:"model=maintenance.request"`

	Name          sdk.String                         `nexa:"required,string=Subject"`
	AssetID       sdk.Many2One[MaintenanceAsset]     `nexa:"index,string=Asset"`
	Equipment     sdk.String                         `nexa:"string=Equipment (free text)"`
	RequesterID   sdk.Many2One[CoreUser]             `nexa:"string=Requested By"`
	Department    sdk.String                         `nexa:"string=Department"`
	AssignedTo    sdk.Many2One[CoreUser]             `nexa:"string=Assigned To"`
	Priority      sdk.Selection[MaintPriority]       `nexa:"string=Priority,default=normal"`
	State         sdk.Selection[RequestState]        `nexa:"string=Status,default=submitted"`
	Strategy      sdk.Selection[MaintStrategy]       `nexa:"string=Maintenance Type,default=corrective"`
	RequestDate   sdk.Date                           `nexa:"string=Request Date"`
	RequestedDate sdk.Date                           `nexa:"string=Requested Completion"`
	SafetyConcern sdk.Boolean                        `nexa:"string=Safety Concern,default=false"`
	Description   sdk.Text                           `nexa:"string=Problem Description"`
	WorkOrderID   sdk.Many2One[MaintenanceWorkOrder] `nexa:"string=Converted Work Order"`
}

// MaintenanceWorkOrder is the core operational repair / service job.
type MaintenanceWorkOrder struct {
	sdk.Model `nexa:"model=maintenance.work.order"`

	Name           sdk.String                                   `nexa:"required,index,string=Reference"`
	Title          sdk.String                                   `nexa:"string=Title"`
	AssetID        sdk.Many2One[MaintenanceAsset]               `nexa:"required,index,string=Asset"`
	LocationID     sdk.Many2One[MaintenanceAssetLocation]       `nexa:"string=Location"`
	RequestID      sdk.Many2One[MaintenanceRequest]             `nexa:"string=Source Request"`
	WorkType       sdk.Selection[MaintStrategy]                 `nexa:"string=Work Type,default=corrective"`
	Priority       sdk.Selection[MaintPriority]                 `nexa:"string=Priority,default=normal"`
	State          sdk.Selection[WorkOrderState]                `nexa:"string=Status,default=draft"`
	TechnicianID   sdk.Many2One[CoreUser]                       `nexa:"string=Technician"`
	Team           sdk.String                                   `nexa:"string=Maintenance Team"`
	PlannedStart   sdk.Date                                     `nexa:"string=Planned Start"`
	PlannedFinish  sdk.Date                                     `nexa:"string=Planned Finish"`
	ActualStart    sdk.Date                                     `nexa:"string=Actual Start"`
	ActualFinish   sdk.Date                                     `nexa:"string=Actual Finish"`
	Diagnosis      sdk.Text                                     `nexa:"string=Diagnosis"`
	RepairPlan     sdk.Text                                     `nexa:"string=Work Instructions"`
	Findings       sdk.Text                                     `nexa:"string=Findings"`
	SafetyRequired sdk.Boolean                                  `nexa:"string=Safety Checklist Required,default=false"`
	SafetyDone     sdk.Boolean                                  `nexa:"string=Safety Checklist Completed,default=false"`
	RequireVerify  sdk.Boolean                                  `nexa:"string=Require Verification,default=false"`
	VerifiedBy     sdk.Many2One[CoreUser]                       `nexa:"string=Verified By"`
	EstimatedHours sdk.Numeric                                  `nexa:"string=Estimated Hours,precision=18,scale=2,default=0"`
	ActualHours    sdk.Numeric                                  `nexa:"string=Actual Hours,precision=18,scale=2,default=0"`
	Billable       sdk.Boolean                                  `nexa:"string=Billable,default=false"`
	Lines          sdk.One2Many[MaintenanceWorkOrderLine]       `nexa:"string=Lines"`
	Assignments    sdk.One2Many[MaintenanceWorkOrderAssignment] `nexa:"string=Assignments"`
	Costs          sdk.One2Many[MaintenanceCost]                `nexa:"string=Costs"`
	Notes          sdk.Text                                     `nexa:"string=Notes"`
}

// MaintenanceWorkOrderLine is a task, part, safety or inspection step on a WO.
type MaintenanceWorkOrderLine struct {
	sdk.Model `nexa:"model=maintenance.work.order.line"`

	WorkOrderID sdk.Many2One[MaintenanceWorkOrder] `nexa:"required,index,string=Work Order"`
	LineType    sdk.Selection[LineType]            `nexa:"string=Type,default=task"`
	Name        sdk.String                         `nexa:"required,string=Description"`
	ProductID   sdk.Many2One[sdk.Any]              `nexa:"comodel=product.product,string=Spare Part"`
	PlannedQty  sdk.Numeric                        `nexa:"string=Planned Qty,precision=18,scale=2,default=0"`
	ConsumedQty sdk.Numeric                        `nexa:"string=Consumed Qty,precision=18,scale=2,default=0"`
	State       sdk.Selection[LineState]           `nexa:"string=Status,default=pending"`
	Required    sdk.Boolean                        `nexa:"string=Required,default=false"`
	Sequence    sdk.Integer                        `nexa:"string=Sequence,default=10"`
	Result      sdk.Text                           `nexa:"string=Result"`
}

// MaintenanceWorkOrderAssignment records a technician or team assigned to a WO.
type MaintenanceWorkOrderAssignment struct {
	sdk.Model `nexa:"model=maintenance.work.order.assignment"`

	WorkOrderID  sdk.Many2One[MaintenanceWorkOrder] `nexa:"required,index,string=Work Order"`
	UserID       sdk.Many2One[CoreUser]             `nexa:"required,string=Technician"`
	Role         sdk.String                         `nexa:"string=Role,default=technician"`
	PlannedHours sdk.Numeric                        `nexa:"string=Planned Hours,precision=18,scale=2,default=0"`
	ActualHours  sdk.Numeric                        `nexa:"string=Actual Hours,precision=18,scale=2,default=0"`
	Active       sdk.Boolean                        `nexa:"string=Active,default=true"`
}

// MaintenanceCost is a source-linked operational cost on a work order.
type MaintenanceCost struct {
	sdk.Model `nexa:"model=maintenance.cost"`

	WorkOrderID sdk.Many2One[MaintenanceWorkOrder] `nexa:"required,index,string=Work Order"`
	AssetID     sdk.Many2One[MaintenanceAsset]     `nexa:"index,string=Asset"`
	Date        sdk.Date                           `nexa:"required,string=Date"`
	CostType    sdk.Selection[CostType]            `nexa:"required,string=Cost Type,default=labor"`
	Description sdk.Text                           `nexa:"string=Description"`
	Qty         sdk.Numeric                        `nexa:"string=Qty / Hours,precision=18,scale=2,default=0"`
	UnitCost    sdk.Numeric                        `nexa:"string=Unit Cost,precision=18,scale=2,default=0"`
	Amount      sdk.Numeric                        `nexa:"string=Amount,precision=18,scale=2,default=0"`
	Billable    sdk.Boolean                        `nexa:"string=Billable,default=false"`
	SourceType  sdk.String                         `nexa:"string=Source Reference"`
}

func init() {
	sdk.MustRegister("maintenance",
		&MaintenanceAssetCategory{},
		&MaintenanceAssetLocation{},
		&MaintenanceAsset{},
		&MaintenanceRequest{},
		&MaintenanceWorkOrder{},
		&MaintenanceWorkOrderLine{},
		&MaintenanceWorkOrderAssignment{},
		&MaintenanceCost{},
	)
}

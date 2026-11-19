package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// ITAM/CMDB: assets, configuration items with relationships, licenses.

type CoreUser = basemodels.CoreUser

// ItamAsset is a tracked technology asset.
type ItamAsset struct {
	sdk.Model `nexa:"model=itam.asset"`

	Name             sdk.String                `nexa:"required,string=Asset Name"`
	AssetTag         sdk.String                `nexa:"required,index,string=Asset Tag"`
	Kind             sdk.Selection[AssetKind]  `nexa:"string=Type,default=hardware,selection=hardware:Hardware,software:Software,cloud:Cloud,network:Network,mobile:Mobile,other:Other"`
	SerialNo         sdk.String                `nexa:"string=Serial Number"`
	DeviceModel      sdk.String                `nexa:"string=Model"`
	Vendor           sdk.String                `nexa:"string=Vendor"`
	PurchaseDate     sdk.Date                  `nexa:"string=Purchase Date"`
	WarrantyUntil    sdk.Date                  `nexa:"string=Warranty Until"`
	AssignedToID     sdk.Many2One[CoreUser]    `nexa:"string=Assigned To"`
	Department       sdk.String                `nexa:"string=Department"`
	Location         sdk.String                `nexa:"string=Location"`
	State            sdk.Selection[AssetState] `nexa:"string=Status,default=in_stock,selection=in_stock:In Stock,assigned:Assigned,in_repair:In Repair,retired:Retired,disposed:Disposed"`
	DisposalDate     sdk.Date                  `nexa:"string=Disposal Date"`
	Note             sdk.Text                  `nexa:"string=Note"`
	WarrantyExpiring sdk.Boolean               `nexa:"string=Warranty Expiring,default=false"`
}

type AssetKind string

const (
	AKHardware AssetKind = "hardware"
	AKSoftware AssetKind = "software"
	AKCloud    AssetKind = "cloud"
	AKNetwork  AssetKind = "network"
	AKMobile   AssetKind = "mobile"
	AKOther    AssetKind = "other"
)

type AssetState string

const (
	AStateInStock  AssetState = "in_stock"
	AStateAssigned AssetState = "assigned"
	AStateInRepair AssetState = "in_repair"
	AStateRetired  AssetState = "retired"
	AStateDisposed AssetState = "disposed"
)

// ItamCI is a configuration item — any node in the service map.
type ItamCI struct {
	sdk.Model `nexa:"model=itam.ci"`

	Name        sdk.String                   `nexa:"required,string=CI Name"`
	Code        sdk.String                   `nexa:"string=CI ID"`
	Kind        sdk.Selection[CIKind]        `nexa:"string=Type,default=server,selection=server:Server,application:Application,database:Database,network:Network,service:Service,other:Other"`
	AssetID     sdk.Many2One[ItamAsset]      `nexa:"index,string=Asset"`
	Criticality sdk.Selection[CICrit]        `nexa:"string=Criticality,default=medium,selection=low:Low,medium:Medium,high:High,critical:Critical"`
	State       sdk.Selection[CIState]       `nexa:"string=Status,default=active,selection=active:Active,maintenance:Maintenance,retired:Retired"`
	Description sdk.Text                     `nexa:"string=Description"`
	Relations   sdk.One2Many[ItamCIRelation] `nexa:"string=Dependencies"`
}

type CIKind string

const (
	CIKindServer      CIKind = "server"
	CIKindApplication CIKind = "application"
	CIKindDatabase    CIKind = "database"
	CIKindNetwork     CIKind = "network"
	CIKindService     CIKind = "service"
	CIKindOther       CIKind = "other"
)

type CICrit string

const (
	CICritLow      CICrit = "low"
	CICritMedium   CICrit = "medium"
	CICritHigh     CICrit = "high"
	CICritCritical CICrit = "critical"
)

type CIState string

const (
	CIStateActive      CIState = "active"
	CIStateMaintenance CIState = "maintenance"
	CIStateRetired     CIState = "retired"
)

// ItamCIRelation is a directed dependency: CI depends_on CI2.
type ItamCIRelation struct {
	sdk.Model `nexa:"model=itam.ci.relation"`

	CIID        sdk.Many2One[ItamCI] `nexa:"required,index,ondelete=cascade,string=Configuration Item"`
	DependsOnID sdk.Many2One[ItamCI] `nexa:"required,index,string=Depends On"`
	Relation    sdk.String           `nexa:"string=Relation Type,default=depends_on"`
}

// ItamLicense is a software license with seat tracking.
type ItamLicense struct {
	sdk.Model `nexa:"model=itam.license"`

	Name           sdk.String              `nexa:"required,string=License"`
	Product        sdk.String              `nexa:"required,string=Product/Software"`
	Vendor         sdk.String              `nexa:"string=Vendor"`
	SeatsTotal     sdk.Integer             `nexa:"string=Seats,default=0"`
	SeatsUsed      sdk.Integer             `nexa:"string=Seats Used,default=0"`
	ExpiryDate     sdk.Date                `nexa:"string=Expiry Date"`
	RenewalDate    sdk.Date                `nexa:"string=Renewal Date"`
	Cost           sdk.Numeric             `nexa:"string=Cost,precision=18,scale=2,default=0"`
	State          sdk.Selection[LicState] `nexa:"string=Status,default=active,selection=active:Active,expired:Expired,suspended:Suspended"`
	Note           sdk.Text                `nexa:"string=Note"`
	UtilizationPct sdk.Numeric             `nexa:"string=Utilization %,precision=18,scale=2,default=0"`
}

type LicState string

const (
	LicActive    LicState = "active"
	LicExpired   LicState = "expired"
	LicSuspended LicState = "suspended"
)

func init() {
	sdk.MustRegister("itam",
		&ItamAsset{},
		&ItamCI{},
		&ItamCIRelation{},
		&ItamLicense{},
	)
}

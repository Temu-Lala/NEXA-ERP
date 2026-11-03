package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
	equipmodels "nexa_addons/maintenance_equipment/models"
)

// Cross-module reference aliases.
type (
	MaintenanceAsset     = maintmodels.MaintenanceAsset
	MaintenanceWorkOrder = maintmodels.MaintenanceWorkOrder
	MaintenancePlan      = equipmodels.MaintenancePlan
	CoreUser             = basemodels.CoreUser
)

// DocState is the publication lifecycle of a knowledge document.
type DocState string

const (
	DocDraft     DocState = "draft"
	DocInReview  DocState = "in_review"
	DocPublished DocState = "published"
	DocObsolete  DocState = "obsolete"
)

// DocType classifies the kind of technical document.
type DocType string

const (
	DocManual                 DocType = "manual"
	DocDiagram                DocType = "diagram"
	DocProcedure              DocType = "procedure"
	DocWorkInstruction        DocType = "work_instruction"
	DocTroubleshooting        DocType = "troubleshooting"
	DocBulletin               DocType = "bulletin"
	DocInvestigationReport    DocType = "investigation_report"
	DocSafetyProcedure        DocType = "safety_procedure"
	DocCalibrationInstruction DocType = "calibration_instruction"
	DocDrawing                DocType = "drawing"
	DocChecklist              DocType = "checklist"
	DocOther                  DocType = "other"
)

// LinkType describes how a document is attached to a record.
type LinkType string

const (
	LinkReference     LinkType = "reference"
	LinkProcedure     LinkType = "procedure"
	LinkManual        LinkType = "manual"
	LinkDrawing       LinkType = "drawing"
	LinkEvidence      LinkType = "evidence"
	LinkSafety        LinkType = "safety"
	LinkInvestigation LinkType = "investigation"
)

// MaintenanceDocumentCategory is a hierarchical, optionally sensitive, document grouping.
type MaintenanceDocumentCategory struct {
	sdk.Model `nexa:"model=maintenance.document.category"`

	Name        sdk.String                                `nexa:"required,index,string=Name"`
	Code        sdk.String                                `nexa:"required,index,string=Code"`
	ParentID    sdk.Many2One[MaintenanceDocumentCategory] `nexa:"string=Parent Category"`
	Sensitive   sdk.Boolean                               `nexa:"string=Access Restricted,default=false"`
	Description sdk.Text                                  `nexa:"string=Description"`
	Active      sdk.Boolean                               `nexa:"string=Active,default=true"`
}

// MaintenanceDocument is a versioned technical document in the knowledge library.
type MaintenanceDocument struct {
	sdk.Model `nexa:"model=maintenance.document"`

	Reference      sdk.String                                `nexa:"required,index,string=Reference"`
	Name           sdk.String                                `nexa:"required,index,string=Title"`
	CategoryID     sdk.Many2One[MaintenanceDocumentCategory] `nexa:"index,string=Category"`
	DocType        sdk.Selection[DocType]                    `nexa:"string=Type,default=procedure"`
	State          sdk.Selection[DocState]                   `nexa:"string=Status,default=draft"`
	AssetModel     sdk.String                                `nexa:"index,string=Applies To (Model)"`
	Manufacturer   sdk.String                                `nexa:"string=Manufacturer"`
	AssetID        sdk.Many2One[MaintenanceAsset]            `nexa:"string=Specific Asset"`
	CurrentVersion sdk.String                                `nexa:"string=Current Version,default=1.0"`
	Restricted     sdk.Boolean                               `nexa:"string=Access Restricted,default=false"`
	EffectiveDate  sdk.Date                                  `nexa:"string=Effective Date"`
	ExpiryDate     sdk.Date                                  `nexa:"string=Review / Expiry Date"`
	OwnerID        sdk.Many2One[CoreUser]                    `nexa:"string=Owner"`
	Tags           sdk.String                                `nexa:"string=Tags"`
	Keywords       sdk.String                                `nexa:"string=Keywords"`
	Summary        sdk.Text                                  `nexa:"string=Summary"`
	Content        sdk.Text                                  `nexa:"string=Content"`
	FileName       sdk.String                                `nexa:"string=File Name"`
	FileRef        sdk.String                                `nexa:"string=File Reference"`
	FileMime       sdk.String                                `nexa:"string=File Type"`
	ViewCount      sdk.Integer                               `nexa:"string=Views,default=0"`
	Versions       sdk.One2Many[MaintenanceDocumentVersion]  `nexa:"string=Versions"`
	Links          sdk.One2Many[MaintenanceDocumentLink]     `nexa:"string=Links"`
	Active         sdk.Boolean                               `nexa:"string=Active,default=true"`
}

// MaintenanceDocumentVersion records the immutable version history of a document so the
// exact revision in force during a job remains identifiable.
type MaintenanceDocumentVersion struct {
	sdk.Model `nexa:"model=maintenance.document.version"`

	DocumentID    sdk.Many2One[MaintenanceDocument] `nexa:"required,index,string=Document"`
	Version       sdk.String                        `nexa:"required,index,string=Version"`
	ChangeNote    sdk.Text                          `nexa:"string=Change Note"`
	FileName      sdk.String                        `nexa:"string=File Name"`
	FileRef       sdk.String                        `nexa:"string=File Reference"`
	FileMime      sdk.String                        `nexa:"string=File Type"`
	EffectiveDate sdk.Date                          `nexa:"string=Effective Date"`
	ReleasedByID  sdk.Many2One[CoreUser]            `nexa:"string=Released By"`
	IsCurrent     sdk.Boolean                       `nexa:"string=Current Revision,default=false"`
}

// MaintenanceDocumentLink attaches a document (optionally a pinned version) to assets,
// work orders and maintenance plans.
type MaintenanceDocumentLink struct {
	sdk.Model `nexa:"model=maintenance.document.link"`

	DocumentID  sdk.Many2One[MaintenanceDocument]        `nexa:"required,index,string=Document"`
	VersionID   sdk.Many2One[MaintenanceDocumentVersion] `nexa:"string=Version Used"`
	AssetID     sdk.Many2One[MaintenanceAsset]           `nexa:"index,string=Asset"`
	WorkOrderID sdk.Many2One[MaintenanceWorkOrder]       `nexa:"index,string=Work Order"`
	PlanID      sdk.Many2One[MaintenancePlan]            `nexa:"index,string=Maintenance Plan"`
	LinkType    sdk.Selection[LinkType]                  `nexa:"string=Link Type,default=reference"`
	Note        sdk.Text                                 `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("maintenance_knowledge",
		&MaintenanceDocumentCategory{},
		&MaintenanceDocument{},
		&MaintenanceDocumentVersion{},
		&MaintenanceDocumentLink{},
	)
}

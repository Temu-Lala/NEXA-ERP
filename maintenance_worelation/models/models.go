package models

import (
	"nexa/core/sdk"
	maintmodels "nexa_addons/maintenance/models"
)

// Cross-module reference aliases.
type (
	MaintenanceWorkOrder = maintmodels.MaintenanceWorkOrder
)

// RelationType describes how two work orders relate (hierarchy, dependency or duplication).
type RelationType string

const (
	RelParentChild RelationType = "parent_child"
	RelBlocks      RelationType = "blocks"
	RelPredecessor RelationType = "predecessor"
	RelDuplicate   RelationType = "duplicate"
	RelRelated     RelationType = "related"
)

// MaintenanceWorkOrderRelation links two work orders to express parent/child hierarchy,
// scheduling dependencies or duplicate detection.
type MaintenanceWorkOrderRelation struct {
	sdk.Model `nexa:"model=maintenance.work.order.relation"`

	Name        sdk.String                         `nexa:"required,string=Reference"`
	SourceWoID  sdk.Many2One[MaintenanceWorkOrder] `nexa:"required,index,string=Source Work Order"`
	TargetWoID  sdk.Many2One[MaintenanceWorkOrder] `nexa:"required,index,string=Target Work Order"`
	Type        sdk.Selection[RelationType]        `nexa:"required,index,string=Relation Type,default=related"`
	LagHours    sdk.Numeric                        `nexa:"string=Lag (hours),precision=18,scale=2,default=0"`
	Description sdk.Text                           `nexa:"string=Note"`
	Active      sdk.Boolean                        `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("maintenance_worelation",
		&MaintenanceWorkOrderRelation{},
	)
}

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
)

type PartRequestState string

const (
	PartReqDraft     PartRequestState = "draft"
	PartReqSubmitted PartRequestState = "submitted"
	PartReqReserved  PartRequestState = "reserved"
	PartReqPurchase  PartRequestState = "to_purchase"
	PartReqReceived  PartRequestState = "received"
	PartReqDone      PartRequestState = "done"
	PartReqCancelled PartRequestState = "cancelled"
)

// MaintenancePartRequest is a request to reserve or source parts for a job.
type MaintenancePartRequest struct {
	sdk.Model `nexa:"model=maintenance.part.request"`

	Name         sdk.String                         `nexa:"required,string=Reference"`
	WorkOrderID  sdk.Many2One[MaintenanceWorkOrder] `nexa:"index,string=Work Order"`
	AssetID      sdk.Many2One[MaintenanceAsset]     `nexa:"index,string=Asset"`
	ProductID    sdk.Many2One[sdk.Any]              `nexa:"comodel=product.product,required,string=Spare Part"`
	RequestedQty sdk.Numeric                        `nexa:"string=Requested Qty,precision=18,scale=2,default=0"`
	ReservedQty  sdk.Numeric                        `nexa:"string=Reserved Qty,precision=18,scale=2,default=0"`
	RequestedBy  sdk.Many2One[CoreUser]             `nexa:"string=Requested By"`
	Date         sdk.Date                           `nexa:"string=Request Date"`
	NeedDate     sdk.Date                           `nexa:"string=Needed By"`
	State        sdk.Selection[PartRequestState]    `nexa:"string=Status,default=draft"`
	PurchaseRef  sdk.String                         `nexa:"string=Purchase Reference"`
	Note         sdk.Text                           `nexa:"string=Note"`
}

// MaintenancePartConsumed records parts actually consumed or returned on a work order.
type MaintenancePartConsumed struct {
	sdk.Model `nexa:"model=maintenance.part.consumed"`

	WorkOrderID  sdk.Many2One[MaintenanceWorkOrder] `nexa:"required,index,string=Work Order"`
	ProductID    sdk.Many2One[sdk.Any]              `nexa:"comodel=product.product,required,string=Spare Part"`
	Quantity     sdk.Numeric                        `nexa:"string=Quantity,precision=18,scale=2,default=0"`
	ReturnedQty  sdk.Numeric                        `nexa:"string=Returned Qty,precision=18,scale=2,default=0"`
	SerialNumber sdk.String                         `nexa:"string=Serial Number"`
	Lot          sdk.String                         `nexa:"string=Lot / Batch"`
	Date         sdk.Date                           `nexa:"string=Date"`
	Note         sdk.Text                           `nexa:"string=Note"`
}

// MaintenanceComponent records a component installed on or part of an asset.
type MaintenanceComponent struct {
	sdk.Model `nexa:"model=maintenance.component"`

	Name         sdk.String                     `nexa:"required,string=Component"`
	AssetID      sdk.Many2One[MaintenanceAsset] `nexa:"required,index,string=Parent Asset"`
	ChildAssetID sdk.Many2One[MaintenanceAsset] `nexa:"string=Child Asset"`
	ProductID    sdk.Many2One[sdk.Any]          `nexa:"comodel=product.product,string=Spare Part"`
	SerialNumber sdk.String                     `nexa:"string=Serial Number"`
	InstallDate  sdk.Date                       `nexa:"string=Install Date"`
	RemoveDate   sdk.Date                       `nexa:"string=Remove Date"`
	Quantity     sdk.Numeric                    `nexa:"string=Quantity,precision=18,scale=2,default=1"`
	Note         sdk.Text                       `nexa:"string=Note"`
}

func init() {
	sdk.MustRegister("maintenance_parts",
		&MaintenancePartRequest{},
		&MaintenancePartConsumed{},
		&MaintenanceComponent{},
	)
}

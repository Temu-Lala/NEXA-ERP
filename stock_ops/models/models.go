package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	productmodels "nexa_addons/product/models"
	stockmodels "nexa_addons/stock/models"
)

// Warehouse Operations (blueprint §16, plus putaway ranking from §3.3/§5.4).
// Tasks are work records that reference ledger documents — they never move
// stock themselves; execution still posts through the anchor's validated
// transfer engine. Waves group outgoing transfers and fan the work out into
// consolidated pick tasks. Every scan lands in an append-only log and a
// mismatch blocks task completion.

type (
	ProductProduct = productmodels.ProductProduct
	CoreUser       = basemodels.CoreUser
	StockLot       = stockmodels.StockLot
	StockPicking   = stockmodels.StockPicking
	StockMove      = stockmodels.StockMove
	StockLocation  = stockmodels.StockLocation
	StockWarehouse = stockmodels.StockWarehouse
)

// ---- Selection types -------------------------------------------------------

type TaskType string

const (
	TaskReceive   TaskType = "receive"
	TaskInspect   TaskType = "inspect"
	TaskPutaway   TaskType = "putaway"
	TaskPick      TaskType = "pick"
	TaskPack      TaskType = "pack"
	TaskCount     TaskType = "count"
	TaskReturn    TaskType = "return"
	TaskTransport TaskType = "transport"
)

type TaskState string

const (
	TaskDraft     TaskState = "draft"
	TaskAssigned  TaskState = "assigned"
	TaskDoing     TaskState = "in_progress"
	TaskDone      TaskState = "done"
	TaskCancelled TaskState = "cancelled"
)

type WaveState string

const (
	WaveDraft     WaveState = "draft"
	WaveFinalized WaveState = "finalized"
	WaveReleased  WaveState = "released"
	WaveDone      WaveState = "done"
	WaveCancelled WaveState = "cancelled"
)

// ---- Models ----------------------------------------------------------------

// StockOperationTask is one unit of warehouse work referencing a transfer
// and/or move with an optional zone, assignee and scan evidence (§16.1).
type StockOperationTask struct {
	sdk.Model      `nexa:"model=stock.operation.task"`
	Name           sdk.String                   `nexa:"required,index,string=Reference"`
	Type           sdk.Selection[TaskType]      `nexa:"required,index,string=Task Type,default=pick"`
	State          sdk.Selection[TaskState]     `nexa:"string=Status,default=draft"`
	Priority       sdk.Integer                  `nexa:"string=Priority,default=0"`
	WaveID         sdk.Many2One[StockWave]      `nexa:"index,string=Wave"`
	PickingID      sdk.Many2One[StockPicking]   `nexa:"index,string=Transfer"`
	MoveID         sdk.Many2One[StockMove]      `nexa:"string=Move"`
	ProductID      sdk.Many2One[ProductProduct] `nexa:"index,string=Product"`
	LocationID     sdk.Many2One[StockLocation]  `nexa:"string=Source Zone"`
	DestLocationID sdk.Many2One[StockLocation]  `nexa:"string=Destination"`
	Quantity       sdk.Numeric                  `nexa:"string=Quantity,precision=18,scale=3,default=0"`
	AssigneeID     sdk.Many2One[CoreUser]       `nexa:"index,string=Assignee"`
	ScanCode       sdk.String                   `nexa:"string=Scan Code"`
	ScanMatch      sdk.Boolean                  `nexa:"string=Scan Verified,default=false"`
	OverrideReason sdk.Text                     `nexa:"string=Override Reason"`
	Note           sdk.Text                     `nexa:"string=Notes"`
}

// StockWave collects outgoing transfers into one picking campaign (§16.2).
type StockWave struct {
	sdk.Model   `nexa:"model=stock.wave"`
	Name        sdk.String                   `nexa:"required,index,string=Wave Reference"`
	WarehouseID sdk.Many2One[StockWarehouse] `nexa:"index,string=Warehouse"`
	State       sdk.Selection[WaveState]     `nexa:"string=Status,default=draft"`
	LineIDs     sdk.One2Many[StockWaveLine]  `nexa:"string=Transfer Orders"`
	Note        sdk.Text                     `nexa:"string=Notes"`
}

// StockWaveLine binds one transfer order into the wave.
type StockWaveLine struct {
	sdk.Model `nexa:"model=stock.wave.line"`
	WaveID    sdk.Many2One[StockWave]    `nexa:"required,index"`
	PickingID sdk.Many2One[StockPicking] `nexa:"required,index,string=Transfer"`
	Origin    sdk.String                 `nexa:"string=Source Document"`
}

// StockOpsScanLog is the append-only barcode trail (§16.3).
type StockOpsScanLog struct {
	sdk.Model `nexa:"model=stock.ops.scan.log"`
	Name      sdk.String                       `nexa:"required,index,string=Reference"`
	TaskID    sdk.Many2One[StockOperationTask] `nexa:"index,string=Task"`
	ProductID sdk.Many2One[ProductProduct]     `nexa:"index,string=Product"`
	Scanned   sdk.String                       `nexa:"index,string=Scanned Code"`
	Expected  sdk.String                       `nexa:"string=Expected"`
	Matched   sdk.Boolean                      `nexa:"string=Match,default=false"`
	Quantity  sdk.Numeric                      `nexa:"string=Quantity,precision=18,scale=3,default=1"`
	ScannedBy sdk.Many2One[CoreUser]           `nexa:"string=Scanned By"`
	ScannedAt sdk.DateTime                     `nexa:"string=Scanned At"`
	Note      sdk.Text                         `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("stock_ops",
		&StockOperationTask{},
		&StockWave{},
		&StockWaveLine{},
		&StockOpsScanLog{},
	)
}

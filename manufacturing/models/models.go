package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Manufacturing: BOMs, work orders and exploded component demand.
// Phase 1 covers the planning layer — what to make, from what, how much.
// Stock consumption/production postings arrive in phase 2 via stock.picking.

type CoreUser = basemodels.CoreUser

// MrpBom is a bill of materials for one finished product.
type MrpBom struct {
	sdk.Model `nexa:"model=mrp.bom"`

	Name        sdk.String               `nexa:"required,string=BOM Reference"`
	ProductID   sdk.Many2One[sdk.Any]    `nexa:"required,index,string=Finished Product,comodel=product.product"`
	ProductQty  sdk.Numeric              `nexa:"string=Quantity,precision=18,scale=3,default=1"`
	Uom         sdk.String               `nexa:"string=Unit of Measure"`
	Variant     sdk.String               `nexa:"string=Variant/Grade"`
	State       sdk.Selection[BomState]  `nexa:"string=Status,default=draft,selection=draft:Draft,active:Active,archived:Archived"`
	Description sdk.Text                 `nexa:"string=Description"`
	Lines       sdk.One2Many[MrpBomLine] `nexa:"string=Components"`
}

type BomState string

const (
	BomDraft    BomState = "draft"
	BomActive   BomState = "active"
	BomArchived BomState = "archived"
)

// MrpBomLine is one component required per BOM quantity.
type MrpBomLine struct {
	sdk.Model `nexa:"model=mrp.bom.line"`

	BomID      sdk.Many2One[MrpBom]  `nexa:"required,index,ondelete=cascade,string=BOM"`
	ProductID  sdk.Many2One[sdk.Any] `nexa:"required,string=Component,comodel=product.product"`
	ProductQty sdk.Numeric           `nexa:"string=Quantity,precision=18,scale=3,default=1"`
	Sequence   sdk.Integer           `nexa:"string=Sequence,default=10"`
	Note       sdk.String            `nexa:"string=Note"`
}

// MrpWorkOrder is a production order: make ProductQty of ProductID.
type MrpWorkOrder struct {
	sdk.Model `nexa:"model=mrp.workorder"`

	Name           sdk.String                     `nexa:"required,string=Reference"`
	ProductID      sdk.Many2One[sdk.Any]          `nexa:"required,index,string=Product,comodel=product.product"`
	BomID          sdk.Many2One[MrpBom]           `nexa:"index,string=BOM"`
	ProductQty     sdk.Numeric                    `nexa:"string=Quantity to Produce,precision=18,scale=3,default=1"`
	QtyProduced    sdk.Numeric                    `nexa:"string=Quantity Produced,precision=18,scale=3,default=0"`
	State          sdk.Selection[WoState]         `nexa:"string=Status,default=draft,selection=draft:Draft,confirmed:Confirmed,in_progress:In Progress,done:Done,cancelled:Cancelled"`
	DateStart      sdk.Date                       `nexa:"string=Planned Start"`
	DateFinished   sdk.Date                       `nexa:"string=Planned Finish"`
	ResponsibleID  sdk.Many2One[CoreUser]         `nexa:"string=Responsible"`
	Origin         sdk.String                     `nexa:"string=Source Reference"`
	Description    sdk.Text                       `nexa:"string=Description"`
	Lines          sdk.One2Many[MrpWorkOrderLine] `nexa:"string=Component Demand"`
	IssuePicking   sdk.Many2One[sdk.Any]          `nexa:"string=Component Issue,comodel=stock.picking"`
	ReceiptPicking sdk.Many2One[sdk.Any]          `nexa:"string=Finished Goods Receipt,comodel=stock.picking"`
	QtyScrapped    sdk.Numeric                    `nexa:"string=Scrap Qty,precision=18,scale=3,default=0"`
	Backflush      sdk.Boolean                    `nexa:"string=Backflush,default=false"`
	YieldPct       sdk.Numeric                    `nexa:"string=Yield %,precision=18,scale=2,default=0"`
	LotID          sdk.Many2One[sdk.Any]          `nexa:"string=Output Lot,comodel=stock.lot"`
}

type WoState string

const (
	WoDraft      WoState = "draft"
	WoConfirmed  WoState = "confirmed"
	WoInProgress WoState = "in_progress"
	WoDone       WoState = "done"
	WoCancelled  WoState = "cancelled"
)

// MrpWorkOrderLine is exploded component demand for one work order — the MRP
// output. Written by the explode action; consumed quantities are tracked here
// until stock integration lands in phase 2.
type MrpWorkOrderLine struct {
	sdk.Model `nexa:"model=mrp.workorder.line"`

	WorkorderID sdk.Many2One[MrpWorkOrder] `nexa:"required,index,ondelete=cascade,string=Work Order"`
	ProductID   sdk.Many2One[sdk.Any]      `nexa:"required,index,string=Component,comodel=product.product"`
	ProductQty  sdk.Numeric                `nexa:"string=Required Qty,precision=18,scale=3,default=0"`
	QtyConsumed sdk.Numeric                `nexa:"string=Consumed Qty,precision=18,scale=3,default=0"`
	Sequence    sdk.Integer                `nexa:"string=Sequence,default=10"`
	Level       sdk.Integer                `nexa:"string=Level,default=1"`
	LotID       sdk.Many2One[sdk.Any]      `nexa:"string=Component Lot,comodel=stock.lot"`
}

func init() {
	sdk.MustRegister("manufacturing",
		&MrpBom{},
		&MrpBomLine{},
		&MrpWorkOrder{},
		&MrpWorkOrderLine{},
		&MrpWorkCenter{},
		&MrpRouting{},
		&MrpRoutingLine{},
		&MrpPlan{},
		&MrpPlanLine{},
		&MrpScenario{},
		&MrpScenarioLine{},
	)
}

// ---------------------------------------------------------------------------
// Phase 3 — routings & work centers.
// ---------------------------------------------------------------------------

// MrpWorkCenter is a production resource (machine, line, cell) with a cost
// rate and daily capacity used for scheduling and costing.
type MrpWorkCenter struct {
	sdk.Model `nexa:"model=mrp.workcenter"`

	Name        sdk.String  `nexa:"required,string=Work Center"`
	Code        sdk.String  `nexa:"string=Code"`
	CapacityH   sdk.Numeric `nexa:"string=Capacity (h/day),precision=18,scale=2,default=8"`
	CostPerHour sdk.Numeric `nexa:"string=Cost per Hour,precision=18,scale=2,default=0"`
	Sequence    sdk.Integer `nexa:"string=Sequence,default=10"`
	Active      sdk.Boolean `nexa:"string=Active,default=true"`
	Note        sdk.Text    `nexa:"string=Note"`
}

// MrpRouting is an ordered set of operations attached to a BOM.
type MrpRouting struct {
	sdk.Model `nexa:"model=mrp.routing"`

	Name        sdk.String                   `nexa:"required,string=Routing"`
	BomID       sdk.Many2One[MrpBom]         `nexa:"required,index,string=BOM"`
	Description sdk.Text                     `nexa:"string=Description"`
	Lines       sdk.One2Many[MrpRoutingLine] `nexa:"string=Operations"`
}

// MrpRoutingLine is one operation: work center + duration per BOM unit.
type MrpRoutingLine struct {
	sdk.Model `nexa:"model=mrp.routing.line"`

	RoutingID    sdk.Many2One[MrpRouting]    `nexa:"required,index,ondelete=cascade,string=Routing"`
	WorkcenterID sdk.Many2One[MrpWorkCenter] `nexa:"required,string=Work Center"`
	Name         sdk.String                  `nexa:"required,string=Operation"`
	DurationMin  sdk.Numeric                 `nexa:"string=Duration (min/unit),precision=18,scale=2,default=0"`
	Sequence     sdk.Integer                 `nexa:"string=Sequence,default=10"`
}

// ---------------------------------------------------------------------------
// Phase 4 — MRP planning run.
// ---------------------------------------------------------------------------

// MrpPlan is one MRP run: it snapshots demand vs supply and produces
// make/buy recommendations as plan lines.
type MrpPlan struct {
	sdk.Model `nexa:"model=mrp.plan"`

	Name        sdk.String                `nexa:"required,string=Run Reference"`
	RunDate     sdk.Date                  `nexa:"string=Run Date"`
	HorizonDays sdk.Integer               `nexa:"string=Horizon (days),default=30"`
	State       sdk.Selection[PlanState]  `nexa:"string=Status,default=draft,selection=draft:Draft,done:Done"`
	Note        sdk.Text                  `nexa:"string=Note"`
	Lines       sdk.One2Many[MrpPlanLine] `nexa:"string=Recommendations"`
}

type PlanState string

const (
	PlanDraft PlanState = "draft"
	PlanDone  PlanState = "done"
)

// MrpPlanLine is one recommendation: make (work order) or buy (PO).
type MrpPlanLine struct {
	sdk.Model `nexa:"model=mrp.plan.line"`

	PlanID      sdk.Many2One[MrpPlan]     `nexa:"required,index,ondelete=cascade,string=Plan"`
	ProductID   sdk.Many2One[sdk.Any]     `nexa:"required,index,string=Product,comodel=product.product"`
	DemandQty   sdk.Numeric               `nexa:"string=Demand Qty,precision=18,scale=3,default=0"`
	OnHandQty   sdk.Numeric               `nexa:"string=On Hand,precision=18,scale=3,default=0"`
	NetQty      sdk.Numeric               `nexa:"string=Net Requirement,precision=18,scale=3,default=0"`
	Action      sdk.Selection[PlanAction] `nexa:"string=Action,selection=make:Make,buy:Buy,none:None"`
	BomID       sdk.Many2One[MrpBom]      `nexa:"string=BOM"`
	WorkorderID sdk.Many2One[sdk.Any]     `nexa:"string=Work Order,comodel=mrp.workorder"`
	Note        sdk.String                `nexa:"string=Note"`
}

type PlanAction string

const (
	ActMake PlanAction = "make"
	ActBuy  PlanAction = "buy"
	ActNone PlanAction = "none"
)

// ---------------------------------------------------------------------------
// Production scenarios (spec §11.1): simulate demand before committing.
// ---------------------------------------------------------------------------

// MrpScenario is a what-if production plan.
type MrpScenario struct {
	sdk.Model `nexa:"model=mrp.scenario"`

	Name    sdk.String                    `nexa:"required,string=Scenario"`
	Notes   sdk.Text                      `nexa:"string=Notes"`
	State   sdk.Selection[ScState]        `nexa:"string=Status,default=draft,selection=draft:Draft,running:Running,done:Done"`
	RunAt   sdk.DateTime                  `nexa:"string=Last Run"`
	Summary sdk.Text                      `nexa:"string=Result Summary"`
	Lines   sdk.One2Many[MrpScenarioLine] `nexa:"string=Lines"`
}

type ScState string

const (
	ScDraft   ScState = "draft"
	ScRunning ScState = "running"
	ScDone    ScState = "done"
)

// MrpScenarioLine is one hypothetical production demand.
type MrpScenarioLine struct {
	sdk.Model `nexa:"model=mrp.scenario.line"`

	ScenarioID sdk.Many2One[MrpScenario] `nexa:"required,index,ondelete=cascade,string=Scenario"`
	ProductID  sdk.Many2One[sdk.Any]     `nexa:"required,index,string=Product,comodel=product.product"`
	BomID      sdk.Many2One[sdk.Any]     `nexa:"string=BOM,comodel=mrp.bom"`
	Qty        sdk.Numeric               `nexa:"string=Qty,precision=18,scale=3,default=1"`
	Result     sdk.Text                  `nexa:"string=Simulation Result"`
}

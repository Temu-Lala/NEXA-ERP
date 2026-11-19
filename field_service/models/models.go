package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Field service: technicians, work orders, parts consumption, service reports.
// A work order is dispatched to a technician, tracked through states, and
// closed with signature + report.

type CoreUser = basemodels.CoreUser

// FsTechnician is a field worker with skills/certifications.
type FsTechnician struct {
	sdk.Model `nexa:"model=fs.technician"`

	Name           sdk.String                `nexa:"required,string=Technician"`
	Code           sdk.String                `nexa:"string=Code"`
	UserID         sdk.Many2One[CoreUser]    `nexa:"string=Linked User"`
	Skills         sdk.String                `nexa:"string=Skills"`
	Certifications sdk.Text                  `nexa:"string=Certifications"`
	Zone           sdk.String                `nexa:"string=Service Zone"`
	Active         sdk.Boolean               `nexa:"string=Active,default=true"`
	Orders         sdk.One2Many[FsWorkOrder] `nexa:"string=Work Orders"`
	OpenOrders     sdk.Integer               `nexa:"string=Open Orders,default=0"`
}

// FsWorkOrder is a dispatched service job.
type FsWorkOrder struct {
	sdk.Model `nexa:"model=fs.workorder"`

	Name          sdk.String                    `nexa:"required,string=Work Order"`
	TicketID      sdk.Many2One[sdk.Any]         `nexa:"index,string=Ticket,comodel=helpdesk.ticket"`
	CustomerID    sdk.Many2One[sdk.Any]         `nexa:"string=Customer,comodel=core.partner"`
	SiteAddress   sdk.Text                      `nexa:"string=Site Address"`
	Title         sdk.String                    `nexa:"required,string=Title"`
	Description   sdk.Text                      `nexa:"string=Description"`
	RequiredSkill sdk.String                    `nexa:"string=Required Skill"`
	Priority      sdk.Selection[WoPri]          `nexa:"string=Priority,default=normal,selection=low:Low,normal:Normal,high:High,urgent:Urgent"`
	State         sdk.Selection[WoState]        `nexa:"string=Status,default=draft,selection=draft:Draft,dispatched:Dispatched,en_route:En Route,on_site:On Site,in_progress:In Progress,completed:Completed,cancelled:Cancelled"`
	TechnicianID  sdk.Many2One[FsTechnician]    `nexa:"index,string=Technician"`
	ScheduledAt   sdk.DateTime                  `nexa:"string=Scheduled At"`
	ArrivedAt     sdk.DateTime                  `nexa:"string=Arrived At"`
	CompletedAt   sdk.DateTime                  `nexa:"string=Completed At"`
	TravelMin     sdk.Integer                   `nexa:"string=Travel (min),default=0"`
	LaborMin      sdk.Integer                   `nexa:"string=Labor (min),default=0"`
	FirstTimeFix  sdk.Boolean                   `nexa:"string=First-time Fix,default=false"`
	Signature     sdk.String                    `nexa:"string=Customer Signature"`
	Report        sdk.Text                      `nexa:"string=Service Report"`
	Parts         sdk.One2Many[FsWorkOrderPart] `nexa:"string=Parts Used"`
}

type WoPri string

const (
	WoLow    WoPri = "low"
	WoNormal WoPri = "normal"
	WoHigh   WoPri = "high"
	WoUrgent WoPri = "urgent"
)

type WoState string

const (
	WoDraft      WoState = "draft"
	WoDispatched WoState = "dispatched"
	WoEnRoute    WoState = "en_route"
	WoOnSite     WoState = "on_site"
	WoInProgress WoState = "in_progress"
	WoCompleted  WoState = "completed"
	WoCancelled  WoState = "cancelled"
)

// FsWorkOrderPart is a part consumed on a work order (links to stock).
type FsWorkOrderPart struct {
	sdk.Model `nexa:"model=fs.workorder.part"`

	WorkorderID sdk.Many2One[FsWorkOrder] `nexa:"required,index,ondelete=cascade,string=Work Order"`
	ProductID   sdk.Many2One[sdk.Any]     `nexa:"required,index,string=Part,comodel=product.product"`
	Quantity    sdk.Numeric               `nexa:"string=Qty,precision=18,scale=3,default=1"`
	UnitCost    sdk.Numeric               `nexa:"string=Unit Cost,precision=18,scale=2,default=0"`
	Amount      sdk.Numeric               `nexa:"string=Amount,precision=18,scale=2,default=0"`
}

func init() {
	sdk.MustRegister("field_service",
		&FsTechnician{},
		&FsWorkOrder{},
		&FsWorkOrderPart{},
	)
}

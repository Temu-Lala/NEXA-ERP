package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Agriculture: farms, fields, seasonal crop plans and field activities.
// Crop plans carry expected yield; activities log inputs/work with cost —
// plan.total_cost aggregates them for field-level profitability.

type CoreUser = basemodels.CoreUser

// AgricultureFarm is a farm or plantation.
type AgricultureFarm struct {
	sdk.Model `nexa:"model=agriculture.farm"`

	Name        sdk.String                     `nexa:"required,string=Farm"`
	Ref         sdk.String                     `nexa:"string=Reference"`
	Location    sdk.String                     `nexa:"string=Location"`
	Region      sdk.String                     `nexa:"string=Region"`
	AreaHaTotal sdk.Numeric                    `nexa:"string=Cultivated Area (ha),precision=18,scale=2,default=0"`
	AreaHa      sdk.Numeric                    `nexa:"string=Total Area (ha),precision=18,scale=2,default=0"`
	ManagerID   sdk.Many2One[CoreUser]         `nexa:"string=Manager"`
	State       sdk.Selection[FarmState]       `nexa:"string=Status,default=active,selection=active:Active,inactive:Inactive,sold:Sold"`
	Fields      sdk.One2Many[AgricultureField] `nexa:"string=Fields"`
}

type FarmState string

const (
	FarmActive   FarmState = "active"
	FarmInactive FarmState = "inactive"
	FarmSold     FarmState = "sold"
)

// AgricultureField is one cultivable parcel inside a farm.
type AgricultureField struct {
	sdk.Model `nexa:"model=agriculture.field"`

	Name       sdk.String                        `nexa:"required,string=Field"`
	FarmID     sdk.Many2One[AgricultureFarm]     `nexa:"required,index,string=Farm"`
	AreaHa     sdk.Numeric                       `nexa:"string=Area (ha),precision=18,scale=2,default=0"`
	SoilType   sdk.String                        `nexa:"string=Soil Type"`
	Irrigation sdk.String                        `nexa:"string=Irrigation"`
	Notes      sdk.Text                          `nexa:"string=Notes"`
	Plans      sdk.One2Many[AgricultureCropPlan] `nexa:"string=Crop Plans"`
}

// AgricultureCropPlan is one crop cycle on a field for a season.
type AgricultureCropPlan struct {
	sdk.Model `nexa:"model=agriculture.crop.plan"`

	Name          sdk.String                        `nexa:"required,string=Plan Reference"`
	FieldID       sdk.Many2One[AgricultureField]    `nexa:"required,index,string=Field"`
	Crop          sdk.String                        `nexa:"required,string=Crop"`
	Variety       sdk.String                        `nexa:"string=Variety"`
	Season        sdk.String                        `nexa:"string=Season"`
	Year          sdk.Integer                       `nexa:"string=Year"`
	AreaHa        sdk.Numeric                       `nexa:"string=Planted Area (ha),precision=18,scale=2,default=0"`
	ExpectedYield sdk.Numeric                       `nexa:"string=Expected Yield (kg),precision=18,scale=2,default=0"`
	ActualYield   sdk.Numeric                       `nexa:"string=Actual Yield (kg),precision=18,scale=2,default=0"`
	State         sdk.Selection[PlanState]          `nexa:"string=Status,default=planned,selection=planned:Planned,planted:Planted,growing:Growing,harvested:Harvested,failed:Failed"`
	Activities    sdk.One2Many[AgricultureActivity] `nexa:"string=Activities"`
	TotalCost     sdk.Numeric                       `nexa:"string=Total Cost,precision=18,scale=2,default=0"`
}

type PlanState string

const (
	PlanPlanned   PlanState = "planned"
	PlanPlanted   PlanState = "planted"
	PlanGrowing   PlanState = "growing"
	PlanHarvested PlanState = "harvested"
	PlanFailed    PlanState = "failed"
)

// AgricultureActivity is one field operation: planting, irrigation,
// fertilizer, pest control, harvest — with optional product link and cost.
type AgricultureActivity struct {
	sdk.Model `nexa:"model=agriculture.activity"`

	PlanID       sdk.Many2One[AgricultureCropPlan] `nexa:"required,index,ondelete=cascade,string=Crop Plan"`
	ActivityType sdk.Selection[ActType]            `nexa:"required,string=Type,selection=planting:Planting,irrigation:Irrigation,fertilizer:Fertilizer,pesticide:Pesticide,scouting:Scouting,harvest:Harvest,other:Other"`
	Date         sdk.Date                          `nexa:"string=Date"`
	Quantity     sdk.Numeric                       `nexa:"string=Quantity,precision=18,scale=3,default=0"`
	Unit         sdk.String                        `nexa:"string=Unit"`
	ProductID    sdk.Many2One[sdk.Any]             `nexa:"string=Input/Product,comodel=product.product"`
	UnitCost     sdk.Numeric                       `nexa:"string=Unit Cost,precision=18,scale=2,default=0"`
	Amount       sdk.Numeric                       `nexa:"string=Amount,precision=18,scale=2,default=0"`
	Notes        sdk.Text                          `nexa:"string=Notes"`
}

type ActType string

const (
	ActPlanting   ActType = "planting"
	ActIrrigation ActType = "irrigation"
	ActFertilizer ActType = "fertilizer"
	ActPesticide  ActType = "pesticide"
	ActScouting   ActType = "scouting"
	ActHarvest    ActType = "harvest"
	ActOther      ActType = "other"
)

func init() {
	sdk.MustRegister("agriculture",
		&AgricultureFarm{},
		&AgricultureField{},
		&AgricultureCropPlan{},
		&AgricultureActivity{},
		&AgricultureHerd{},
		&AgricultureAnimal{},
		&AgricultureAnimalEvent{},
	)
}

// ---------------------------------------------------------------------------
// Livestock (spec §9.3): herds, individual animals, health & production events.
// ---------------------------------------------------------------------------

// AgricultureHerd is a group of animals on a farm/field.
type AgricultureHerd struct {
	sdk.Model `nexa:"model=agriculture.herd"`

	Name    sdk.String                      `nexa:"required,string=Herd"`
	FarmID  sdk.Many2One[sdk.Any]           `nexa:"required,index,string=Farm,comodel=agriculture.farm"`
	Species sdk.String                      `nexa:"required,string=Species"`
	Breed   sdk.String                      `nexa:"string=Breed"`
	Purpose sdk.String                      `nexa:"string=Purpose"`
	Animals sdk.One2Many[AgricultureAnimal] `nexa:"string=Animals"`
}

// AgricultureAnimal is an individual animal.
type AgricultureAnimal struct {
	sdk.Model `nexa:"model=agriculture.animal"`

	Name      sdk.String                           `nexa:"required,string=Animal ID/Tag"`
	HerdID    sdk.Many2One[AgricultureHerd]        `nexa:"required,index,string=Herd"`
	Tag       sdk.String                           `nexa:"string=Ear Tag/RFID"`
	Sex       sdk.String                           `nexa:"string=Sex"`
	BirthDate sdk.Date                             `nexa:"string=Birth Date"`
	Weight    sdk.Numeric                          `nexa:"string=Weight (kg),precision=18,scale=2,default=0"`
	State     sdk.Selection[AnState]               `nexa:"string=Status,default=active,selection=active:Active,slaughtered:Slaughtered,sold:Sold,dead:Dead"`
	Events    sdk.One2Many[AgricultureAnimalEvent] `nexa:"string=Events"`
}

type AnState string

const (
	AnActive      AnState = "active"
	AnSlaughtered AnState = "slaughtered"
	AnSold        AnState = "sold"
	AnDead        AnState = "dead"
)

// AgricultureAnimalEvent is a health/production/movement record.
type AgricultureAnimalEvent struct {
	sdk.Model `nexa:"model=agriculture.animal.event"`

	AnimalID  sdk.Many2One[AgricultureAnimal] `nexa:"required,index,ondelete=cascade,string=Animal"`
	Kind      sdk.Selection[EvKind]           `nexa:"required,string=Type,selection=vaccination:Vaccination,deworming:Deworming,vet_visit:Vet Visit,birth:Birth,death:Death,sale:Sale,movement:Movement,weight:Weight,milk:Milk,eggs:Eggs,feed:Feed"`
	Date      sdk.Date                        `nexa:"required,string=Date"`
	Qty       sdk.Numeric                     `nexa:"string=Qty,precision=18,scale=3,default=1"`
	Unit      sdk.String                      `nexa:"string=Unit"`
	ProductID sdk.Many2One[sdk.Any]           `nexa:"string=Product,comodel=product.product"`
	VetName   sdk.String                      `nexa:"string=Vet/Provider"`
	Note      sdk.Text                        `nexa:"string=Note"`
}

type EvKind string

const (
	EvVaccination EvKind = "vaccination"
	EvDeworming   EvKind = "deworming"
	EvVetVisit    EvKind = "vet_visit"
	EvBirth       EvKind = "birth"
	EvDeath       EvKind = "death"
	EvSale        EvKind = "sale"
	EvMovement    EvKind = "movement"
	EvWeight      EvKind = "weight"
	EvMilk        EvKind = "milk"
	EvEggs        EvKind = "eggs"
	EvFeed        EvKind = "feed"
)

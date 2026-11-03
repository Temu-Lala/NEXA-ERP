package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	setupmodels "nexa_addons/school_setup/models"
	staffmodels "nexa_addons/school_staff/models"
)

// School Operations (blueprint section 7). This is the campus-operations companion and
// spans five operational areas: TRANSPORT (routes with ordered stops, vehicles with a
// driver and seat capacity, and learner ROUTE ASSIGNMENTS guarded by vehicle capacity),
// LIBRARY (a book catalogue and borrower LOANS with issue/return, availability tracking
// and overdue fines), BOARDING (houses with dormitories and learner BOARDERS assigned to a
// bed and guarded by bed capacity), INVENTORY (items with on-hand stock and a two-sided
// movement ledger that posts receipts and issues without ever driving stock negative) and
// CAFETERIA (menus and learner meal SUBSCRIPTIONS with a lifecycle). Capacity and stock
// guards, overdue/fine arithmetic and movement application are pure and unit-tested; the
// ORM layer projects count/availability computes and orchestrates the lifecycle actions.
// Drivers and matrons reuse school.staff.member and campuses reuse school_setup, so the
// module reuses the existing school data model wherever it can. Navigated under the single
// School app.

type (
	CoreUser           = basemodels.CoreUser
	CoreCompany        = basemodels.CoreCompany
	SchoolStudent      = schoolmodels.SchoolStudent
	SchoolClass        = schoolmodels.SchoolClass
	SchoolSection      = schoolmodels.SchoolSection
	SchoolAcademicYear = schoolmodels.SchoolAcademicYear
	SchoolCampus       = setupmodels.SchoolCampus
	SchoolTerm         = setupmodels.SchoolTerm
	SchoolStaffMember  = staffmodels.SchoolStaffMember
)

type VehicleStatus string

const (
	VehAvailable   VehicleStatus = "available"
	VehInUse       VehicleStatus = "in_use"
	VehMaintenance VehicleStatus = "maintenance"
)

type TransportMode string

const (
	ModePickup TransportMode = "pickup"
	ModeDrop   TransportMode = "drop"
	ModeBoth   TransportMode = "both"
)

type TransportAssignState string

const (
	TAssignPending   TransportAssignState = "pending"
	TAssignActive    TransportAssignState = "active"
	TAssignCancelled TransportAssignState = "cancelled"
)

type LoanState string

const (
	LoanRequested LoanState = "requested"
	LoanBorrowed  LoanState = "borrowed"
	LoanReturned  LoanState = "returned"
	LoanOverdue   LoanState = "overdue"
)

type GenderType string

const (
	GenderMale   GenderType = "male"
	GenderFemale GenderType = "female"
	GenderMixed  GenderType = "mixed"
)

type BoarderState string

const (
	BoardPending BoarderState = "pending"
	BoardActive  BoarderState = "active"
	BoardLeft    BoarderState = "left"
)

type MoveType string

const (
	MoveIn  MoveType = "in"
	MoveOut MoveType = "out"
)

type MoveState string

const (
	MvDraft  MoveState = "draft"
	MvPosted MoveState = "posted"
	MvCancel MoveState = "cancelled"
)

type MealType string

const (
	MealBreakfast MealType = "breakfast"
	MealLunch     MealType = "lunch"
	MealSnack     MealType = "snack"
	MealDinner    MealType = "dinner"
)

type SubState string

const (
	SubPending SubState = "pending"
	SubActive  SubState = "active"
	SubPaused  SubState = "paused"
	SubEnded   SubState = "ended"
)

// ---------------------------------------------------------------------------
// Transport
// ---------------------------------------------------------------------------

// SchoolTransportRoute is a pick-up/drop-off route served by a vehicle.
type SchoolTransportRoute struct {
	sdk.Model `nexa:"model=school.transport.route"`

	Name        sdk.String                           `nexa:"required,size=120,string=Route Name"`
	Code        sdk.String                           `nexa:"index,size=40,string=Route Code"`
	CampusID    sdk.Many2One[SchoolCampus]           `nexa:"index,string=Campus"`
	VehicleID   sdk.Many2One[SchoolTransportVehicle] `nexa:"index,string=Vehicle"`
	Origin      sdk.String                           `nexa:"size=120,string=Origin"`
	Destination sdk.String                           `nexa:"size=120,string=Destination"`
	DistanceKm  sdk.Float64                          `nexa:"string=Distance (km),default=0"`

	Stops sdk.One2Many[SchoolTransportStop] `nexa:"string=Stops,inverse=route_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	StopCount     sdk.Integer `nexa:"string=Stops,compute=compute_route_stop_count"`
	AssignedCount sdk.Integer `nexa:"string=Assigned,compute=compute_route_assigned_count"`
}

// SchoolTransportStop is an ordered stop on a route.
type SchoolTransportStop struct {
	sdk.Model `nexa:"model=school.transport.stop"`

	RouteID    sdk.Many2One[SchoolTransportRoute] `nexa:"required,index,string=Route"`
	Name       sdk.String                         `nexa:"required,size=120,string=Stop Name"`
	Sequence   sdk.Integer                        `nexa:"string=Order,default=0"`
	PickupTime sdk.String                         `nexa:"size=8,string=Pickup Time"`
	DropTime   sdk.String                         `nexa:"size=8,string=Drop Time"`
	Location   sdk.String                         `nexa:"size=200,string=Location"`
	Notes      sdk.Text                           `nexa:"string=Notes"`
}

// SchoolTransportVehicle is a bus/van with seat capacity and a driver.
type SchoolTransportVehicle struct {
	sdk.Model `nexa:"model=school.transport.vehicle"`

	LicensePlate  sdk.String                      `nexa:"required,index,size=20,string=License Plate"`
	Name          sdk.String                      `nexa:"size=120,string=Vehicle"`
	CapacitySeats sdk.Integer                     `nexa:"string=Seats,default=0"`
	DriverID      sdk.Many2One[SchoolStaffMember] `nexa:"index,string=Driver"`
	Status        sdk.Selection[VehicleStatus]    `nexa:"string=Status,default=available,selection=available:Available,in_use:In Use,maintenance:Maintenance,index"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

// SchoolTransportAssignment links a learner to a route and stop.
type SchoolTransportAssignment struct {
	sdk.Model `nexa:"model=school.transport.assignment"`

	StudentID      sdk.Many2One[SchoolStudent]         `nexa:"required,index,string=Student"`
	RouteID        sdk.Many2One[SchoolTransportRoute]  `nexa:"required,index,string=Route"`
	StopID         sdk.Many2One[SchoolTransportStop]   `nexa:"index,string=Stop"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear]    `nexa:"index,string=Academic Year"`
	Mode           sdk.Selection[TransportMode]        `nexa:"string=Mode,default=both,selection=pickup:Pickup,drop:Drop,both:Pickup & Drop"`
	State          sdk.Selection[TransportAssignState] `nexa:"string=Status,default=pending,selection=pending:Pending,active:Active,cancelled:Cancelled,index"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// ---------------------------------------------------------------------------
// Library
// ---------------------------------------------------------------------------

// SchoolLibraryBook is a catalogue title with copy counts.
type SchoolLibraryBook struct {
	sdk.Model `nexa:"model=school.library.book"`

	Title           sdk.String  `nexa:"required,size=200,string=Title"`
	ISBN            sdk.String  `nexa:"index,size=40,string=ISBN"`
	Author          sdk.String  `nexa:"size=120,string=Author"`
	Category        sdk.String  `nexa:"size=80,string=Category"`
	Shelf           sdk.String  `nexa:"size=60,string=Shelf"`
	TotalCopies     sdk.Integer `nexa:"string=Total Copies,default=1"`
	AvailableCopies sdk.Integer `nexa:"string=Available Copies,default=1"`
	Price           sdk.Float64 `nexa:"string=Price,default=0"`

	Loans sdk.One2Many[SchoolLibraryLoan] `nexa:"string=Loans,inverse=book_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	BorrowedCount sdk.Integer `nexa:"string=On Loan,compute=compute_book_borrowed_count"`
}

// SchoolLibraryLoan is a borrower's loan of a book with due/return tracking and a fine.
type SchoolLibraryLoan struct {
	sdk.Model `nexa:"model=school.library.loan"`

	BookID     sdk.Many2One[SchoolLibraryBook] `nexa:"required,index,string=Book"`
	StudentID  sdk.Many2One[SchoolStudent]     `nexa:"required,index,string=Borrower"`
	LoanDate   sdk.Date                        `nexa:"index,string=Loan Date"`
	DueDate    sdk.Date                        `nexa:"index,string=Due Date"`
	ReturnDate sdk.Date                        `nexa:"string=Return Date"`
	State      sdk.Selection[LoanState]        `nexa:"string=Status,default=requested,selection=requested:Requested,borrowed:Borrowed,returned:Returned,overdue:Overdue,index"`
	FineAmount sdk.Float64                     `nexa:"string=Fine,default=0"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// ---------------------------------------------------------------------------
// Boarding
// ---------------------------------------------------------------------------

// SchoolBoardingHouse is a boarding house with a matron and dormitories.
type SchoolBoardingHouse struct {
	sdk.Model `nexa:"model=school.boarding.house"`

	Name        sdk.String                      `nexa:"required,size=120,string=House Name"`
	CampusID    sdk.Many2One[SchoolCampus]      `nexa:"index,string=Campus"`
	MatronID    sdk.Many2One[SchoolStaffMember] `nexa:"index,string=Matron/Warden"`
	Description sdk.Text                        `nexa:"string=Description"`

	Dormitories sdk.One2Many[SchoolBoardingDormitory] `nexa:"string=Dormitories,inverse=house_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	DormCount sdk.Integer `nexa:"string=Dormitories,compute=compute_house_dorm_count"`
}

// SchoolBoardingDormitory is a bed-capacity unit within a house.
type SchoolBoardingDormitory struct {
	sdk.Model `nexa:"model=school.boarding.dormitory"`

	HouseID    sdk.Many2One[SchoolBoardingHouse] `nexa:"required,index,string=House"`
	Name       sdk.String                        `nexa:"required,size=120,string=Dormitory"`
	Capacity   sdk.Integer                       `nexa:"string=Capacity,default=0"`
	GenderType sdk.Selection[GenderType]         `nexa:"string=Gender,default=male,selection=male:Male,female:Female,mixed:Mixed,index"`
	Floor      sdk.String                        `nexa:"size=20,string=Floor"`

	Boarders sdk.One2Many[SchoolBoardingBoarder] `nexa:"string=Boarders,inverse=dormitory_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`

	BoarderCount  sdk.Integer `nexa:"string=Occupied,compute=compute_dorm_boarder_count"`
	AvailableBeds sdk.Integer `nexa:"string=Available Beds,compute=compute_dorm_available_beds"`
}

// SchoolBoardingBoarder is a learner assigned to a dormitory bed.
type SchoolBoardingBoarder struct {
	sdk.Model `nexa:"model=school.boarding.boarder"`

	StudentID      sdk.Many2One[SchoolStudent]           `nexa:"required,index,string=Student"`
	HouseID        sdk.Many2One[SchoolBoardingHouse]     `nexa:"required,index,string=House"`
	DormitoryID    sdk.Many2One[SchoolBoardingDormitory] `nexa:"required,index,string=Dormitory"`
	BedNumber      sdk.String                            `nexa:"size=20,string=Bed"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear]      `nexa:"index,string=Academic Year"`
	DateFrom       sdk.Date                              `nexa:"string=From"`
	DateTo         sdk.Date                              `nexa:"string=To"`
	State          sdk.Selection[BoarderState]           `nexa:"string=Status,default=pending,selection=pending:Pending,active:Active,left:Left,index"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// ---------------------------------------------------------------------------
// Inventory
// ---------------------------------------------------------------------------

// SchoolInventoryItem is a stock-keeping item with on-hand quantity and a reorder level.
type SchoolInventoryItem struct {
	sdk.Model `nexa:"model=school.inventory.item"`

	Name          sdk.String  `nexa:"required,size=120,string=Item Name"`
	Code          sdk.String  `nexa:"index,size=40,string=Item Code"`
	Category      sdk.String  `nexa:"size=80,string=Category"`
	UnitOfMeasure sdk.String  `nexa:"size=20,string=Unit,default=unit"`
	Location      sdk.String  `nexa:"size=120,string=Store/Location"`
	OnHandQty     sdk.Float64 `nexa:"string=On Hand,default=0"`
	ReorderLevel  sdk.Float64 `nexa:"string=Reorder Level,default=0"`
	UnitCost      sdk.Float64 `nexa:"string=Unit Cost,default=0"`

	Movements sdk.One2Many[SchoolInventoryMovement] `nexa:"string=Movements,inverse=item_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	IsLow sdk.Boolean `nexa:"string=Low Stock,compute=compute_item_is_low"`
}

// SchoolInventoryMovement is a receipt or issue against an item, posted to adjust stock.
type SchoolInventoryMovement struct {
	sdk.Model `nexa:"model=school.inventory.movement"`

	ItemID    sdk.Many2One[SchoolInventoryItem] `nexa:"required,index,string=Item"`
	MoveDate  sdk.Date                          `nexa:"index,string=Date"`
	MoveType  sdk.Selection[MoveType]           `nexa:"string=Direction,default=in,selection=in:Receipt,out:Issue,index"`
	Quantity  sdk.Float64                       `nexa:"string=Quantity,default=0"`
	Reference sdk.String                        `nexa:"size=120,string=Reference"`
	State     sdk.Selection[MoveState]          `nexa:"string=Status,default=draft,selection=draft:Draft,posted:Posted,cancelled:Cancelled,index"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// ---------------------------------------------------------------------------
// Cafeteria
// ---------------------------------------------------------------------------

// SchoolCafeteriaMenu is a meal offering for a campus/term.
type SchoolCafeteriaMenu struct {
	sdk.Model `nexa:"model=school.cafeteria.menu"`

	Name     sdk.String                 `nexa:"required,size=120,string=Menu Name"`
	CampusID sdk.Many2One[SchoolCampus] `nexa:"index,string=Campus"`
	TermID   sdk.Many2One[SchoolTerm]   `nexa:"index,string=Term"`
	MealType sdk.Selection[MealType]    `nexa:"string=Meal,default=lunch,selection=breakfast:Breakfast,lunch:Lunch,snack:Snack,dinner:Dinner,index"`
	Dishes   sdk.Text                   `nexa:"string=Dishes"`
	Price    sdk.Float64                `nexa:"string=Price,default=0"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

// SchoolCafeteriaSubscription is a learner's meal plan subscription.
type SchoolCafeteriaSubscription struct {
	sdk.Model `nexa:"model=school.cafeteria.subscription"`

	StudentID sdk.Many2One[SchoolStudent]       `nexa:"required,index,string=Student"`
	MenuID    sdk.Many2One[SchoolCafeteriaMenu] `nexa:"index,string=Menu"`
	MealPlan  sdk.String                        `nexa:"size=120,string=Meal Plan"`
	DailyRate sdk.Float64                       `nexa:"string=Daily Rate,default=0"`
	DateFrom  sdk.Date                          `nexa:"string=From"`
	DateTo    sdk.Date                          `nexa:"string=To"`
	State     sdk.Selection[SubState]           `nexa:"string=Status,default=pending,selection=pending:Pending,active:Active,paused:Paused,ended:Ended,index"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("school_ops",
		&SchoolTransportRoute{},
		&SchoolTransportStop{},
		&SchoolTransportVehicle{},
		&SchoolTransportAssignment{},
		&SchoolLibraryBook{},
		&SchoolLibraryLoan{},
		&SchoolBoardingHouse{},
		&SchoolBoardingDormitory{},
		&SchoolBoardingBoarder{},
		&SchoolInventoryItem{},
		&SchoolInventoryMovement{},
		&SchoolCafeteriaMenu{},
		&SchoolCafeteriaSubscription{},
	)
}

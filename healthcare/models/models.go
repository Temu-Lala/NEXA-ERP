package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Healthcare administration: providers, patients, appointments, encounters.
// Deliberately administrative — clinical decision support is out of scope.
// Access separation: front-desk group sees scheduling only; clinical group
// sees encounter notes. Patient PII fields stay on restricted models.

type CoreUser = basemodels.CoreUser

// HealthcareProvider is a practitioner or clinician.
type HealthcareProvider struct {
	sdk.Model `nexa:"model=healthcare.provider"`

	Name         sdk.String                          `nexa:"required,string=Provider"`
	Code         sdk.String                          `nexa:"string=Provider Code"`
	Specialty    sdk.String                          `nexa:"string=Specialty"`
	Department   sdk.String                          `nexa:"string=Department"`
	UserID       sdk.Many2One[CoreUser]              `nexa:"string=Linked User"`
	LicenseNo    sdk.String                          `nexa:"string=License Number"`
	Active       sdk.Boolean                         `nexa:"string=Active,default=true"`
	Appointments sdk.One2Many[HealthcareAppointment] `nexa:"string=Appointments"`
}

// HealthcarePatient is a registered patient.
type HealthcarePatient struct {
	sdk.Model `nexa:"model=healthcare.patient"`

	PatientNo      sdk.String                          `nexa:"required,index,string=Patient No"`
	Name           sdk.String                          `nexa:"required,string=Full Name"`
	DateOfBirth    sdk.Date                            `nexa:"string=Date of Birth"`
	Gender         sdk.Selection[Gender]               `nexa:"string=Gender,selection=male:Male,female:Female,other:Other,undisclosed:Undisclosed"`
	Phone          sdk.String                          `nexa:"string=Phone"`
	Email          sdk.String                          `nexa:"string=Email"`
	Address        sdk.Text                            `nexa:"string=Address"`
	BloodType      sdk.String                          `nexa:"string=Blood Type"`
	Allergies      sdk.Text                            `nexa:"string=Allergies"`
	EmergencyName  sdk.String                          `nexa:"string=Emergency Contact"`
	EmergencyPhone sdk.String                          `nexa:"string=Emergency Phone"`
	ConsentRecords sdk.Boolean                         `nexa:"string=Consent on File,default=false"`
	State          sdk.Selection[PatientState]         `nexa:"string=Status,default=active,selection=active:Active,inactive:Inactive,deceased:Deceased"`
	AgeYears       sdk.Integer                         `nexa:"string=Age,default=0"`
	Appointments   sdk.One2Many[HealthcareAppointment] `nexa:"string=Appointments"`
	Encounters     sdk.One2Many[HealthcareEncounter]   `nexa:"string=Encounters"`
}

type Gender string

const (
	GenderMale        Gender = "male"
	GenderFemale      Gender = "female"
	GenderOther       Gender = "other"
	GenderUndisclosed Gender = "undisclosed"
)

type PatientState string

const (
	PatientActive   PatientState = "active"
	PatientInactive PatientState = "inactive"
	PatientDeceased PatientState = "deceased"
)

// HealthcareAppointment is a scheduled visit.
type HealthcareAppointment struct {
	sdk.Model `nexa:"model=healthcare.appointment"`

	Name        sdk.String                       `nexa:"required,string=Appointment"`
	PatientID   sdk.Many2One[HealthcarePatient]  `nexa:"required,index,string=Patient"`
	ProviderID  sdk.Many2One[HealthcareProvider] `nexa:"required,index,string=Provider"`
	ScheduledAt sdk.DateTime                     `nexa:"string=Scheduled At"`
	DurationMin sdk.Integer                      `nexa:"string=Duration (min),default=30"`
	Type        sdk.Selection[ApptType]          `nexa:"string=Type,default=consultation,selection=consultation:Consultation,follow_up:Follow-up,procedure:Procedure,lab:Lab,imaging:Imaging,other:Other"`
	Reason      sdk.String                       `nexa:"string=Reason"`
	State       sdk.Selection[ApptState]         `nexa:"string=Status,default=scheduled,selection=scheduled:Scheduled,confirmed:Confirmed,checked_in:Checked In,in_progress:In Progress,completed:Completed,cancelled:Cancelled,no_show:No Show"`
	Note        sdk.Text                         `nexa:"string=Internal Note"`
	Encounter   sdk.Many2One[sdk.Any]            `nexa:"string=Encounter,comodel=healthcare.encounter"`
}

type ApptType string

const (
	ApptConsultation ApptType = "consultation"
	ApptFollowUp     ApptType = "follow_up"
	ApptProcedure    ApptType = "procedure"
	ApptLab          ApptType = "lab"
	ApptImaging      ApptType = "imaging"
	ApptOther        ApptType = "other"
)

type ApptState string

const (
	ApptScheduled  ApptState = "scheduled"
	ApptConfirmed  ApptState = "confirmed"
	ApptCheckedIn  ApptState = "checked_in"
	ApptInProgress ApptState = "in_progress"
	ApptCompleted  ApptState = "completed"
	ApptCancelled  ApptState = "cancelled"
	ApptNoShow     ApptState = "no_show"
)

// HealthcareEncounter is a clinical visit record — restricted group only.
type HealthcareEncounter struct {
	sdk.Model `nexa:"model=healthcare.encounter"`

	Name           sdk.String                       `nexa:"required,string=Encounter"`
	PatientID      sdk.Many2One[HealthcarePatient]  `nexa:"required,index,string=Patient"`
	ProviderID     sdk.Many2One[HealthcareProvider] `nexa:"required,index,string=Provider"`
	AppointmentID  sdk.Many2One[sdk.Any]            `nexa:"string=Appointment,comodel=healthcare.appointment"`
	EncounterDate  sdk.DateTime                     `nexa:"string=Date"`
	Type           sdk.Selection[EncType]           `nexa:"string=Type,default=outpatient,selection=outpatient:Outpatient,inpatient:Inpatient,emergency:Emergency,telehealth:Telehealth"`
	ChiefComplaint sdk.String                       `nexa:"string=Chief Complaint"`
	Notes          sdk.Text                         `nexa:"string=Clinical Notes"`
	State          sdk.Selection[EncState]          `nexa:"string=Status,default=open,selection=open:Open,closed:Closed"`
}

type EncType string

const (
	EncOutpatient EncType = "outpatient"
	EncInpatient  EncType = "inpatient"
	EncEmergency  EncType = "emergency"
	EncTelehealth EncType = "telehealth"
)

type EncState string

const (
	EncOpen   EncState = "open"
	EncClosed EncState = "closed"
)

func init() {
	sdk.MustRegister("healthcare",
		&HealthcareProvider{},
		&HealthcarePatient{},
		&HealthcareAppointment{},
		&HealthcareEncounter{}, &HealthcareTariff{},
		&HealthcareClaim{},
		&HealthcareClaimLine{},
	)

}

// ---------------------------------------------------------------------------
// Healthcare finance (spec §6.4): tariffs, billing and insurance claims.
// ---------------------------------------------------------------------------

// HealthcareTariff is a service price list (consultation, procedure, lab...).
type HealthcareTariff struct {
	sdk.Model `nexa:"model=healthcare.tariff"`

	Name       sdk.String            `nexa:"required,string=Tariff"`
	Code       sdk.String            `nexa:"string=Code"`
	Service    sdk.String            `nexa:"required,string=Service"`
	Category   sdk.String            `nexa:"string=Category"`
	Price      sdk.Numeric           `nexa:"string=Price,precision=18,scale=2,default=0"`
	CurrencyID sdk.Many2One[sdk.Any] `nexa:"string=Currency,comodel=currency.currency"`
	PayerShare sdk.Numeric           `nexa:"string=Insurance Share %,precision=18,scale=2,default=0"`
	Active     sdk.Boolean           `nexa:"string=Active,default=true"`
}

// HealthcareClaim is an insurance claim against a payer.
type HealthcareClaim struct {
	sdk.Model `nexa:"model=healthcare.claim"`

	Name           sdk.String                        `nexa:"required,string=Claim Ref"`
	PatientID      sdk.Many2One[HealthcarePatient]   `nexa:"required,index,string=Patient"`
	EncounterID    sdk.Many2One[HealthcareEncounter] `nexa:"index,string=Encounter"`
	PayerID        sdk.Many2One[sdk.Any]             `nexa:"string=Payer,comodel=core.partner"`
	PolicyNo       sdk.String                        `nexa:"string=Policy Number"`
	AmountTotal    sdk.Numeric                       `nexa:"string=Billed,precision=18,scale=2,default=0"`
	AmountApproved sdk.Numeric                       `nexa:"string=Approved,precision=18,scale=2,default=0"`
	AmountPaid     sdk.Numeric                       `nexa:"string=Paid,precision=18,scale=2,default=0"`
	Copay          sdk.Numeric                       `nexa:"string=Patient Copay,precision=18,scale=2,default=0"`
	State          sdk.Selection[ClaimState]         `nexa:"string=Status,default=draft,selection=draft:Draft,submitted:Submitted,adjudicated:Adjudicated,paid:Paid,rejected:Rejected"`
	SubmittedAt    sdk.DateTime                      `nexa:"string=Submitted At"`
	DecidedAt      sdk.DateTime                      `nexa:"string=Decided At"`
	DenialReason   sdk.String                        `nexa:"string=Denial Reason"`
	Lines          sdk.One2Many[HealthcareClaimLine] `nexa:"string=Lines"`
}

type ClaimState string

const (
	ClaimDraft       ClaimState = "draft"
	ClaimSubmitted   ClaimState = "submitted"
	ClaimAdjudicated ClaimState = "adjudicated"
	ClaimPaid        ClaimState = "paid"
	ClaimRejected    ClaimState = "rejected"
)

// HealthcareClaimLine is one billed service on a claim.
type HealthcareClaimLine struct {
	sdk.Model `nexa:"model=healthcare.claim.line"`

	ClaimID   sdk.Many2One[HealthcareClaim]  `nexa:"required,index,ondelete=cascade,string=Claim"`
	TariffID  sdk.Many2One[HealthcareTariff] `nexa:"string=Tariff"`
	Service   sdk.String                     `nexa:"string=Service"`
	Quantity  sdk.Numeric                    `nexa:"string=Qty,precision=18,scale=2,default=1"`
	UnitPrice sdk.Numeric                    `nexa:"string=Unit Price,precision=18,scale=2,default=0"`
	Amount    sdk.Numeric                    `nexa:"string=Amount,precision=18,scale=2,default=0"`
	Approved  sdk.Numeric                    `nexa:"string=Approved,precision=18,scale=2,default=0"`
	Diagnosis sdk.String                     `nexa:"string=Diagnosis Code"`
	Procedure sdk.String                     `nexa:"string=Procedure Code"`
}

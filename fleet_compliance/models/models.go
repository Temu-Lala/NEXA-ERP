package models

import (
	basemodels "nexa/addons/base/models"
	fleetdrivermodels "nexa_addons/fleet_driver/models"
	fleetmodels "nexa_addons/fleet/models"
	fleettripmodels "nexa_addons/fleet_trip/models"
	"nexa/core/sdk"
)

// Cross-module aliases
type CoreUser = basemodels.CoreUser
type FleetVehicle = fleetmodels.FleetVehicle
type FleetDriver = fleetdrivermodels.FleetDriver
type FleetTrip = fleettripmodels.FleetTrip

// Category Selection: registration, insurance_cert, roadworthiness, operating_permit, route_permit, commercial_transport, tax_token, customs_import, warranty, lease_contract
type DocumentCategory string

const (
	DocCatRegistration       DocumentCategory = "registration"
	DocCatInsuranceCert       DocumentCategory = "insurance_cert"
	DocCatRoadworthiness      DocumentCategory = "roadworthiness"
	DocCatOperatingPermit     DocumentCategory = "operating_permit"
	DocCatRoutePermit         DocumentCategory = "route_permit"
	DocCatCommercialTransport DocumentCategory = "commercial_transport"
	DocCatTaxToken            DocumentCategory = "tax_token"
	DocCatCustomsImport       DocumentCategory = "customs_import"
	DocCatWarranty            DocumentCategory = "warranty"
	DocCatLeaseContract       DocumentCategory = "lease_contract"
)

// Status Selection: valid, expiring_soon, expired, rejected
type DocumentStatus string

const (
	DocStatusValid        DocumentStatus = "valid"
	DocStatusExpiringSoon DocumentStatus = "expiring_soon"
	DocStatusExpired      DocumentStatus = "expired"
	DocStatusRejected     DocumentStatus = "rejected"
)

// ActionOnExpiry Selection: advisory, warning, block_dispatch
type ActionOnExpiry string

const (
	ActionOnExpiryAdvisory      ActionOnExpiry = "advisory"
	ActionOnExpiryWarning       ActionOnExpiry = "warning"
	ActionOnExpiryBlockDispatch ActionOnExpiry = "block_dispatch"
)

// CoverageType Selection: comprehensive, third_party, collision, fleet_umbrella
type PolicyCoverageType string

const (
	CoverageComprehensive PolicyCoverageType = "comprehensive"
	CoverageThirdParty    PolicyCoverageType = "third_party"
	CoverageCollision     PolicyCoverageType = "collision"
	CoverageFleetUmbrella PolicyCoverageType = "fleet_umbrella"
)

// PolicyStatus Selection: active, expired, renewed, cancelled
type PolicyStatus string

const (
	PolicyStatusActive    PolicyStatus = "active"
	PolicyStatusExpired   PolicyStatus = "expired"
	PolicyStatusRenewed   PolicyStatus = "renewed"
	PolicyStatusCancelled PolicyStatus = "cancelled"
)

// IncidentType Selection: collision, rollover, pedestrian, theft, vandalism, fire, near_miss, weather
type IncidentType string

const (
	IncidentTypeCollision  IncidentType = "collision"
	IncidentTypeRollover   IncidentType = "rollover"
	IncidentTypePedestrian IncidentType = "pedestrian"
	IncidentTypeTheft      IncidentType = "theft"
	IncidentTypeVandalism  IncidentType = "vandalism"
	IncidentTypeFire       IncidentType = "fire"
	IncidentTypeNearMiss   IncidentType = "near_miss"
	IncidentTypeWeather    IncidentType = "weather"
)

// IncidentSeverity Selection: minor_damage, moderate_damage, severe_damage, total_loss
type IncidentSeverity string

const (
	SeverityMinorDamage    IncidentSeverity = "minor_damage"
	SeverityModerateDamage IncidentSeverity = "moderate_damage"
	SeveritySevereDamage   IncidentSeverity = "severe_damage"
	SeverityTotalLoss      IncidentSeverity = "total_loss"
)

// ClaimStatus Selection: not_applicable, reported, filed, under_assessment, settled, rejected
type ClaimStatus string

const (
	ClaimNotApplicable   ClaimStatus = "not_applicable"
	ClaimReported        ClaimStatus = "reported"
	ClaimFiled           ClaimStatus = "filed"
	ClaimUnderAssessment ClaimStatus = "under_assessment"
	ClaimSettled         ClaimStatus = "settled"
	ClaimRejected        ClaimStatus = "rejected"
)

// IncidentState Selection: reported, investigating, corrective_action, closed
type IncidentState string

const (
	IncidentStateReported         IncidentState = "reported"
	IncidentStateInvestigating    IncidentState = "investigating"
	IncidentStateCorrectiveAction IncidentState = "corrective_action"
	IncidentStateClosed           IncidentState = "closed"
)

// FleetVehicleDocument (fleet.vehicle.document)
type FleetVehicleDocument struct {
	sdk.Model `nexa:"model=fleet.vehicle.document"`

	VehicleID        sdk.Many2One[FleetVehicle]      `nexa:"required,index,string=Vehicle"`
	Category         sdk.Selection[DocumentCategory] `nexa:"string=Document Category,default=registration"`
	DocumentNumber   sdk.String                      `nexa:"required,index,string=Document Number"`
	IssuingAuthority sdk.String                      `nexa:"string=Issuing Authority"`
	IssueDate        sdk.Date                        `nexa:"required,string=Issue Date"`
	ExpiryDate       sdk.Date                        `nexa:"required,index,string=Expiry Date"`
	Status           sdk.Selection[DocumentStatus]   `nexa:"string=Status,default=valid"`
	ActionOnExpiry   sdk.Selection[ActionOnExpiry]   `nexa:"string=Action On Expiry,default=advisory"`
	VerifiedBy       sdk.Many2One[CoreUser]          `nexa:"string=Verified By"`
	VerifiedDate     sdk.Date                        `nexa:"string=Verified Date"`
	Notes            sdk.Text                        `nexa:"string=Notes"`
}

// FleetInsurancePolicy (fleet.insurance.policy)
type FleetInsurancePolicy struct {
	sdk.Model `nexa:"model=fleet.insurance.policy"`

	PolicyNumber     sdk.String                        `nexa:"required,index,string=Policy Number"`
	Insurer          sdk.String                        `nexa:"required,string=Insurer"`
	Broker           sdk.String                        `nexa:"string=Broker"`
	CoverageType     sdk.Selection[PolicyCoverageType] `nexa:"string=Coverage Type,default=comprehensive"`
	PolicyStatus     sdk.Selection[PolicyStatus]       `nexa:"string=Status,default=active"`
	EffectiveDate    sdk.Date                          `nexa:"required,string=Effective Date"`
	ExpiryDate       sdk.Date                          `nexa:"required,index,string=Expiry Date"`
	PremiumAmount    sdk.Numeric                       `nexa:"string=Premium Amount,precision=18,scale=2,default=0"`
	DeductibleAmount sdk.Numeric                       `nexa:"string=Deductible Amount,precision=18,scale=2,default=0"`
	Currency         sdk.String                        `nexa:"string=Currency,default=USD"`
	PolicyLimit      sdk.Numeric                       `nexa:"string=Policy Limit,precision=18,scale=2,default=0"`
	Notes            sdk.Text                          `nexa:"string=Notes"`
}

// FleetIncident (fleet.incident)
type FleetIncident struct {
	sdk.Model `nexa:"model=fleet.incident"`

	Name                 sdk.String                      `nexa:"required,index,string=Incident #"`
	VehicleID            sdk.Many2One[FleetVehicle]      `nexa:"required,index,string=Vehicle"`
	DriverID             sdk.Many2One[FleetDriver]       `nexa:"index,string=Driver"`
	TripID               sdk.Many2One[FleetTrip]         `nexa:"index,string=Trip"`
	IncidentDate         sdk.DateTime                    `nexa:"required,string=Incident Date & Time"`
	Location             sdk.String                      `nexa:"required,string=Location"`
	IncidentType         sdk.Selection[IncidentType]     `nexa:"string=Incident Type,default=collision"`
	IncidentSeverity     sdk.Selection[IncidentSeverity] `nexa:"string=Severity,default=minor_damage"`
	InjuryInvolved       sdk.Boolean                     `nexa:"string=Injury Involved,default=false"`
	PoliceReportNumber   sdk.String                      `nexa:"string=Police Report #"`
	ThirdPartyDetails    sdk.Text                        `nexa:"string=Third Party Details"`
	EstimatedCost        sdk.Numeric                     `nexa:"string=Estimated Cost,precision=18,scale=2,default=0"`
	ActualCost           sdk.Numeric                     `nexa:"string=Actual Cost,precision=18,scale=2,default=0"`
	ClaimStatus          sdk.Selection[ClaimStatus]      `nexa:"string=Claim Status,default=not_applicable"`
	InsuranceClaimNumber sdk.String                      `nexa:"string=Insurance Claim #"`
	RootCause            sdk.Text                        `nexa:"string=Root Cause"`
	CorrectiveActions    sdk.Text                        `nexa:"string=Corrective Actions"`
	IncidentState        sdk.Selection[IncidentState]    `nexa:"string=Incident Status,default=reported"`
	Notes                sdk.Text                        `nexa:"string=Notes"`
}

func init() {
	sdk.MustRegister("fleet_compliance",
		&FleetVehicleDocument{},
		&FleetInsurancePolicy{},
		&FleetIncident{},
	)
}

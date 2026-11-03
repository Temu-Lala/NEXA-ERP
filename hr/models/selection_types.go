package models

// ---------------------------------------------------------------------------
// Employee profile enumerations (aligned to the HR & HCM SRS §2.1/§2.2).
// Values are stable identifiers persisted in the DB; the display label is set
// via the selection= tag on the field.
// ---------------------------------------------------------------------------

// EmployeeGender mirrors the personal-info gender choice.
type EmployeeGender string

const (
	GenderMale    EmployeeGender = "male"
	GenderFemale  EmployeeGender = "female"
	GenderOther   EmployeeGender = "other"
	GenderUnknown EmployeeGender = "undisclosed"
)

// EmployeeMarital mirrors the personal-info marital status choice.
type EmployeeMarital string

const (
	MaritalSingle     EmployeeMarital = "single"
	MaritalMarried    EmployeeMarital = "married"
	MaritalCohabiting EmployeeMarital = "cohabiting"
	MaritalSeparated  EmployeeMarital = "separated"
	MaritalDivorced   EmployeeMarital = "divorced"
	MaritalWidowed    EmployeeMarital = "widowed"
)

// EmployeeType mirrors the employment category (§2.1).
type EmployeeType string

const (
	EmploymentFullTime   EmployeeType = "full_time"
	EmploymentPartTime   EmployeeType = "part_time"
	EmploymentFixedTerm  EmployeeType = "fixed_term"
	EmploymentTemporary  EmployeeType = "temporary"
	EmploymentIntern     EmployeeType = "intern"
	EmploymentContractor EmployeeType = "contractor"
	EmploymentFreelance  EmployeeType = "freelance"
	EmploymentOther      EmployeeType = "other"
)

// EmployeeState is the auditable lifecycle (§2.2):
// Draft → Pending → Active → On Leave / Suspended → Notice → Terminated / Retired.
type EmployeeState string

const (
	StateDraft      EmployeeState = "draft"
	StatePending    EmployeeState = "pending"
	StateActive     EmployeeState = "active"
	StateOnLeave    EmployeeState = "on_leave"
	StateSuspended  EmployeeState = "suspended"
	StateNotice     EmployeeState = "notice"
	StateTerminated EmployeeState = "terminated"
	StateRetired    EmployeeState = "retired"
)

// EmployeeTermination records why an employment ended (§2.2 / offboarding).
type EmployeeTermination string

const (
	TerminationUnspecified EmployeeTermination = "unspecified"
	TerminationResignation EmployeeTermination = "resignation"
	TerminationFired       EmployeeTermination = "fired"
	TerminationRetired     EmployeeTermination = "retired"
	TerminationContractEnd EmployeeTermination = "contract_end"
	TerminationOther       EmployeeTermination = "other"
)

// DocumentCategory classifies an employee document (§2.3).
type DocumentCategory string

const (
	DocCatIdentity      DocumentCategory = "identity"
	DocCatContract      DocumentCategory = "contract"
	DocCatCertification DocumentCategory = "certification"
	DocCatTax           DocumentCategory = "tax"
	DocCatDiploma       DocumentCategory = "diploma"
	DocCatMedical       DocumentCategory = "medical"
	DocCatOther         DocumentCategory = "other"
)

// DocumentStatus is derived live from the expiry date (see compute_document_status).
type DocumentStatus string

const (
	DocumentValid    DocumentStatus = "valid"
	DocumentExpiring DocumentStatus = "expiring"
	DocumentExpired  DocumentStatus = "expired"
)

// LeaveState is the approval workflow shared by leave requests and allocations.
type LeaveState string

const (
	LeaveDraft    LeaveState = "draft"
	LeaveConfirm  LeaveState = "confirm"
	LeaveValidate LeaveState = "validate"
	LeaveRefuse   LeaveState = "refuse"
)

// ApplicantStage is the recruitment pipeline stage (§ recruitment board).
type ApplicantStage string

const (
	ApplicantNew       ApplicantStage = "new"
	ApplicantInterview ApplicantStage = "interview"
	ApplicantOffer     ApplicantStage = "offer"
	ApplicantHired     ApplicantStage = "hired"
	ApplicantRejected  ApplicantStage = "rejected"
)

// ContractType classifies an employment agreement.
type ContractType string

const (
	ContractPermanent  ContractType = "permanent"
	ContractFixedTerm  ContractType = "fixed_term"
	ContractInternship ContractType = "internship"
	ContractContractor ContractType = "contractor"
	ContractTemporary  ContractType = "temporary"
)

// ContractState is derived live from the contract end date.
type ContractState string

const (
	ContractOpen     ContractState = "open"
	ContractRunning  ContractState = "running"
	ContractExpiring ContractState = "expiring"
	ContractExpired  ContractState = "expired"
)

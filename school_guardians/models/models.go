package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
)

// School Guardians (blueprint section 7 / "Advanced parent access"). A guardian is a
// first-class person record, and the link table (school.student.guardian) is what
// ties guardians to learners. The link is deliberately time-bounded and status-
// carrying so it doubles as the relationship/access history: a relationship change
// or an access revocation is expressed by closing the old row (valid_to / revoked)
// and opening a new one — never by silently overwriting who a child's guardian is.
// Financial visibility is an explicit, per-guardian, per-student opt-in flag so the
// portal can honour "configure which guardian can view financial information".

type (
	CorePartner   = basemodels.CorePartner
	CoreCompany   = basemodels.CoreCompany
	SchoolStudent = schoolmodels.SchoolStudent
)

// GuardianRelationship is the role a guardian plays for a specific learner.
type GuardianRelationship string

const (
	RelFather        GuardianRelationship = "father"
	RelMother        GuardianRelationship = "mother"
	RelGuardian      GuardianRelationship = "guardian"
	RelUncle         GuardianRelationship = "uncle"
	RelAunt          GuardianRelationship = "aunt"
	RelGrandparent   GuardianRelationship = "grandparent"
	RelSibling       GuardianRelationship = "sibling"
	RelLegalGuardian GuardianRelationship = "legal_guardian"
	RelOther         GuardianRelationship = "other"
)

// LinkStatus marks whether a guardian-to-student authorisation is live or has been
// revoked. Revoked rows are retained for the audit trail, never deleted.
type LinkStatus string

const (
	LinkActive  LinkStatus = "active"
	LinkRevoked LinkStatus = "revoked"
)

// SchoolGuardian is the authoritative guardian / parent person record. One guardian
// can be linked to many learners (and one learner can have many guardians) through
// the school.student.guardian link table.
type SchoolGuardian struct {
	sdk.Model `nexa:"model=school.guardian"`

	Code      sdk.String                `nexa:"required,index,size=40,string=Guardian Code"`
	Name      sdk.String                `nexa:"required,size=120,string=Full Name"`
	PartnerID sdk.Many2One[CorePartner] `nexa:"string=Contact / Portal Login"`

	Phone      sdk.String `nexa:"size=40,string=Phone"`
	Email      sdk.String `nexa:"size=120,string=Email"`
	Address    sdk.String `nexa:"size=240,string=Address"`
	Occupation sdk.String `nexa:"size=120,string=Occupation"`
	Employer   sdk.String `nexa:"size=120,string=Employer"`
	NationalID sdk.String `nexa:"size=60,string=National ID"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	// Inline one-to-many of this guardian's learner links (child FK guardian_id).
	LearnerLinks sdk.One2Many[SchoolStudentGuardian] `nexa:"string=Learner Links,inverse=guardian_id"`

	// Non-stored signal for the list/form: how many live learner links exist.
	ChildrenCount sdk.Integer `nexa:"string=Learners,compute=compute_guardian_children"`
}

// SchoolStudentGuardian is one time-bounded guardian-to-student relationship. Each row
// captures the relationship role plus the access authorisation matrix; the date window
// and status make the whole table an append-friendly relationship history.
type SchoolStudentGuardian struct {
	sdk.Model `nexa:"model=school.student.guardian"`

	StudentID    sdk.Many2One[SchoolStudent]         `nexa:"required,index,string=Student"`
	GuardianID   sdk.Many2One[SchoolGuardian]        `nexa:"required,index,string=Guardian"`
	Relationship sdk.Selection[GuardianRelationship] `nexa:"string=Relationship,default=guardian,selection=father:Father,mother:Mother,guardian:Guardian,uncle:Uncle,aunt:Aunt,grandparent:Grandparent,sibling:Sibling,legal_guardian:Legal Guardian,other:Other"`

	IsPrimary            sdk.Boolean `nexa:"string=Primary Contact,default=false"`
	CanViewAcademic      sdk.Boolean `nexa:"string=Can View Academic,default=true"`
	CanViewFinancial     sdk.Boolean `nexa:"string=Can View Financial,default=false"`
	CanMakePayments      sdk.Boolean `nexa:"string=Can Make Payments,default=true"`
	CanTakeMedicalAction sdk.Boolean `nexa:"string=Can Authorise Medical,default=false"`

	Status        sdk.Selection[LinkStatus] `nexa:"string=Status,default=active,index,selection=active:Active,revoked:Revoked"`
	ValidFrom     sdk.Date                  `nexa:"string=Valid From,default=today"`
	ValidTo       sdk.Date                  `nexa:"string=Valid To"`
	RevokedReason sdk.String                `nexa:"size=240,string=Revocation Reason"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("school_guardians",
		&SchoolGuardian{},
		&SchoolStudentGuardian{},
	)
}

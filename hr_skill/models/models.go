package models

import (
	"nexa/core/sdk"
	hrmodels "nexa_addons/hr/models"
)

// Cross-module reference aliases.
type (
	HrEmployee = hrmodels.HrEmployee
	HrJob      = hrmodels.HrJob
)

type SkillLevel string

const (
	SkillLevelBeginner     SkillLevel = "beginner"
	SkillLevelIntermediate SkillLevel = "intermediate"
	SkillLevelAdvanced     SkillLevel = "advanced"
	SkillLevelExpert       SkillLevel = "expert"
)

// HrSkillType categorizes skills (e.g. Technical, Communication, Leadership).
type HrSkillType struct {
	sdk.Model `nexa:"model=hr.skill.type"`

	Name   sdk.String  `nexa:"required,string=Name"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}

// HrSkill represents an individual skill definition.
type HrSkill struct {
	sdk.Model `nexa:"model=hr.skill"`

	Name   sdk.String                `nexa:"required,string=Skill"`
	TypeID sdk.Many2One[HrSkillType] `nexa:"required,string=Skill Type"`
	Active sdk.Boolean               `nexa:"string=Active,default=true"`
}

// HrEmployeeSkill records an employee's proficiency in a skill.
type HrEmployeeSkill struct {
	sdk.Model `nexa:"model=hr.employee.skill"`

	EmployeeID sdk.Many2One[HrEmployee]  `nexa:"required,string=Employee"`
	SkillID    sdk.Many2One[HrSkill]     `nexa:"required,string=Skill"`
	Level      sdk.Selection[SkillLevel] `nexa:"string=Proficiency,default=beginner"`
	Notes      sdk.Text                  `nexa:"string=Notes"`
}

// HrJobSkill defines a skill requirement for a job position.
type HrJobSkill struct {
	sdk.Model `nexa:"model=hr.job.skill"`

	JobID   sdk.Many2One[HrJob]       `nexa:"required,string=Job Position"`
	SkillID sdk.Many2One[HrSkill]     `nexa:"required,string=Required Skill"`
	Level   sdk.Selection[SkillLevel] `nexa:"string=Minimum Level,default=intermediate"`
}

func init() {
	sdk.MustRegister("hr_skill",
		&HrSkillType{},
		&HrSkill{},
		&HrEmployeeSkill{},
		&HrJobSkill{},
	)
}

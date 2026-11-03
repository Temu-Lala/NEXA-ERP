package models

import (
	"nexa/core/sdk"
)

// ProjectTemplate is a reusable blueprint of phases and tasks that can be
// instantiated into a concrete project, standardising how repeatable work starts.
type ProjectTemplate struct {
	sdk.Model `nexa:"model=project.template"`

	Name        sdk.String                         `nexa:"required,string=Template"`
	Category    sdk.String                         `nexa:"index,string=Category"`
	Description sdk.Text                           `nexa:"string=Description"`
	Active      sdk.Boolean                        `nexa:"string=Active,default=true"`
	Phases      sdk.One2Many[ProjectTemplatePhase] `nexa:"string=Phase Blueprint"`
	Tasks       sdk.One2Many[ProjectTemplateTask]  `nexa:"string=Task Blueprint"`
}

// ProjectTemplatePhase is a WBS phase in the blueprint with an optional planned
// duration (days) used as guidance when the project is created.
type ProjectTemplatePhase struct {
	sdk.Model `nexa:"model=project.template.phase"`

	TemplateID   sdk.Many2One[ProjectTemplate] `nexa:"required,index,string=Template"`
	Name         sdk.String                    `nexa:"required,string=Phase"`
	DurationDays sdk.Integer                   `nexa:"string=Duration (days),default=0"`
	Sequence     sdk.Integer                   `nexa:"string=Sequence,default=10"`
}

// ProjectTemplateTask is a task definition in the blueprint, optionally tied to a
// blueprint phase so the phase structure is reproduced on instantiation.
type ProjectTemplateTask struct {
	sdk.Model `nexa:"model=project.template.task"`

	TemplateID  sdk.Many2One[ProjectTemplate]      `nexa:"required,index,string=Template"`
	PhaseID     sdk.Many2One[ProjectTemplatePhase] `nexa:"string=Phase"`
	Name        sdk.String                         `nexa:"required,string=Task"`
	Priority    sdk.String                         `nexa:"string=Priority"`
	EstHours    sdk.Numeric                        `nexa:"string=Estimated Hours,precision=18,scale=2,default=0"`
	Sequence    sdk.Integer                        `nexa:"string=Sequence,default=10"`
	Description sdk.Text                           `nexa:"string=Description"`
}

func init() {
	sdk.MustRegister("project_template",
		&ProjectTemplate{},
		&ProjectTemplatePhase{},
		&ProjectTemplateTask{},
	)
}

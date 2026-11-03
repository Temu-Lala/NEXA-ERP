package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	projmodels "nexa_addons/project/models"
)

// Cross-module reference aliases.
type (
	ProjectProject = projmodels.ProjectProject
	CoreUser       = basemodels.CoreUser
)

// AllocationState is the lifecycle of a resource assignment.
type AllocationState string

const (
	AllocDraft     AllocationState = "draft"
	AllocConfirmed AllocationState = "confirmed"
	AllocReleased  AllocationState = "released"
)

// RequestState is the approval lifecycle of a capacity request. It is kept
// separate from AllocationState so a pending demand (submitted) never reads as a
// committed assignment (confirmed allocation).
type RequestState string

const (
	ReqDraft     RequestState = "draft"
	ReqSubmitted RequestState = "submitted"
	ReqApproved  RequestState = "approved"
	ReqRejected  RequestState = "rejected"
)

// ProjectResourceAllocation assigns an employee to a project with a planned
// allocation percentage for a period. One person may hold allocations across many
// projects; capacity is evaluated across all of them, not per project.
type ProjectResourceAllocation struct {
	sdk.Model `nexa:"model=project.resource.allocation"`

	ProjectID sdk.Many2One[ProjectProject]   `nexa:"required,index,string=Project"`
	UserID    sdk.Many2One[CoreUser]         `nexa:"required,index,string=Employee"`
	Role      sdk.String                     `nexa:"string=Role"`
	AllocPct  sdk.Numeric                    `nexa:"string=Allocation %,precision=18,scale=2,default=0"`
	StartDate sdk.Date                       `nexa:"string=From"`
	EndDate   sdk.Date                       `nexa:"string=To"`
	State     sdk.Selection[AllocationState] `nexa:"string=Status,default=draft"`
	Overalloc sdk.Boolean                    `nexa:"string=Over-allocated,default=false"`
	Note      sdk.Text                       `nexa:"string=Note"`
}

// ProjectResourceRequest is a demand for capacity that a manager approves before
// it becomes a confirmed allocation.
type ProjectResourceRequest struct {
	sdk.Model `nexa:"model=project.resource.request"`

	ProjectID   sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	UserID      sdk.Many2One[CoreUser]       `nexa:"string=Requested Employee"`
	Role        sdk.String                   `nexa:"string=Role Needed"`
	NeededPct   sdk.Numeric                  `nexa:"string=Needed %,precision=18,scale=2,default=0"`
	StartDate   sdk.Date                     `nexa:"string=From"`
	EndDate     sdk.Date                     `nexa:"string=To"`
	State       sdk.Selection[RequestState]  `nexa:"string=Status,default=draft"`
	RequestedBy sdk.Many2One[CoreUser]       `nexa:"string=Requested By"`
	ApprovedBy  sdk.Many2One[CoreUser]       `nexa:"string=Approved By"`
	Note        sdk.Text                     `nexa:"string=Justification"`
}

func init() {
	sdk.MustRegister("project_resource",
		&ProjectResourceAllocation{},
		&ProjectResourceRequest{},
	)
}

package models

import (
	"nexa/core/sdk"
)

// Releases (Jira "Versions"): a named, dated bundle of work a project ships.
// Issues point at one via fix_version_id; the Releases tab lists them with
// live roll-up counts computed from the issues that target each release.

const releaseModel = "project.release"

type ReleaseState string

const (
	ReleasePlanned   ReleaseState = "planned"
	ReleaseActive    ReleaseState = "in_progress"
	ReleaseReleased  ReleaseState = "released"
	ReleaseArchived  ReleaseState = "archived"
)

type ProjectRelease struct {
	sdk.Model `nexa:"model=project.release"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name        sdk.String               `nexa:"required,string=Name"`
	ProjectID   sdk.Many2One[ProjectProject]  `nexa:"required,index,string=Project"`
	State       sdk.Selection[ReleaseState] `nexa:"string=Status,default=planned,selection=planned:Planned,in_progress:In Progress,released:Released,archived:Archived"`
	ReleaseDate sdk.Date                 `nexa:"string=Release Date"`
	Description sdk.Text                 `nexa:"string=Description"`
}

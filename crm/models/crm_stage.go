package models

import (
	"nexa/core/sdk"
)

type CrmStage struct {
	sdk.Model `nexa:"model=crm.stage"`

	Name                 sdk.String            `nexa:"required,string=Stage Name"`
	Sequence             sdk.Integer           `nexa:"string=Sequence,default=1"`
	IsWon                sdk.Boolean           `nexa:"string=Is Won Stage?"`
	Fold                 sdk.Boolean           `nexa:"string=Folded in Pipeline"`
	Requirements         sdk.Text              `nexa:"string=Requirements"`
	Active               sdk.Boolean           `nexa:"string=Active,default=true"`
	TeamIDs              sdk.Many2Many[CrmTeam] `nexa:"string=Sales Teams,table=crm_stage_team_rel,left=stage_id,right=team_id"`
	RottingThresholdDays sdk.Integer           `nexa:"string=Days to Rot,default=0"`
	Color                sdk.Integer           `nexa:"string=Color Index,default=0"`
}

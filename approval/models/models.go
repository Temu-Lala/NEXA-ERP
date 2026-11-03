package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// CoreUser → res.users (base)
type CoreUser = basemodels.CoreUser

type RequestState string

const (
	RequestStatePending   RequestState = "pending"
	RequestStateApproved  RequestState = "approved"
	RequestStateRejected  RequestState = "rejected"
	RequestStateCancelled RequestState = "cancelled"
)

type DecisionState string

const (
	DecisionStateNone     DecisionState = "none"
	DecisionStateApproved DecisionState = "approved"
	DecisionStateRejected DecisionState = "rejected"
)

type ApprovalCategory struct {
	sdk.Model `nexa:"model=approval.category"`

	Name          sdk.String  `nexa:"required,string=Name"`
	ResourceModel sdk.String  `nexa:"string=Model (technical)"`
	Active        sdk.Boolean `nexa:"string=Active,default=true"`
}

type ApprovalLevel struct {
	sdk.Model `nexa:"model=approval.level"`

	Name       sdk.String                     `nexa:"required,string=Level Name"`
	CategoryID sdk.Many2One[ApprovalCategory] `nexa:"required,string=Category"`
	Sequence   sdk.Integer                    `nexa:"string=Sequence,default=10"`
	ApproverID sdk.Many2One[CoreUser]         `nexa:"string=Approver"`
	AmountFrom sdk.Numeric                    `nexa:"string=Amount From,precision=18,scale=2,default=0"`
	AmountTo   sdk.Numeric                    `nexa:"string=Amount To,precision=18,scale=2"`
}

type ApprovalRequest struct {
	sdk.Model `nexa:"model=approval.request"`

	Name          sdk.String                     `nexa:"required,string=Reference"`
	CategoryID    sdk.Many2One[ApprovalCategory] `nexa:"required,string=Category"`
	ResourceModel sdk.String                     `nexa:"string=Record Model"`
	ResourceID    sdk.Integer                    `nexa:"string=Record ID"`
	Amount        sdk.Numeric                    `nexa:"string=Amount,precision=18,scale=2,default=0"`
	State         sdk.Selection[RequestState]    `nexa:"string=Status,default=pending"`
	RequesterID   sdk.Many2One[CoreUser]         `nexa:"string=Requested By"`
	Description   sdk.Text                       `nexa:"string=Description"`
}

type ApprovalAction struct {
	sdk.Model `nexa:"model=approval.action"`

	RequestID  sdk.Many2One[ApprovalRequest] `nexa:"required,string=Request"`
	LevelID    sdk.Many2One[ApprovalLevel]   `nexa:"string=Level"`
	ApproverID sdk.Many2One[CoreUser]        `nexa:"string=Approver"`
	Decision   sdk.Selection[DecisionState]  `nexa:"string=Decision,default=none"`
	Comment    sdk.Text                      `nexa:"string=Comment"`
	ActionDate sdk.DateTime                  `nexa:"string=Date"`
}

func init() {
	sdk.MustRegister("approval",
		&ApprovalCategory{},
		&ApprovalLevel{},
		&ApprovalRequest{},
		&ApprovalAction{},
	)
}

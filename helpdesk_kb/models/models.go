package models

import (
	"nexa/core/sdk"
	hdmodels "nexa_addons/helpdesk/models"
)

// Cross-module reference aliases.
type (
	CoreUser = hdmodels.CoreUser
)

type ArticleState string

const (
	ArticleStateDraft     ArticleState = "draft"
	ArticleStateReview    ArticleState = "review"
	ArticleStatePublished ArticleState = "published"
	ArticleStateArchived  ArticleState = "archived"
)

type ArticleVisibility string

const (
	VisibilityPublic     ArticleVisibility = "public"
	VisibilityInternal   ArticleVisibility = "internal"
	VisibilityRestricted ArticleVisibility = "restricted"
)

// HelpdeskKBCategory organizes knowledge articles.
type HelpdeskKBCategory struct {
	sdk.Model `nexa:"model=helpdesk.kb.category"`

	Name     sdk.String                       `nexa:"required,string=Name"`
	ParentID sdk.Many2One[HelpdeskKBCategory] `nexa:"string=Parent Category"`
	Sequence sdk.Integer                      `nexa:"string=Sequence,default=10"`
	Active   sdk.Boolean                      `nexa:"string=Active,default=true"`
}

// HelpdeskKBArticle is a knowledge base article.
type HelpdeskKBArticle struct {
	sdk.Model `nexa:"model=helpdesk.kb.article"`

	Name           sdk.String                       `nexa:"required,string=Title"`
	Summary        sdk.String                       `nexa:"string=Summary"`
	Content        sdk.Text                         `nexa:"string=Content"`
	CategoryID     sdk.Many2One[HelpdeskKBCategory] `nexa:"string=Category"`
	AuthorID       sdk.Many2One[CoreUser]           `nexa:"string=Author"`
	Visibility     sdk.Selection[ArticleVisibility] `nexa:"string=Visibility,default=public"`
	State          sdk.Selection[ArticleState]      `nexa:"string=Status,default=draft"`
	ViewCount      sdk.Integer                      `nexa:"string=Views,default=0"`
	HelpfulCount   sdk.Integer                      `nexa:"string=Helpful,default=0"`
	UnhelpfulCount sdk.Integer                      `nexa:"string=Not Helpful,default=0"`
	PublishedDate  sdk.Date                         `nexa:"string=Published Date"`
	Active         sdk.Boolean                      `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("helpdesk_kb",
		&HelpdeskKBCategory{},
		&HelpdeskKBArticle{},
	)
}

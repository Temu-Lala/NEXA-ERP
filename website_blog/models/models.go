package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// CoreUser → core.user
type CoreUser = basemodels.CoreUser

type PostState string

const (
	PostStateDraft     PostState = "draft"
	PostStatePublished PostState = "published"
	PostStateArchived  PostState = "archived"
)

type BlogTag struct {
	sdk.Model `nexa:"model=blog.tag"`

	Name   sdk.String  `nexa:"required,string=Tag Name"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}

type BlogPost struct {
	sdk.Model `nexa:"model=blog.post"`

	Name     sdk.String               `nexa:"required,string=Title"`
	Slug     sdk.String               `nexa:"string=Slug"`
	AuthorID sdk.Many2One[CoreUser]   `nexa:"string=Author"`
	TagID    sdk.Many2One[BlogTag]    `nexa:"string=Tag"`
	BlogDate sdk.DateTime             `nexa:"string=Publish Date"`
	Summary  sdk.Text                 `nexa:"string=Summary"`
	Content  sdk.HTML                 `nexa:"string=Content"`
	Views    sdk.Integer              `nexa:"string=Views,default=0"`
	State    sdk.Selection[PostState] `nexa:"string=Status,default=draft"`
	Active   sdk.Boolean              `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("website_blog",
		&BlogTag{},
		&BlogPost{},
	)
}

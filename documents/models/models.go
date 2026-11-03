package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// CoreUser → core.user
type CoreUser = basemodels.CoreUser

type DocumentState string

const (
	DocumentStateUpload    DocumentState = "upload"
	DocumentStateValidated DocumentState = "validated"
	DocumentStateLocked    DocumentState = "locked"
	DocumentStateArchived  DocumentState = "archived"
)

type DocumentsFolder struct {
	sdk.Model `nexa:"model=documents.folder"`

	Name   sdk.String  `nexa:"required,string=Folder Name"`
	Notes  sdk.Text    `nexa:"string=Notes"`
	Active sdk.Boolean `nexa:"string=Active,default=true"`
}

type DocumentsDocument struct {
	sdk.Model `nexa:"model=documents.document"`

	Name       sdk.String                    `nexa:"required,string=Name"`
	FolderID   sdk.Many2One[DocumentsFolder] `nexa:"string=Folder"`
	OwnerID    sdk.Many2One[CoreUser]        `nexa:"string=Owner"`
	DocType    sdk.String                    `nexa:"string=Type"`
	UploadDate sdk.Date                      `nexa:"string=Upload Date"`
	State      sdk.Selection[DocumentState]  `nexa:"string=Status,default=upload"`
	Notes      sdk.Text                      `nexa:"string=Notes"`
	Active     sdk.Boolean                   `nexa:"string=Active,default=true"`
}

func init() {
	sdk.MustRegister("documents",
		&DocumentsFolder{},
		&DocumentsDocument{},
	)
}

package models

import (
	"context"

	"nexa/core/orm"
	"nexa/core/sdk"
)

// Custom fields (spec foundation). A field DEFINITION declares a typed,
// user-authored attribute on project.task; VALUES are stored one row per
// (field, task) so types stay validated and lists/reports can query them.
// This mirrors Jira's admin-created custom fields without touching core schema.

const (
	fieldDefModel   = "project.field.definition"
	fieldValueModel = "project.field.value"
)

type FieldType string

const (
	FieldText     FieldType = "text"
	FieldTextarea FieldType = "textarea"
	FieldInteger  FieldType = "integer"
	FieldFloat    FieldType = "float"
	FieldBoolean  FieldType = "boolean"
	FieldDate     FieldType = "date"
	FieldDatetime FieldType = "datetime"
	FieldSelect   FieldType = "select"
	FieldRef      FieldType = "reference"
)

// ProjectFieldDefinition is one admin-defined custom field on issues.
type ProjectFieldDefinition struct {
	sdk.Model `nexa:"model=project.field.definition"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name      sdk.String               `nexa:"required,string=Name"`
	Code      sdk.String               `nexa:"required,string=Code"`
	FieldType sdk.Selection[FieldType] `nexa:"required,string=Type,selection=text:Text,textarea:Text (multi-line),integer:Integer,float:Decimal,boolean:Checkbox,date:Date,datetime:Date & Time,select:Selection,reference:Reference"`
	Options   sdk.String               `nexa:"string=Options,help='For select: one option per line'"`
	Required  sdk.Boolean              `nexa:"string=Required,default=false"`
	Sequence  sdk.Integer              `nexa:"string=Sequence,default=10"`
	Active    sdk.Boolean              `nexa:"string=Active,default=true"`
}

// ProjectFieldValue stores one custom-field value on one task. Exactly one of
// the typed columns is meaningful, chosen by the definition's field_type;
// reference fields use ValueRefModel + ValueRefID.
type ProjectFieldValue struct {
	sdk.Model `nexa:"model=project.field.value"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	FieldID sdk.Many2One[ProjectFieldDefinition] `nexa:"required,index,ondelete=cascade,string=Field"`
	TaskID  sdk.Many2One[ProjectTask]            `nexa:"required,index,ondelete=cascade,string=Issue"`

	ValueText     sdk.String  `nexa:"string=Text"`
	ValueNumber   sdk.Float   `nexa:"string=Number"`
	ValueBool     sdk.Boolean `nexa:"string=Boolean"`
	ValueDate     sdk.Date    `nexa:"string=Date"`
	ValueRefModel sdk.String  `nexa:"string=Ref Model"`
	ValueRefID    sdk.Integer `nexa:"string=Ref ID"`
}

func init() {
	// One row per (field, task): writes upsert rather than accumulate.
	orm.RegisterConstraint(fieldValueModel, "field_value_unique", constraintFieldValueUnique)
}

func constraintFieldValueUnique(ctx context.Context, rec map[string]interface{}) error {
	fid, tid := intID(rec["field_id"]), intID(rec["task_id"])
	if fid <= 0 || tid <= 0 {
		return nil
	}
	id := intID(rec["id"])
	rows, err := orm.SearchPage(ctx, fieldValueModel,
		[][]interface{}{{"field_id", "=", fid}, {"task_id", "=", tid}}, 10, 0, "id asc")
	if err != nil {
		return nil // never block writes on lookup failure
	}
	for _, row := range rows {
		if rid := intID(row["id"]); rid != id && rid > 0 {
			return errDuplicateFieldValue
		}
	}
	return nil
}

var errDuplicateFieldValue = &fieldValueError{"a value already exists for this field on this issue"}

type fieldValueError struct{ msg string }

func (e *fieldValueError) Error() string { return e.msg }

package models

import (
	"nexa/core/sdk"
	productmodels "nexa_addons/product/models"
)

// ProductProduct → product.product
type ProductProduct = productmodels.ProductProduct

// AttributeKind is the data type of an attribute's values.
type AttributeKind string

const (
	KindSelect      AttributeKind = "select"
	KindMultiselect AttributeKind = "multiselect"
	KindNumeric     AttributeKind = "numeric"
	KindText        AttributeKind = "text"
	KindBoolean     AttributeKind = "boolean"
	KindDate        AttributeKind = "date"
)

// ProductAttribute is a reusable, typed product attribute definition.
type ProductAttribute struct {
	sdk.Model `nexa:"model=product.attribute"`

	Name            sdk.String                          `nexa:"required,index,string=Attribute"`
	Kind            sdk.Selection[AttributeKind]        `nexa:"string=Type,default=select"`
	AttributeValues sdk.One2Many[ProductAttributeValue] `nexa:"string=Values"`
	Required        sdk.Boolean                         `nexa:"string=Required,default=false"`
	Active          sdk.Boolean                         `nexa:"string=Active,default=true"`
}

// ProductAttributeValue is an allowed value for an attribute.
type ProductAttributeValue struct {
	sdk.Model `nexa:"model=product.attribute.value"`

	AttributeID sdk.Many2One[ProductAttribute] `nexa:"required,index,string=Attribute"`
	Name        sdk.String                     `nexa:"required,string=Value"`
	Code        sdk.String                     `nexa:"string=Code"`
	Sequence    sdk.Integer                    `nexa:"string=Sequence,default=10"`
	Active      sdk.Boolean                    `nexa:"string=Active,default=true"`
}

// ProductAttributeLine assigns attribute values to a product.
type ProductAttributeLine struct {
	sdk.Model `nexa:"model=product.attribute.line"`

	ProductID   sdk.Many2One[ProductProduct]         `nexa:"required,index,string=Product"`
	AttributeID sdk.Many2One[ProductAttribute]       `nexa:"required,index,string=Attribute"`
	ValueIDs    sdk.Many2Many[ProductAttributeValue] `nexa:"string=Values,table=product_attribute_value_rel,left=line_id,right=value_id"`
	Required    sdk.Boolean                          `nexa:"string=Required,default=false"`
}

func init() {
	sdk.MustRegister("product_attribute",
		&ProductAttribute{},
		&ProductAttributeValue{},
		&ProductAttributeLine{},
	)
}

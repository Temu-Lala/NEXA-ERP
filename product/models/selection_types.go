package models

type ProductType string

const (
	ProductTypeConsu    ProductType = "consu"
	ProductTypeService  ProductType = "service"
	ProductTypeStorable ProductType = "storable"
	ProductTypeDigital  ProductType = "digital"
	ProductTypeKit      ProductType = "kit"
)

// ProductState is the controlled catalog lifecycle.
type ProductState string

const (
	ProductStateDraft        ProductState = "draft"
	ProductStateReview       ProductState = "review"
	ProductStateActive       ProductState = "active"
	ProductStateDiscontinued ProductState = "discontinued"
	ProductStateArchived     ProductState = "archived"
)

// ProductCostMethod is the valuation approach for a storable product.
type ProductCostMethod string

const (
	CostMethodStandard ProductCostMethod = "standard"
	CostMethodAverage  ProductCostMethod = "average"
	CostMethodFIFO     ProductCostMethod = "fifo"
)

// ProductTracking governs lot/serial traceability for a storable product.
type ProductTracking string

const (
	TrackingNone   ProductTracking = "none"
	TrackingLot    ProductTracking = "lot"
	TrackingSerial ProductTracking = "serial"
)

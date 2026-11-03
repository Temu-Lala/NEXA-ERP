package models

type LoanState string

const (
	LoanStateDraft     LoanState = "draft"
	LoanStateBorrowed  LoanState = "borrowed"
	LoanStateReturned  LoanState = "returned"
	LoanStateCancelled LoanState = "cancelled"
)

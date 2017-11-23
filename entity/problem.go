package entity

import (
	"time"
)

// constant values
const (
	KindNameProblem   = "Problem"
	ProblemStockCount = 50
)

// Problem type
type Problem struct {
	State     string    `datastore:"state,noindex"`
	Csa       string    `datastore:"csa,noindex"`
	Type      int       `datastore:"type"`
	Used      bool      `datastore:"used"`
	Answer    []string  `datastore:"answer,noindex"`
	CreatedAt time.Time `datastore:"created_at"`
	UpdatedAt time.Time `datastore:"updated_at"`
}

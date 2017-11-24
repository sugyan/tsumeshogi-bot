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
	CSA       string    `datastore:"csa,noindex"`
	Type      int       `datastore:"type"`
	Used      bool      `datastore:"used"`
	Images    []string  `datastore:"images,noindex"`
	QImage    string    `datastore:"q_image,noindex"`
	AImage    string    `datastore:"a_image,noindex"`
	CreatedAt time.Time `datastore:"created_at"`
	UpdatedAt time.Time `datastore:"updated_at"`
}

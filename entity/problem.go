package entity

import (
	"time"
)

// constant values
const (
	KindNameProblem   = "Problem"
	ProblemStockCount = 100
)

// Problem type
type Problem struct {
	CSA       string    `datastore:"csa,noindex"`
	Type      int       `datastore:"type"`
	Used      bool      `datastore:"used"`
	QImage    string    `datastore:"q_image,noindex"`
	AImage    string    `datastore:"a_image,noindex"`
	Score     int       `datastore:"score"`
	CreatedAt time.Time `datastore:"created_at"`
	UpdatedAt time.Time `datastore:"updated_at"`
}

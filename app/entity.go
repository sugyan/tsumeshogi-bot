package app

// constant values
const (
	KindNameProblem = "Problem"
	StockCount      = 50
)

type problemEntity struct {
	State     string   `datastore:"state,noindex"`
	Type      int      `datastore:"type"`
	Used      bool     `datastore:"used"`
	Answer    []string `datastore:"answer,noindex"`
	CreatedAt int64    `datastore:"created_at"`
	UpdatedAt int64    `datastore:"updated_at"`
}

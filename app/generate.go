package app

import (
	"context"
	"net/http"
	"time"

	"github.com/sugyan/shogi/format/csa"
	"github.com/sugyan/shogi/logic/problem/generator"
	"github.com/sugyan/shogi/logic/problem/solver"
	"github.com/sugyan/shogi/record"
	"github.com/sugyan/tsumeshogi_bot/entity"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

func (s *server) generateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Infof(ctx, "generate...")
	for _, problemType := range []generator.Problem{
		generator.ProblemType1,
		generator.ProblemType3,
	} {
		if err := generateAndSave(ctx, problemType); err != nil {
			log.Errorf(ctx, "failed to generate: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

func generateAndSave(ctx context.Context, problemType generator.Problem) error {
	count, err := datastore.NewQuery(entity.KindNameProblem).
		Filter("type = ", problemType.Steps()).
		Filter("used = ", false).
		Count(ctx)
	if err != nil {
		return err
	}
	log.Infof(ctx, "type: %v, count: %v", problemType.Steps(), count)
	if count >= entity.ProblemStockCount {
		return nil
	}
	p := generator.Generate(problemType)
	answer, err := solver.Solve(p)
	if err != nil {
		return err
	}
	record := &record.Record{
		State: p,
		Moves: answer,
	}
	answerString, err := p.MoveStrings(answer)
	if err != nil {
		return err
	}
	problem := &entity.Problem{
		State: csa.InitialState2(p),
		Csa: record.ConvertToString(csa.NewConverter(&csa.ConvertOption{
			InitialState: csa.InitialStateOption2,
		})),
		Type:      len(answer),
		Answer:    answerString,
		Used:      false,
		CreatedAt: time.Now(),
	}
	key, err := datastore.Put(ctx, datastore.NewIncompleteKey(ctx, entity.KindNameProblem, nil), problem)
	if err != nil {
		return err
	}
	log.Infof(ctx, "key: %v", key)
	return nil
}

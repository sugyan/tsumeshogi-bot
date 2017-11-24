package app

import (
	"context"
	"net/http"
	"time"

	"github.com/sugyan/shogi/format/csa"
	"github.com/sugyan/shogi/logic/problem/generator"
	"github.com/sugyan/shogi/logic/problem/solver"
	"github.com/sugyan/shogi/record"
	"github.com/sugyan/tsumeshogi-bot/entity"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

func (s *server) generateHandler(w http.ResponseWriter, r *http.Request) {
	// cron request only
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	ctx := appengine.NewContext(r)
	log.Infof(ctx, "generate...")
	for _, problemType := range []generator.Problem{
		generator.ProblemType1,
		generator.ProblemType3,
	} {
		if err := s.generateAndSave(ctx, problemType); err != nil {
			log.Errorf(ctx, "failed to generate: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

func (s *server) generateAndSave(ctx context.Context, problemType generator.Problem) error {
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
	q := generator.Generate(problemType)
	a, err := solver.Solve(q)
	if err != nil {
		return err
	}
	record := &record.Record{
		State: q,
		Moves: a,
	}
	state := q.Clone()
	qImage, err := s.uploadImage(ctx, state, nil)
	if err != nil {
		return err
	}
	for _, m := range a {
		state.Apply(m)
	}
	aImage, err := s.uploadImage(ctx, state, &a[len(a)-1].Dst)
	if err != nil {
		return err
	}
	problem := &entity.Problem{
		CSA: record.ConvertToString(csa.NewConverter(&csa.ConvertOption{
			InitialState: csa.InitialStateOption2,
		})),
		Type:      len(a),
		Used:      false,
		QImage:    qImage,
		AImage:    aImage,
		CreatedAt: time.Now(),
	}
	_, err = datastore.Put(ctx, datastore.NewIncompleteKey(ctx, entity.KindNameProblem, nil), problem)
	return err
}

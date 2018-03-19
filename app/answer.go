package app

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	"github.com/sugyan/shogi"
	"github.com/sugyan/shogi/format/csa"
	"github.com/sugyan/tsumeshogi-bot/entity"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

func (s *server) answerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	csa := false
	encodedKey := strings.TrimPrefix(r.URL.Path, "/answer/")
	if strings.HasSuffix(encodedKey, ".csa") {
		csa = true
		encodedKey = strings.TrimSuffix(encodedKey, ".csa")
	}
	problem, _, err := getProblem(ctx, encodedKey)
	if err != nil {
		log.Infof(ctx, "failed to get problem: %v", err.Error())
		http.NotFound(w, r)
		return
	}
	if csa {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(problem.CSA))
		return
	}
	answer, _, err := generateAnswer(problem)
	if err != nil {
		log.Errorf(ctx, "failed to retrieve answer: %v", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := renderTemplate(w, "answer", map[string]string{
		"answer": strings.Join(answer, " "),
	}); err != nil {
		log.Errorf(ctx, "failed to render template: %v", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func generateAnswer(problem *entity.Problem) ([]string, *shogi.State, error) {
	record, err := csa.Parse(bytes.NewBufferString(problem.CSA))
	if err != nil {
		return nil, nil, err
	}
	answer := []string{}
	state := record.State.Clone()
	for _, move := range record.Moves {
		ms, err := state.MoveString(move)
		if err != nil {
			return nil, nil, err
		}
		answer = append(answer, ms)
		state.Apply(move)
	}
	return answer, state, nil
}

func getProblem(ctx context.Context, encoded string) (*entity.Problem, *datastore.Key, error) {
	var problem entity.Problem
	key, err := datastore.DecodeKey(encoded)
	if err != nil {
		return nil, nil, err
	}
	if err := datastore.Get(ctx, key, &problem); err != nil {
		return nil, nil, err
	}
	return &problem, key, nil
}

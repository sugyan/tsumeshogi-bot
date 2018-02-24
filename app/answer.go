package app

import (
	"bytes"
	"context"
	"fmt"
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

	encodedKey := strings.TrimPrefix(r.URL.Path, "/answer/")
	problem, _, err := getProblem(ctx, encodedKey)
	if err != nil {
		log.Infof(ctx, "failed to get problem: %v", err.Error())
		http.NotFound(w, r)
		return
	}
	answer, state, err := generateAnswer(problem)
	if err != nil {
		log.Errorf(ctx, "failed to retrieve answer: %v", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	answerImageURL := fmt.Sprintf(
		"https://shogi-img.appspot.com/%s/answer.png",
		strings.Join(strings.Split(strings.TrimSpace(csa.InitialState2(state)), "\n"), "/"))
	html := `<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
	<meta name="viewport" content="width=device-width,initial-scale=1">
	<title>詰将棋BOT</title>
  </head>
  <body>
    <p>` + fmt.Sprintf("正解は、 %s です！", strings.Join(answer, " ")) + `</p>
    <img style="max-width: 100%;" src="` + answerImageURL + `">
  </body>
</html>`
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
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

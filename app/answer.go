package app

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

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
	answer, err := answerString(problem)
	if err != nil {
		log.Errorf(ctx, "failed to retrieve answer: %v", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	html := `<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
	<meta name="viewport" content="width=device-width,initial-scale=1">
	<title>詰将棋BOT</title>
  </head>
  <body>
    <p>` + fmt.Sprintf("正解は、 %s です！", strings.Join(answer, " ")) + `</p>
    <img style="max-width: 100%;" src="` + problem.AImage + `">
  </body>
</html>`
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func answerString(problem *entity.Problem) ([]string, error) {
	record, err := csa.Parse(bytes.NewBufferString(problem.CSA))
	if err != nil {
		return nil, err
	}
	answer, err := record.State.MoveStrings(record.Moves)
	if err != nil {
		return nil, err
	}
	return answer, nil
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

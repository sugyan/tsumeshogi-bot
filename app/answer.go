package app

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/sugyan/shogi/format/csa"
	"github.com/sugyan/tsumeshogi-bot/entity"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

func (s *server) answerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	encodedKey := strings.TrimPrefix(r.URL.Path, "/answer/")
	problem, err := getProblem(ctx, encodedKey)
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
` + fmt.Sprintf("正解は、 %s です！", strings.Join(answer, " ")) + `
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

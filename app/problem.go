package app

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/sugyan/shogi/format/csa"

	"github.com/sugyan/shogi/logic/problem/generator"
	"github.com/sugyan/tsumeshogi-bot/entity"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

func (s *server) problemHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	t := r.URL.Query().Get("type")
	var (
		problem *entity.Problem
		key     *datastore.Key
		err     error
	)
	switch t {
	case "1":
		problem, key, err = s.fetchProblem(ctx, generator.Type1)
	case "3":
		problem, key, err = s.fetchProblem(ctx, generator.Type3)
	case "5":
		problem, key, err = s.fetchProblem(ctx, generator.Type5)
	default:
		log.Errorf(ctx, "type '%v' is invalid", t)
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Errorf(ctx, "failed to fetch problem: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	record, err := csa.Parse(bytes.NewBufferString(problem.CSA))
	if err != nil {
		log.Errorf(ctx, "failed to parse problem: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	buf := bytes.NewBufferString(fmt.Sprintf("'%s\n", key.Encode()))
	convertOption := &csa.ConvertOption{
		InitialState: csa.InitialStateOption1,
	}
	if r.URL.Query().Get("short") != "" {
		convertOption.InitialState = csa.InitialStateOption2
	}
	buf.WriteString(record.ConvertToString(csa.NewConverter(convertOption)))
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.Write(buf.Bytes())
}

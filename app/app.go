package app

import (
	"context"
	"math/rand"
	"net/http"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"github.com/sugyan/shogi/logic/problem/generator"
	"github.com/sugyan/tsumeshogi-bot/config"
	"github.com/sugyan/tsumeshogi-bot/entity"
	"google.golang.org/appengine/datastore"
)

type server struct {
	config *config.Config
}

func init() {
	config, err := config.LoadConfig("config.toml")
	if err != nil {
		panic(err)
	}
	anaconda.SetConsumerKey(config.TwitterBot.ConsumerKey)
	anaconda.SetConsumerSecret(config.TwitterBot.ConsumerSecret)

	server := &server{
		config: config,
	}
	http.HandleFunc("/callback", server.callbackHandler)
	http.HandleFunc("/tweet", server.tweetHandler)
	http.HandleFunc("/answer/", server.answerHandler)
	http.HandleFunc("/problem", server.problemHandler)
}

func (s *server) fetchProblem(ctx context.Context, problemType generator.Problem) (*entity.Problem, *datastore.Key, error) {
	query := datastore.NewQuery(entity.KindNameProblem).
		Filter("type = ", problemType.Steps())
	iter := query.
		Filter("used = ", false).
		Run(ctx)
	var problem entity.Problem
	key, err := iter.Next(&problem)
	if err != nil {
		if err != datastore.Done {
			return nil, nil, err
		}
		count, err := query.Count(ctx)
		if err != nil {
			return nil, nil, err
		}
		if count == 0 {
			return nil, nil, datastore.Done
		}
		if count > 100 {
			count = 100
		}
		key, err = query.
			Order("-score").
			Offset(rand.Intn(count)).
			Run(ctx).
			Next(&problem)
		if err != nil {
			return nil, nil, err
		}
	} else {
		problem.Used = true
		problem.UpdatedAt = time.Now()
		_, err = datastore.Put(ctx, key, &problem)
		if err != nil {
			return nil, nil, err
		}
	}
	return &problem, key, nil
}

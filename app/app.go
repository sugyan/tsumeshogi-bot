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
	// fetch high scored problems
	baseQuery := datastore.NewQuery(entity.KindNameProblem).
		Filter("type = ", problemType.Steps()).
		Order("-score")
	candidates := make([]*datastore.Key, 0, 10)
fetch:
	for _, used := range []bool{false, true} {
		iter := baseQuery.Filter("used = ", used).Run(ctx)
		for {
			key, err := iter.Next(nil)
			if err != nil {
				if err == datastore.Done {
					continue fetch
				} else {
					return nil, nil, err
				}
			}
			candidates = append(candidates, key)
			if len(candidates) >= 10 {
				break fetch
			}
		}
	}
	if len(candidates) == 0 {
		return nil, nil, datastore.Done
	}
	// select from candidates randomly
	key := candidates[rand.Intn(len(candidates))]
	var problem entity.Problem
	if err := datastore.Get(ctx, key, &problem); err != nil {
		return nil, nil, err
	}
	// mark as used
	if !problem.Used {
		problem.Used = true
		problem.UpdatedAt = time.Now()
		if _, err := datastore.Put(ctx, key, &problem); err != nil {
			return nil, nil, err
		}
	}
	return &problem, key, nil
}

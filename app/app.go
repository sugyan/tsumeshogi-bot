package app

import (
	"context"
	"math/rand"
	"net/http"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"github.com/sugyan/shogi/logic/problem/generator"
	"github.com/sugyan/tsumeshogi_bot/config"
	"github.com/sugyan/tsumeshogi_bot/entity"
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
	http.HandleFunc("/generate", server.generateHandler)
	http.HandleFunc("/tweet", server.tweetHandler)
	http.HandleFunc("/answer/", server.answerHandler)
}

func fetchProblem(ctx context.Context, problemType generator.Problem) (*entity.Problem, string, error) {
	query := datastore.NewQuery(entity.KindNameProblem).
		Filter("type = ", problemType.Steps())
	iter := query.
		Filter("used = ", false).
		Run(ctx)
	var problem entity.Problem
	key, err := iter.Next(&problem)
	if err != nil {
		if err != datastore.Done {
			return nil, "", err
		}
		count, err := query.Count(ctx)
		if err != nil {
			return nil, "", err
		}
		if count == 0 {
			return nil, "", datastore.Done
		}
		key, err = query.Offset(rand.Intn(count)).Run(ctx).Next(&problem)
		if err != nil {
			return nil, "", err
		}
	} else {
		problem.Used = true
		problem.UpdatedAt = time.Now()
		_, err = datastore.Put(ctx, key, &problem)
		if err != nil {
			return nil, "", err
		}
	}
	return &problem, key.Encode(), nil
}

func getProblem(ctx context.Context, encoded string) (*entity.Problem, error) {
	var problem entity.Problem
	key, err := datastore.DecodeKey(encoded)
	if err != nil {
		return nil, err
	}
	if err := datastore.Get(ctx, key, &problem); err != nil {
		return nil, err
	}
	return &problem, nil
}

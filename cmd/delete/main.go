package main

import (
	"context"
	"log"
	"time"

	"github.com/sugyan/tsumeshogi-bot/config"
	"github.com/sugyan/tsumeshogi-bot/entity"
	"golang.org/x/oauth2/google"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/remote_api"
)

type deleter struct {
	config *config.Config
}

func main() {
	config, err := config.LoadConfig("app/config.toml")
	if err != nil {
		log.Fatal(err)
	}

	client, err := google.DefaultClient(context.Background(),
		"https://www.googleapis.com/auth/appengine.apis",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		log.Fatal(err)
	}
	ctx, err := remote_api.NewRemoteContext(config.Host, client)
	if err != nil {
		log.Fatal(err)
	}

	deleter := &deleter{config: config}
	if err := deleter.deleteOldProblems(ctx); err != nil {
		log.Fatal(err)
	}
}

func (d *deleter) deleteOldProblems(ctx context.Context) error {
	iter := datastore.NewQuery(entity.KindNameProblem).
		Filter("created_at < ", time.Now().Add(-time.Hour*24*60)).
		Run(ctx)
	for {
		var p entity.Problem
		key, err := iter.Next(&p)
		if err == datastore.Done {
			break
		}
		if err != nil {
			return err
		}
		if err := p.Delete(ctx, key); err != nil {
			return err
		}
		log.Printf("%v deleted.", key.IntID())
		time.Sleep(time.Second)
	}
	return nil
}

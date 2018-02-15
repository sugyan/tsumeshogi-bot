package main

import (
	"context"
	"fmt"
	"image/png"
	golog "log"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sugyan/shogi"
	"github.com/sugyan/shogi/format/csa"
	"github.com/sugyan/shogi/logic/problem/generator"
	"github.com/sugyan/shogi/logic/problem/solver"
	"github.com/sugyan/shogi/record"
	"github.com/sugyan/shogi/util/image"
	"github.com/sugyan/tsumeshogi-bot/config"
	"github.com/sugyan/tsumeshogi-bot/entity"
	"golang.org/x/oauth2/google"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/remote_api"
)

type problemGenerator struct {
	config *config.Config
}

func main() {
	config, err := config.LoadConfig("app/config.toml")
	if err != nil {
		golog.Fatal(err)
	}
	pg := &problemGenerator{
		config: config,
	}

	client, err := google.DefaultClient(context.Background(),
		"https://www.googleapis.com/auth/appengine.apis",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		golog.Fatal(err)
	}
	ctx, err := remote_api.NewRemoteContext(config.Host, client)
	if err != nil {
		golog.Fatal(err)
	}

	for _, problemType := range []generator.Problem{
		generator.Type1,
		generator.Type3,
	} {
		if err := pg.generateProblem(ctx, problemType); err != nil {
			log.Errorf(ctx, "generate error %d: %v", problemType.Steps(), err)
		}
	}
}

func (pg *problemGenerator) generateProblem(ctx context.Context, problemType generator.Problem) error {
	count, err := datastore.NewQuery(entity.KindNameProblem).
		Filter("type = ", problemType.Steps()).
		Filter("used = ", false).
		Count(ctx)
	if err != nil {
		return err
	}
	if count >= entity.ProblemStockCount {
		return nil
	}
	log.Infof(ctx, "count: %v", count)

	// generate problem
	q, score := generator.Generate(problemType)
	a := solver.Solve(q)
	record := &record.Record{
		State: q,
		Moves: a,
	}
	// generate image
	state := q.Clone()
	qImage, err := pg.uploadImage(ctx, state, nil)
	if err != nil {
		return err
	}
	for _, m := range a {
		state.Apply(m)
	}
	aImage, err := pg.uploadImage(ctx, state, &a[len(a)-1].Dst)
	if err != nil {
		return err
	}
	// save
	problem := &entity.Problem{
		CSA: record.ConvertToString(csa.NewConverter(&csa.ConvertOption{
			InitialState: csa.InitialStateOption2,
		})),
		Type:      len(a),
		Used:      false,
		QImage:    qImage,
		AImage:    aImage,
		Score:     score,
		CreatedAt: time.Now(),
	}
	key, err := datastore.Put(ctx, datastore.NewIncompleteKey(ctx, entity.KindNameProblem, nil), problem)
	log.Infof(ctx, "problem %v (%s) saved", key.IntID(), key.Encode())
	return nil
}

func (pg *problemGenerator) uploadImage(ctx context.Context, state *shogi.State, highlight *shogi.Position) (string, error) {
	bucketName := pg.config.Host
	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	objectName := state.Hash()
	if highlight != nil {
		objectName += fmt.Sprintf("-%d%d", highlight.File, highlight.Rank)
	}
	objectName += ".png"
	w := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	w.ACL = []storage.ACLRule{
		{
			Entity: storage.AllUsers,
			Role:   storage.RoleReader,
		},
	}
	w.ContentType = "image/png"
	img, err := image.Generate(state, &image.StyleOptions{
		Board:     image.BoardStripe,
		Piece:     image.PieceDirty,
		HighLight: highlight,
	})
	if err != nil {
		return "", err
	}
	if err := png.Encode(w, img); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return strings.Join([]string{
		"https://storage.googleapis.com", bucketName, objectName,
	}, "/"), nil
}

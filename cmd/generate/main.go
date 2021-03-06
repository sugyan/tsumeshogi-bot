package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"image/png"
	"io"
	"log"
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
	"google.golang.org/appengine/remote_api"
)

type problemGenerator struct {
	config *config.Config
}

func main() {
	go func() {
		time.Sleep(time.Minute)
		log.Fatal("timeout")
	}()

	config, err := config.LoadConfig("app/config.toml")
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}
	ctx, err := remote_api.NewRemoteContext(config.Host, client)
	if err != nil {
		log.Fatal(err)
	}

	for _, problemType := range []generator.Problem{
		generator.Type1,
		generator.Type3,
		generator.Type5,
	} {
		if err := pg.generateProblem(ctx, problemType); err != nil {
			log.Printf("generate error %d: %v", problemType.Steps(), err)
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
		return pg.deleteLowScore(ctx, problemType)
	}
	log.Printf("type %d: %v", problemType.Steps(), count)

	// generate problem
	q, score := generator.Generate(problemType)
	a := solver.Solve(q)
	record := &record.Record{
		State: q,
		Moves: a,
	}

	// generate image
	var qImage, aImage string
	{
		var b io.Reader
		s := q.Clone()
		b, err = generatePNG(s, nil)
		if err != nil {
			return err
		}
		qImage, err = pg.uploadImage(ctx, b, "png")
		if err != nil {
			return err
		}
		for _, m := range a {
			s.Apply(m)
		}
		b, err = generatePNG(s, &a[len(a)-1].Dst)
		if err != nil {
			return err
		}
		aImage, err = pg.uploadImage(ctx, b, "png")
		if err != nil {
			return err
		}
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
	log.Printf("problem %v (%s) saved", key.IntID(), key.Encode())
	return nil
}

func (pg *problemGenerator) uploadImage(ctx context.Context, r io.Reader, ext string) (string, error) {
	bucketName := pg.config.Host
	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	objectName, err := randomHex(20)
	if err != nil {
		return "", err
	}
	objectName += "." + ext
	w := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	w.ACL = []storage.ACLRule{
		{
			Entity: storage.AllUsers,
			Role:   storage.RoleReader,
		},
	}
	w.ContentType = "image/" + ext
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(w, r); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return strings.Join([]string{
		"https://storage.googleapis.com", bucketName, objectName,
	}, "/"), nil
}

func (pg *problemGenerator) deleteLowScore(ctx context.Context, problemType generator.Problem) error {
	iter := datastore.NewQuery(entity.KindNameProblem).
		Filter("type = ", problemType.Steps()).
		Filter("used = ", false).
		Order("score").
		Run(ctx)
	for i := 0; i < int(entity.ProblemStockCount*0.1); i++ {
		var p entity.Problem
		key, err := iter.Next(&p)
		if err == datastore.Done {
			break
		}
		if err != nil {
			return err
		}
		log.Printf("delete %v", key.IntID())
		if err := p.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func generatePNG(state *shogi.State, highlight *shogi.Position) (io.Reader, error) {
	img, err := image.Generate(state, &image.StyleOptions{
		Board:     image.BoardStripe,
		Piece:     image.PieceDirty,
		HighLight: highlight,
	})
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}
	return buf, nil
}

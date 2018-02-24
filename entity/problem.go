package entity

import (
	"context"
	"log"
	"net/url"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/appengine/datastore"
)

// constant values
const (
	KindNameProblem   = "Problem"
	ProblemStockCount = 100
)

// Problem type
type Problem struct {
	CSA       string    `datastore:"csa,noindex"`
	Type      int       `datastore:"type"`
	Used      bool      `datastore:"used"`
	QImage    string    `datastore:"q_image,noindex"`
	AImage    string    `datastore:"a_image,noindex"`
	Score     int       `datastore:"score"`
	CreatedAt time.Time `datastore:"created_at"`
	UpdatedAt time.Time `datastore:"updated_at"`
}

// Delete method
func (p *Problem) Delete(ctx context.Context, key *datastore.Key) error {
	for _, imageURL := range []string{p.QImage, p.AImage} {
		if err := deleteImage(ctx, imageURL); err != nil {
			if err == storage.ErrObjectNotExist {
				log.Printf("%v: %v", imageURL, err.Error())
			} else {
				return err
			}
		}
	}
	return datastore.Delete(ctx, key)
}

func deleteImage(ctx context.Context, imageURL string) error {
	u, err := url.ParseRequestURI(imageURL)
	if err != nil {
		return err
	}
	d, objectName := path.Split(u.Path)
	bucketName := strings.Trim(d, "/")

	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Bucket(bucketName).Object(objectName).Delete(ctx)
}

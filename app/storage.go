package app

import (
	"context"
	"image/png"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/sugyan/shogi"
	"github.com/sugyan/shogi/util/image"
)

func (s *server) uploadImages(ctx context.Context, states []*shogi.State) ([]string, error) {
	bucketName := s.config.Host
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	results := []string{}
	for _, state := range states {
		objectName := state.Hash()
		w := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
		w.ACL = []storage.ACLRule{
			{
				Entity: storage.AllUsers,
				Role:   storage.RoleReader,
			},
		}
		w.ContentType = "image/png"
		img, err := image.Generate(state, &image.StyleOptions{
			Board: image.BoardStripe,
			Piece: image.PieceDirty,
		})
		if err != nil {
			return nil, err
		}
		if err := png.Encode(w, img); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		URL := strings.Join([]string{
			"https://storage.googleapis.com", bucketName, objectName,
		}, "/")
		results = append(results, URL)
	}
	return results, nil
}

package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"net/http"
	"net/url"

	"github.com/ChimeraCoder/anaconda"
	"github.com/sugyan/shogi/format/csa"
	"github.com/sugyan/shogi/logic/problem/generator"
	"github.com/sugyan/shogi/util/image"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

func (s *server) tweetHandler(w http.ResponseWriter, r *http.Request) {
	// cron request only
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	ctx := appengine.NewContext(r)
	log.Infof(ctx, "tweet...")
	if err := s.tweetProblem(ctx); err != nil {
		log.Errorf(ctx, "failed to tweet: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (s *server) tweetProblem(ctx context.Context) error {
	api := anaconda.NewTwitterApi(s.config.TwitterBot.AccessToken, s.config.TwitterBot.AccessTokenSecret)
	api.HttpClient.Transport = &urlfetch.Transport{Context: ctx}

	problem, encodedKey, err := s.fetchProblem(ctx, generator.ProblemType3)
	if err != nil {
		return err
	}
	record, err := csa.Parse(bytes.NewBufferString(problem.CSA))
	if err != nil {
		return err
	}
	img, err := image.Generate(record.State, nil)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer([]byte{})
	// Error when sending PNG image...
	if err := jpeg.Encode(base64.NewEncoder(base64.RawStdEncoding, buf), img, &jpeg.Options{
		Quality: 100,
	}); err != nil {
		return err
	}
	media, err := api.UploadMedia(buf.String())
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Add("media_ids", media.MediaIDString)
	URL, err := url.Parse("https://" + s.config.Host + "/answer/" + encodedKey)
	if err != nil {
		return err
	}
	status := fmt.Sprintf("%d手詰の問題です！\n正解はこちら → %s", problem.Type, URL.String())
	tweet, err := api.PostTweet(status, params)
	if err != nil {
		return err
	}
	log.Infof(ctx, "tweeted: %v", tweet.IdStr)
	return nil
}

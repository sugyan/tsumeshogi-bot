package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/sugyan/shogi/format/csa"
	"github.com/sugyan/shogi/logic/problem/generator"
	"github.com/sugyan/shogi/util/image"
	"github.com/sugyan/tsumeshogi_bot/config"
	"github.com/sugyan/tsumeshogi_bot/entity"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
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
	http.HandleFunc("/callback", server.botHandler)
	http.HandleFunc("/generate", server.generateHandler)
	http.HandleFunc("/tweet", server.tweetHandler)
}

func (s *server) botHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	bot, err := linebot.New(
		s.config.LineBot.ChannelSecret, s.config.LineBot.ChannelAccessToken,
		linebot.WithHTTPClient(urlfetch.Client(ctx)),
	)
	if err != nil {
		log.Errorf(ctx, "error: %v", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	events, err := bot.ParseRequest(r)
	if err != nil {
		log.Errorf(ctx, "failed to parse request: %v", err.Error())
		if err == linebot.ErrInvalidSignature {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}
	for _, event := range events {
		log.Infof(ctx, "event: %v", event)
		switch event.Type {
		case linebot.EventTypeMessage:
			if message, ok := event.Message.(*linebot.TextMessage); ok {
				var (
					problemType  generator.Problem
					replyMessage linebot.Message
				)
				switch {
				case strings.HasPrefix(message.Text, "1手詰"):
					problemType = generator.ProblemType1
				case strings.HasPrefix(message.Text, "3手詰"):
					problemType = generator.ProblemType3
				}
				if problemType == nil {
					return
				}
				problemEntity, err := fetchProblem(ctx, problemType)
				if err != nil {
					log.Errorf(ctx, "failed to fetch problem: %v", err)
					return
				}
				problem, err := csa.Parse(bytes.NewBufferString(problemEntity.State))
				if err != nil {
					log.Errorf(ctx, "failed to parse problem string: %v", err)
					return
				}
				path := strings.Replace(csa.InitialState2(problem.State), "\n", "/", -1)
				imageURL := fmt.Sprintf("https://shogi-img.appspot.com/%s/simple.png", path)
				text := fmt.Sprintf("%d手詰の問題です！", problemType.Steps())
				replyMessage = linebot.NewTemplateMessage(
					text+" LINEアプリでご覧ください",
					linebot.NewButtonsTemplate(
						imageURL, "", text,
						linebot.NewURITemplateAction("画像URL", imageURL),
						linebot.NewPostbackTemplateAction(
							"正解を見る",
							fmt.Sprintf("正解は…\n%s です！", strings.Join(problemEntity.Answer, " ")),
							"",
						),
					),
				)
				_, err = bot.ReplyMessage(event.ReplyToken, replyMessage).WithContext(ctx).Do()
				if err != nil {
					log.Errorf(ctx, "failed to reply message: %v", err.Error())
				}
			}
		case linebot.EventTypePostback:
			replyMessage := linebot.NewTextMessage(event.Postback.Data)
			_, err := bot.ReplyMessage(event.ReplyToken, replyMessage).WithContext(ctx).Do()
			if err != nil {
				log.Errorf(ctx, "failed to reply message: %v", err.Error())
			}
		}
	}
}

func (s *server) tweetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Appengine-Cron") != "true" {
		return
	}
	// TODO: check cron header
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

	problemEntity, err := fetchProblem(ctx, generator.ProblemType3)
	if err != nil {
		return err
	}
	problem, err := csa.Parse(bytes.NewBufferString(problemEntity.State))
	if err != nil {
		return err
	}
	img, err := image.Generate(problem.State, &image.StyleOptions{
		Board: image.BoardStripe,
		Piece: image.PieceDirty,
	})
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer([]byte{})
	// Error when sending PNG image...
	if err := jpeg.Encode(base64.NewEncoder(base64.RawStdEncoding, buf), img, &jpeg.Options{
		Quality: 90,
	}); err != nil {
		return err
	}
	media, err := api.UploadMedia(buf.String())
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Add("media_ids", media.MediaIDString)
	status := fmt.Sprintf("%d手詰の問題です！", problemEntity.Type)
	tweet, err := api.PostTweet(status, params)
	if err != nil {
		return err
	}
	log.Infof(ctx, "tweeted: %v", tweet.IdStr)
	return nil
}

func fetchProblem(ctx context.Context, problemType generator.Problem) (*entity.Problem, error) {
	query := datastore.NewQuery(entity.KindNameProblem).
		Filter("type = ", problemType.Steps())
	iter := query.
		Filter("used = ", false).
		Run(ctx)
	var problem entity.Problem
	key, err := iter.Next(&problem)
	if err != nil {
		if err != datastore.Done {
			return nil, err
		}
		count, err := query.Count(ctx)
		if err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, datastore.Done
		}
		_, err = query.Offset(rand.Intn(count)).Run(ctx).Next(&problem)
		if err != nil {
			return nil, err
		}
	} else {
		problem.Used = true
		problem.UpdatedAt = time.Now()
		_, err = datastore.Put(ctx, key, &problem)
		if err != nil {
			return nil, err
		}
	}
	return &problem, nil
}

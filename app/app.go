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

	"github.com/BurntSushi/toml"
	"github.com/ChimeraCoder/anaconda"
	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/sugyan/shogi/format/csa"
	"github.com/sugyan/shogi/logic/problem/generator"
	"github.com/sugyan/shogi/logic/problem/solver"
	"github.com/sugyan/shogi/util/image"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

type config struct {
	LineBot struct {
		ChannelSecret      string `toml:"channel_secret"`
		ChannelAccessToken string `toml:"channel_access_token"`
	} `toml:"line_bot"`
	TwitterBot struct {
		ConsumerKey       string `toml:"consumer_key"`
		ConsumerSecret    string `toml:"consumer_secret"`
		AccessToken       string `toml:"access_token"`
		AccessTokenSecret string `toml:"access_token_secret"`
	} `toml:"twitter_bot"`
}

type server struct {
	config config
}

func init() {
	var config config
	_, err := toml.DecodeFile("config.toml", &config)
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
				path := strings.Replace(csa.InitialState2(problem), "\n", "/", -1)
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

func (s *server) generateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Infof(ctx, "generate...")
	for _, problemType := range []generator.Problem{
		generator.ProblemType1,
		generator.ProblemType3,
	} {
		if err := generateAndSave(ctx, problemType); err != nil {
			log.Errorf(ctx, "failed to generate: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
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
	img, err := image.Generate(problem, &image.StyleOptions{
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

// constant values
const (
	KindNameProblem = "Problem"
	StockCount      = 50
)

type problemEntity struct {
	State  string   `datastore:"state,noindex"`
	Type   int      `datastore:"type"`
	Used   bool     `datastore:"used"`
	Answer []string `datastore:"answer,noindex"`
}

func generateAndSave(ctx context.Context, problemType generator.Problem) error {
	count, err := datastore.NewQuery(KindNameProblem).
		Filter("type = ", problemType.Steps()).
		Filter("used = ", false).
		Count(ctx)
	if err != nil {
		return err
	}
	log.Infof(ctx, "type: %v, count: %v", problemType.Steps(), count)
	if count >= StockCount {
		return nil
	}
	p := generator.Generate(problemType)
	answer, err := solver.Solve(p)
	if err != nil {
		return err
	}
	entity := &problemEntity{
		State:  csa.InitialState2(p),
		Type:   len(answer),
		Answer: answer,
		Used:   false,
	}
	key, err := datastore.Put(ctx, datastore.NewIncompleteKey(ctx, KindNameProblem, nil), entity)
	if err != nil {
		return err
	}
	log.Infof(ctx, "key: %v", key)
	return nil
}

func fetchProblem(ctx context.Context, problemType generator.Problem) (*problemEntity, error) {
	query := datastore.NewQuery(KindNameProblem).
		Filter("type = ", problemType.Steps())
	iter := query.
		Filter("used = ", false).
		Run(ctx)
	var entity problemEntity
	key, err := iter.Next(&entity)
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
		_, err = query.Offset(rand.Intn(count)).Run(ctx).Next(&entity)
		if err != nil {
			return nil, err
		}
	} else {
		entity.Used = true
		_, err = datastore.Put(ctx, key, &entity)
		if err != nil {
			return nil, err
		}
	}
	return &entity, nil
}

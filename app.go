package app

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/sugyan/shogi/format/csa"
	"github.com/sugyan/shogi/logic/problem/generator"
	"github.com/sugyan/shogi/logic/problem/solver"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

type config struct {
	ChannelSecret      string `toml:"channel_secret"`
	ChannelAccessToken string `toml:"channel_access_token"`
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
	server := &server{
		config: config,
	}
	http.HandleFunc("/", server.handler)
}

func (s *server) handler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	bot, err := linebot.New(
		s.config.ChannelSecret, s.config.ChannelAccessToken,
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
			if _, ok := event.Message.(*linebot.TextMessage); ok {
				problem := generator.Generate()
				answerString := ""
				for _, move := range solver.Solve(problem) {
					ms, _ := problem.MoveString(move)
					answerString += ms
				}
				path := strings.Replace(csa.InitialState2(problem), "\n", "/", -1)
				imageURL := fmt.Sprintf("https://shogi-img.appspot.com/%s/1.png", path)
				replyMessage := linebot.NewTemplateMessage(
					"this is template message. Please see in LINE app.",
					linebot.NewButtonsTemplate(
						imageURL, "", "1手詰の問題です！",
						linebot.NewURITemplateAction("画像URL", imageURL),
						linebot.NewPostbackTemplateAction("正解を見る", fmt.Sprintf("正解は %s です！", answerString), ""),
					),
				)
				_, err := bot.ReplyMessage(event.ReplyToken, replyMessage).WithContext(ctx).Do()
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

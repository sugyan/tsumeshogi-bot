package app

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/sugyan/shogi/format/csa"
	"github.com/sugyan/shogi/logic/problem/generator"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

func (s *server) callbackHandler(w http.ResponseWriter, r *http.Request) {
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
		if err := s.handleBotEvent(ctx, bot, event); err != nil {
			log.Errorf(ctx, "failed to handle event: %v", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

func (s *server) handleBotEvent(ctx context.Context, bot *linebot.Client, event linebot.Event) error {
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
				return nil
			}
			problem, encodedKey, err := fetchProblem(ctx, problemType)
			if err != nil {
				return err
			}
			record, err := csa.Parse(bytes.NewBufferString(problem.CSA))
			if err != nil {
				return err
			}
			path := strings.Replace(csa.InitialState2(record.State), "\n", "/", -1)
			imageURL := fmt.Sprintf("https://shogi-img.appspot.com/%s/simple.png", path)
			text := fmt.Sprintf("%d手詰の問題です！", problemType.Steps())
			replyMessage = linebot.NewTemplateMessage(
				text+" LINEアプリでご覧ください",
				linebot.NewButtonsTemplate(
					imageURL, "", text,
					linebot.NewURITemplateAction("画像URL", imageURL),
					linebot.NewPostbackTemplateAction(
						"正解を見る",
						encodedKey,
						"",
					),
				),
			)
			_, err = bot.ReplyMessage(event.ReplyToken, replyMessage).WithContext(ctx).Do()
			if err != nil {
				return err
			}
		}
	case linebot.EventTypePostback:
		problem, err := getProblem(ctx, event.Postback.Data)
		if err != nil {
			return err
		}
		record, err := csa.Parse(bytes.NewBufferString(problem.CSA))
		if err != nil {
			return err
		}
		answer, err := record.State.MoveStrings(record.Moves)
		if err != nil {
			return err
		}
		replyMessage := linebot.NewTextMessage(fmt.Sprintf("正解は…\n%s です！", strings.Join(answer, " ")))
		if _, err = bot.ReplyMessage(event.ReplyToken, replyMessage).WithContext(ctx).Do(); err != nil {
			return err
		}
	}
	return nil
}

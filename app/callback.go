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

func (s *server) handleBotEvent(ctx context.Context, bot *linebot.Client, event *linebot.Event) error {
	switch event.Type {
	case linebot.EventTypeMessage:
		if message, ok := event.Message.(*linebot.TextMessage); ok {
			var (
				problemType  generator.Problem
				replyMessage linebot.Message
			)
			switch {
			case strings.HasPrefix(message.Text, "1手詰"):
				problemType = generator.Type1
			case strings.HasPrefix(message.Text, "3手詰"):
				problemType = generator.Type3
			case strings.HasPrefix(message.Text, "5手詰"):
				problemType = generator.Type5
			}
			if problemType == nil {
				return nil
			}
			problem, key, err := s.fetchProblem(ctx, problemType)
			if err != nil {
				return err
			}
			text := fmt.Sprintf("%d手詰の問題です！", problemType.Steps())
			replyMessage = linebot.NewTemplateMessage(
				text+" LINEアプリでご覧ください",
				linebot.NewButtonsTemplate(
					problem.QImage, "", text,
					linebot.NewURITemplateAction("画像URL", problem.QImage),
					linebot.NewPostbackTemplateAction(
						"正解を見る",
						key.Encode(),
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
		var replyMessage linebot.Message
		s := strings.Split(event.Postback.Data, ":")
		encoded := s[0]
		problem, _, err := getProblem(ctx, encoded)
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
		text := fmt.Sprintf("正解は…\n%s です！", strings.Join(answer, " "))
		replyMessage = linebot.NewTemplateMessage(
			text,
			linebot.NewButtonsTemplate(
				problem.AImage, "", text,
				linebot.NewMessageTemplateAction("もう1問！", fmt.Sprintf("%d手詰", problem.Type)),
			),
		)
		if _, err := bot.ReplyMessage(event.ReplyToken, replyMessage).WithContext(ctx).Do(); err != nil {
			return err
		}
	}
	return nil
}

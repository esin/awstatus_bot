// Package p contains an HTTP Cloud Function.
package p

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type telegramUserStruct struct {
	ID       int64
	Regions  []string
	Services []string
}

type serviceListStruct struct {
	ID                string
	URL               string
	Service           string // AWS service
	ServicePrettyName string // AWS service pretty name
	Region            string // AWS service region, can be empty
	RegionPrettyName  string
	RegionEmojiFlag   string
}

var client *firestore.Client
var ctx context.Context

func checkIsUserStarted(userID int64) bool {

	// Crazy function - select everything from document
	dsnap, err := client.Collection("notifications").Doc("telegram").Get(ctx)
	if err != nil {
		log.Fatalln("Error reveiving RSS URL list")
	}

	telegramRegisteredUsers := dsnap.Data()
	userIDStr := strconv.FormatInt(userID, 10)

	if telegramRegisteredUsers[userIDStr] != nil {
		// TODO
		return true
	}

	telegramUser := make(map[string]telegramUserStruct)

	telegramUser[strconv.FormatInt(userID, 10)] = telegramUserStruct{ID: userID, Regions: []string{}, Services: []string{}}
	_, err = client.Collection("notifications").Doc("telegram").Set(ctx, telegramUser, firestore.MergeAll)

	if err != nil {
		log.Fatalf("Failed adding: %v", err)
	}
	return false
}

func TelegramDash(w http.ResponseWriter, r *http.Request) {

	telegramBotToken := os.Getenv("TELEGRAM_TOKEN")
	bot, err := tgbotapi.NewBotAPI(telegramBotToken)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true

	update := tgbotapi.Update{}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Fail to parse JSON body", http.StatusBadRequest)
		return
	}

	ctx = context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: "awstatus"})
	if err != nil {
		log.Fatalln(err)
	}
	client, err = app.Firestore(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	defer client.Close()

	if update.Message != nil {
		if update.Message.IsCommand() {
			if update.Message.Command() == "start" {
				msgToSend := fmt.Sprintf("Hey %s! I'm just bot, with some extra features (soon) and I can send you notification about changing status on https://status.aws.amazon.com\n\n", update.Message.From.FirstName)
				msgToSend = msgToSend + fmt.Sprintf("Well, now I'll notify you about issues in any region and any service")

				if checkIsUserStarted(int64(update.Message.From.ID)) {
					msgToSend = fmt.Sprintf("Looks like we've already met. Anyway, I'm glad to see you\n")
					msgToSend = msgToSend + fmt.Sprintf("If you forgot - I'm bot, which will notify you about issues on https://status.aws.amazon.com\n")
				}

				msg := tgbotapi.NewMessage(int64(update.Message.From.ID), msgToSend)
				msg.ParseMode = "markdown"
				msg.DisableWebPagePreview = true
				_, err = bot.Send(msg)
				if err != nil {
					log.Fatalln("Telegram send err: ", err)
				}
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}

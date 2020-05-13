package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/mitchellh/mapstructure"
)

type FirestoreEvent struct {
	OldValue FirestoreValue `json:"oldValue"`
	Value    FirestoreValue `json:"value"`
}

type FirestoreValue struct {
	CreateTime time.Time               `json:"createTime"`
	Name       string                  `json:"name"`
	UpdateTime time.Time               `json:"updateTime"`
	Fields     awsStatusServicesStruct `json:"fields"`
}

type awsStatusServicesStruct struct {
	LastUpdate  StringValue `json:"LastUpdate"`
	GUID        StringValue `json:"GUID"`
	Description StringValue `json:"Description"`
	ID          StringValue `json:"ID"`
}

type StringValue struct {
	StringValue string `json:"stringValue"`
}

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

func FirestoreFunction(ctx context.Context, e FirestoreEvent) error {
	telegramBotToken := os.Getenv("TELEGRAM_TOKEN")
	if telegramBotToken == "" {
		log.Fatalln("Env variable TELEGRAM_TOKEN not found")
	}

	bot, err := tgbotapi.NewBotAPI(telegramBotToken)
	if err != nil {
		log.Panic(err)
	}

	conf := &firebase.Config{ProjectID: "awstatus"}
	app, err := firebase.NewApp(ctx, conf)
	if err != nil {
		log.Fatalln(err)
	}

	client, err := app.Firestore(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	defer client.Close()

	serviceLookupSnap, err := client.Collection("awsstatus").Doc("awsstatusurls").Get(ctx)
	if err != nil {
		log.Fatalln("Error receiving RSS URL list")
	}

	serviceLookup := serviceLookupSnap.Data()
	AWSServiceLabel := ""
	AWSServiceRegion := ""
	TGTags := ""
	for serviceID, serviceData := range serviceLookup {

		c := serviceListStruct{}
		err = mapstructure.Decode(serviceData, &c)
		if err != nil {
			log.Println("mapstructure.Decode err: ", err)
			continue
		}

		if serviceID == e.Value.Fields.ID.StringValue {
			AWSServiceLabel = c.ServicePrettyName
			TGTags = TGTags + fmt.Sprintf("#%s ", strings.ReplaceAll(c.Service, "-", "")) // Tags can't contain dash :(

			if c.RegionPrettyName != "" {
				AWSServiceRegion = fmt.Sprintf("%s%s", c.RegionEmojiFlag, c.RegionPrettyName)
				TGTags = TGTags + fmt.Sprintf("#%s ", strings.ReplaceAll(c.Region, "-", ""))
			}
			break
		}
	}

	dsnap, err := client.Collection("notifications").Doc("telegram").Get(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	telegramUserNotifications := dsnap.Data()

	for _, telegramUser := range telegramUserNotifications {

		c := telegramUserStruct{}
		err = mapstructure.Decode(telegramUser, &c)
		if err != nil {
			log.Println("mapstructure.Decode err: ", err)
			continue
		}

		//  Check region, service etc and notify

		guid := e.Value.Fields.GUID.StringValue
		guid = strings.ReplaceAll(guid, "_", "\\_")
		msgToSend := fmt.Sprintf("Service: *%s*\nRegion: *%s*\nURL: [%s](%s) \n\n%s\n\n%s", AWSServiceLabel, AWSServiceRegion, e.Value.Fields.GUID.StringValue, guid, e.Value.Fields.Description.StringValue, TGTags)

		msg := tgbotapi.NewMessage(c.ID, msgToSend)
		msg.ParseMode = "markdown"
		msg.DisableWebPagePreview = true
		_, err = bot.Send(msg)
		if err != nil {
			log.Println("Telegram send err: ", err)
		}
	}

	return nil
}

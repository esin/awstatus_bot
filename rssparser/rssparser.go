// Package p contains an HTTP Cloud Function.
package p

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"github.com/mitchellh/mapstructure"
	"github.com/mmcdole/gofeed"
	"google.golang.org/api/iterator"
)

type serviceListStruct struct {
	ID      string
	URL     string
	Service string // AWS service
	Region  string // AWS service region, can be empty
}

type fireStoreListStruct struct {
	URLs []serviceListStruct
}

type awsStatusServicesStruct struct {
	LastUpdate  string
	GUID        string
	Description string
	ID          string // AWS service name with region if present
}

func worker(urlChan chan string, resultChan chan string, id int) {
	for {
		rssUrl := <-urlChan
		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(rssUrl)
		if err != nil {
			resultChan <- fmt.Sprintf("Error with RSS %s: %s\n", rssUrl, err.Error())
			continue
		}
		log.Printf("Start parsing: %s", rssUrl)
		if feed.Items != nil {
			if len(feed.Items) > 0 {

				for _, feedItem := range feed.Items {
					rssFileName := filepath.Base(rssUrl)
					serviceName := strings.TrimSuffix(rssFileName, filepath.Ext(rssFileName))
					mu.Lock()
					awsStatusServices = append(awsStatusServices, awsStatusServicesStruct{ID: serviceName, Description: feedItem.Description, GUID: feedItem.GUID, LastUpdate: feedItem.Published})
					mu.Unlock()

					resultChan <- fmt.Sprintf("Done parsing: %s", rssUrl)

					break
				}
				continue
			}
			resultChan <- fmt.Sprintf("Items not found in %s", rssUrl)
		}
	}
}

func generator(url string, urlChan chan string) {
	urlChan <- url
}

var client *firestore.Client
var ctx context.Context
var mu = &sync.Mutex{}
var awsStatusServices []awsStatusServicesStruct

func AWSstatuschecker(w http.ResponseWriter, r *http.Request) {

	// Due that this works as a Google Function in GCP, it doesn't need auth info,
	// so you you want work with Firebase you should correctly init GCP creds
	// and begin with uncommenting line below
	// os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "awsstatus.json")

	ctx = context.Background()
	conf := &firebase.Config{ProjectID: "awstatus"}
	app, err := firebase.NewApp(ctx, conf)
	if err != nil {
		log.Fatalln(err)
	}

	client, err = app.Firestore(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	defer client.Close()

	dataStoreSnapshot, err := client.Collection("awsstatus").Doc("awsstatusurls").Get(ctx)
	if err != nil {
		log.Fatalln("Error receiving RSS URL list")
	}

	fsl := dataStoreSnapshot.Data()

	if len(fsl) == 0 {
		log.Fatalln("RSS URL list is empty")
	}

	urlChan := make(chan string)
	resultChan := make(chan string)

	for i := 0; i < 30; i++ {
		go worker(urlChan, resultChan, i)
	}

	for _, rssUrl := range fsl {

		c := serviceListStruct{}
		err = mapstructure.Decode(rssUrl, &c)
		if err != nil {
			log.Println("mapstructure.Decode err: ", err)
			continue
		}
		go generator(c.URL, urlChan)
	}

	for i := 0; i < len(fsl); i++ {
		log.Printf("%s\n", <-resultChan)
	}

	currentDBSnapshot := make(map[string]awsStatusServicesStruct)
	iter := client.Collection("awsservices").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			continue
		}

		var awsServiceItem awsStatusServicesStruct
		doc.DataTo(&awsServiceItem)

		currentDBSnapshot[awsServiceItem.ID] = awsServiceItem
	}

	updatedServices := 0
	for _, awsService := range awsStatusServices {

		newDate, err := time.Parse(time.RFC1123, awsService.LastUpdate)
		if err != nil {
			log.Printf("Failed parsing time newDate: %v", err)
			continue
		}

		var currDate time.Time
		if currentDBSnapshot[awsService.ID].LastUpdate != "" {
			currDate, err = time.Parse(time.RFC1123, currentDBSnapshot[awsService.ID].LastUpdate)
			if err != nil {
				log.Printf("Failed parsing time currDate: %v", err)
				continue
			}
		}

		// If new last update is newer than saved in database -> add to database
		if newDate.After(currDate) || currentDBSnapshot[awsService.ID].LastUpdate == "" {
			_, err = client.Collection("awsservices").Doc(awsService.ID).Set(ctx, map[string]interface{}{
				"LastUpdate":  awsService.LastUpdate,
				"GUID":        awsService.GUID,
				"Description": awsService.Description,
				"ID":          awsService.ID,
			})

			if err != nil {
				log.Printf("Failed adding: %v", err)
				continue
			}
			updatedServices++
		}
	}

	log.Printf("Updated %d AWS services", updatedServices)
}

// Package p contains an HTTP Cloud Function.
package p

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"

	firebase "firebase.google.com/go"
)

type serviceListStruct struct {
	ID                string
	URL               string
	Service           string // AWS service
	ServicePrettyName string // AWS service pretty name
	Region            string // AWS service region, can be empty
	RegionPrettyName  string
	RegionEmojiFlag   string
}

type regionLookupTableStruct struct {
	PrettyName string
	EmojiFlag  string
}

var regionLookup = map[string]regionLookupTableStruct{
	"us-east-1":   {PrettyName: "US East (N. Virginia)", EmojiFlag: "🇺🇸"},
	"us-east-2":   {PrettyName: "US East (Ohio)", EmojiFlag: "🇺🇸"},
	"us-west-1":   {PrettyName: "US West (N. California)", EmojiFlag: "🇺🇸"},
	"us-west-2":   {PrettyName: "US West (Oregon)", EmojiFlag: "🇺🇸"},
	"us-standard": {PrettyName: "US East (N. Virginia)", EmojiFlag: "🇺🇸"}, // For s3 N. Virginia

	"us-gov-east-1": {PrettyName: "AWS GovCloud (US-East)", EmojiFlag: "🇺🇸"},
	"us-gov-west-1": {PrettyName: "AWS GovCloud (US)", EmojiFlag: "🇺🇸"},

	"af-south-1": {PrettyName: "Africa (Cape Town)", EmojiFlag: "🇿🇦"},

	"ap-east-1":      {PrettyName: "Asia Pacific (Hong Kong) ", EmojiFlag: "🇭🇰"},
	"ap-south-1":     {PrettyName: "Asia Pacific (Mumbai)", EmojiFlag: "🇮🇳"},
	"ap-southeast-1": {PrettyName: "Asia Pacific (Singapore)", EmojiFlag: "🇸🇬"},
	"ap-southeast-2": {PrettyName: "Asia Pacific (Sydney) ", EmojiFlag: "🇦🇺"},
	"ap-northeast-1": {PrettyName: "Asia Pacific (Tokyo)", EmojiFlag: "🇯🇵"},
	"ap-northeast-2": {PrettyName: "Asia Pacific (Seoul)", EmojiFlag: "🇰🇷"},
	"ap-northeast-3": {PrettyName: "Asia Pacific (Osaka-Local)", EmojiFlag: "🇯🇵"},

	"ca-central-1": {PrettyName: "Canada (Central)", EmojiFlag: "🇨🇦"},

	"eu-central-1": {PrettyName: "Europe (Frankfurt)", EmojiFlag: "🇩🇪"},
	"eu-west-1":    {PrettyName: "Europe (Ireland)", EmojiFlag: "🇮🇪"},
	"eu-west-2":    {PrettyName: "Europe (London)", EmojiFlag: "🇬🇧"},
	"eu-west-3":    {PrettyName: "Europe (Paris)", EmojiFlag: "🇫🇷"},
	"eu-north-1":   {PrettyName: "Europe (Stockholm)", EmojiFlag: "🇸🇪"},
	"eu-south-1":   {PrettyName: "Italy (Milan)", EmojiFlag: "🇮🇹"},

	"me-south-1": {PrettyName: "Middle East (Bahrain)", EmojiFlag: "🇧🇭"},

	"cn-north-1":     {PrettyName: "China (Beijing)", EmojiFlag: "🇨🇳"},
	"cn-northwest-1": {PrettyName: "China (Ningxia)", EmojiFlag: "🇨🇳"},

	"sa-east-1": {PrettyName: "South America (São Paulo)", EmojiFlag: "🇧🇷"},

	"global": {PrettyName: "", EmojiFlag: "🌏"},
	"":       {PrettyName: "", EmojiFlag: "🌏"},

	"test-region-1": {PrettyName: "My test region 1", EmojiFlag: "👽"},
	"test-region-2": {PrettyName: "My test region 2", EmojiFlag: "🤪"},
}

var rawRSSListRegExp = `(?U)<td class="bb top pad8">(?P<srvname>[a-zA-Z0-9 ]+)( \([A-Za-z -\.]+\))?<\/td>[\s\S]*.*a href="(?P<srvurl>/rss/[a-zA-Z0-9-.]+\.rss)`
var serviceRSSUrlRegExp = `.*\/rss\/([a-z0-9A-Z]+|neptune-db)-?([a-z]+)?-?([a-z]+)?-?([0-9]+)?\.rss$`

func uniq(inArr []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range inArr {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func RSSListGenerator(w http.ResponseWriter, r *http.Request) {

	resp, err := http.Get("https://status.aws.amazon.com/")
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Fatalln(err)
	}

	rx := regexp.MustCompile(rawRSSListRegExp)

	// TODO use named groups

	foundUrls := rx.FindAllStringSubmatch(string(body), -1)

	parsedUrls := make(map[string]string, 0)

	// Creating map. Key - rssurl, Value - Service Name
	for _, foundService := range foundUrls {
		if len(foundService) > 1 {
			rssUrl := fmt.Sprintf("https://status.aws.amazon.com%s", foundService[2])
			if len(foundService) == 4 {
				rssUrl = fmt.Sprintf("https://status.aws.amazon.com%s", foundService[3])
			}

			parsedUrls[rssUrl] = foundService[1]
		}
	}

	rxSrv := regexp.MustCompile(serviceRSSUrlRegExp)

	serviceList := make(map[string]serviceListStruct, 0)
	for url, prettyName := range parsedUrls {

		matched := rxSrv.FindAllStringSubmatch(string(url), -1)
		if len(matched) == 0 {
			log.Printf("Regex didn't work for: %s", url)
			continue
		}

		service := matched[0][1]
		region := ""

		for i := 2; i < len(matched[0]); i++ {
			if len(matched[0][i]) > 0 {
				region = region + "-" + matched[0][i]
			}
		}

		serviceID := fmt.Sprintf("%s", service)

		if region != "" {
			region = region[1:]
			serviceID = fmt.Sprintf("%s-%s", service, region)
		}
		serviceList[serviceID] = serviceListStruct{ServicePrettyName: prettyName, ID: serviceID, URL: url, Service: service, Region: region, RegionEmojiFlag: regionLookup[region].EmojiFlag, RegionPrettyName: regionLookup[region].PrettyName}
	}

	// Found rss urls, ok let's go

	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: "awstatus"})
	if err != nil {
		log.Fatalln(err)
	}

	client, err := app.Firestore(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	defer client.Close()

	_, err = client.Collection("awsstatus").Doc("awsstatusurls").Set(ctx, serviceList)
	if err != nil {
		log.Fatalf("Failed adding: %v", err)
	}
}

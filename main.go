package main

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

type Item struct {
	TimeReceive  string
	ItemName     string
	ItemId       string
	ItemQuantity string
	TrackingId   string
	ShipAdd      string
}

type ItemsPerPage struct {
	Items         []Item
	NextPageToken string
}

const User = "me"

const PerPageNumber = 500

// const Label = "label:sephora-arrived"
const Label = "label:Số lượng hàng-The INKEY"

var HeaderCSV = []string{
	"Date & Time Received", "Name", "Item ID", "Quantity", "Tracking ID", "Ship To",
}

func getMailItems(srv *gmail.Service, mail []*gmail.Message) []Item {
	var items []Item
	for _, m := range mail {
		fmt.Printf("%v\n", m.Id)
		messageResponse, err := srv.Users.Messages.Get(User, string(m.Id)).Do()
		if err != nil {
			log.Fatalf("Get Messages Error: %v", err)
		}

		var timeReceive string
		for _, v := range messageResponse.Payload.Headers {
			if v.Name == "Date" {
				timeReceive = v.Value
			}
		}

		dataEncode := (messageResponse.Payload.Parts[1].Body.Data)
		data, err := base64.URLEncoding.DecodeString(dataEncode)
		if err != nil {
			log.Fatalf("DecodeString Error: %v", err)
		}

		z := html.NewTokenizer(strings.NewReader(string(data)))
		const NameStyle = "font-family:Helvetica;font-size:12px;font-weight:700;letter-spacing:0.25;line-height:18px;text-align:left;color:#0A0A0A;"
		const ItemIdStyle = "font-family:Helvetica;font-size:12px;font-weight:400;letter-spacing:0.25;line-height:18px;text-align:left;color:#000000;"
		const QuantityStyle = "font-family:Helvetica;font-size:12px;font-weight:400;letter-spacing:0.25;line-height:18px;text-align:center;color:#4D4D4D;"

		var trackingId string
		// var orderDate string
		var shipAdd string
		var itemIds []string
		var itemNames []string
		var itemQuantites []string

		end := false
		for {
			if end {
				break
			}
			tt := z.Next()
			switch tt {
			case html.ErrorToken:
				fmt.Println("End")
				end = true

			case html.StartTagToken:
				t := z.Token()
				if t.Data != "div" {
					continue
				}
				attrs := t.Attr
				for _, v := range attrs {
					if v.Key == "style" && v.Val == ItemIdStyle {
						z.Next()
						t := z.Token()
						itemId := strings.ReplaceAll(t.Data, "ITEM", "")
						itemId = strings.TrimSpace(itemId)
						itemIds = append(itemIds, itemId)
					}

					if v.Key == "style" && v.Val == NameStyle {
						z.Next()
						t := z.Token()
						// fmt.Printf("%T %v \n", t.Data, t.Data)
						itemNames = append(itemNames, t.Data)
					}

					if v.Key == "style" && v.Val == QuantityStyle {
						z.Next()
						t := z.Token()
						itemQuantites = append(itemQuantites, t.Data)
					}
				}

			case html.TextToken:
				t := z.Token()
				htmlText := strings.TrimSpace(t.Data)
				if htmlText == "" {
					continue
				}
				if htmlText == "TRACKING #:" {
					z.Next()
					z.Next()
					t = z.Token()
					trackingId = t.Data
					// fmt.Printf("trackingId %T %v \n", trackingId, trackingId)
					continue
				}
				// if strings.Contains(htmlText, "ORDER DATE:") {
				// 	orderDate = strings.ReplaceAll(htmlText, "ORDER DATE:", "")
				// 	orderDate = strings.TrimSpace(orderDate)
				// 	// fmt.Printf("orderDate %T %v \n", orderDate, orderDate)
				// 	continue
				// }
				if htmlText == "SHIP TO:" {
					z.Next()
					z.Next()
					z.Next()
					z.Next()
					t = z.Token()
					shipAdd = strings.TrimSpace(t.Data)
					z.Next()
					z.Next()
					t = z.Token()
					shipAdd += "\n" + strings.TrimSpace(t.Data)
					// fmt.Printf("shipAdd %T %v \n", shipAdd, shipAdd)
					continue
				}
			}
		}

		for i, _ := range itemIds {
			items = append(items, Item{
				TimeReceive:  timeReceive,
				ItemName:     itemNames[i],
				ItemId:       itemIds[i],
				ItemQuantity: itemQuantites[i],
				TrackingId:   trackingId,
				ShipAdd:      shipAdd,
			})
		}
		// fmt.Printf("%T %v \n", trackingId, trackingId)
		// fmt.Printf("%T %v \n", shipAdd, shipAdd)

		// fmt.Printf("%T %v \n", itemNames, itemNames)
		// fmt.Printf("%T %v \n", itemIds, itemIds)
		// fmt.Printf("%T %v \n", itemQuantites, itemQuantites)
	}
	return items
}

func exportCsv(items []Item) {
	csvFile, err := os.Create("items.csv")

	if err != nil {
		log.Fatalf("failed creating file: %s", err)
	}

	csvwriter := csv.NewWriter(csvFile)
	_ = csvwriter.Write(HeaderCSV)

	for _, item := range items {
		row := []string{
			item.TimeReceive,
			item.ItemName,
			item.ItemId,
			item.ItemQuantity,
			item.TrackingId,
			item.ShipAdd,
		}
		_ = csvwriter.Write(row)
	}
	csvwriter.Flush()
	csvFile.Close()
}

func getFirstMails(srv *gmail.Service) ItemsPerPage {
	var r *gmail.ListMessagesResponse
	var err error
	r, err = srv.Users.Messages.List(User).Q(Label).MaxResults(PerPageNumber).Do()

	return getMailPaginate(srv, r, err)
}

func getNextMails(srv *gmail.Service, nextPageToken string) ItemsPerPage {
	var r *gmail.ListMessagesResponse
	var err error
	r, err = srv.Users.Messages.List(User).MaxResults(PerPageNumber).Q(Label).PageToken(nextPageToken).Do()

	return getMailPaginate(srv, r, err)
}

func getMailPaginate(srv *gmail.Service, r *gmail.ListMessagesResponse, err error) ItemsPerPage {
	if err != nil {
		log.Fatalf("Unable to retrieve Messages: %v", err)
	}
	if len(r.Messages) == 0 {
		log.Fatalf("No Messages found.")
	}

	items := getMailItems(srv, r.Messages)

	return ItemsPerPage{
		Items:         items,
		NextPageToken: r.NextPageToken,
	}
}

func main() {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Gmail client: %v", err)
	}

	var items []Item
	firstMails := getFirstMails(srv)
	items = append(items, firstMails.Items...)
	nextPageToken := firstMails.NextPageToken

	for {
		if strings.TrimSpace(nextPageToken) == "" {
			break
		}
		nextMails := getNextMails(srv, nextPageToken)
		items = append(items, nextMails.Items...)
		nextPageToken = nextMails.NextPageToken
	}

	exportCsv(items)
}

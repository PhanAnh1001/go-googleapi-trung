package main

import (
	"context"
	"log"

	"google.golang.org/api/gmail/v1"
)

func main() {
	ctx := context.Background()
	gmailService, err := gmail.NewService(ctx, option.WithCredentialsFile("trung.json"))
	req := gmailService.Users.Messages.List("trunidojoan@gmail.com").Q("label:sephora-arrived")
	r, err := req.Do()
	log.Printf("Processing %v messages...\n", len(r.Messages))
}

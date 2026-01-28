//go:build track_example
// +build track_example

package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	sensorswave "github.com/sensorswave/sdk-go"
)

type exampleArgs struct {
	sourceToken string
	endpoint    string
	anonID      string
	loginID     string
}

func main() {
	args := parseArgs()

	client, err := sensorswave.New(
		sensorswave.Endpoint(args.endpoint),
		sensorswave.SourceToken(args.sourceToken),
	)
	if err != nil {
		log.Fatalf("create sensorswave client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("close client error: %v", err)
		}
	}()

	user := sensorswave.User{
		AnonID:  args.anonID,
		LoginID: args.loginID,
	}

	if err := runIdentify(client, user); err != nil {
		log.Fatalf("identify failed: %v", err)
	}
	if err := runTrackEvent(client, user); err != nil {
		log.Fatalf("track event failed: %v", err)
	}
	if err := runProfileSet(client, user); err != nil {
		log.Fatalf("profile set failed: %v", err)
	}

	log.Println("example done")
}

func parseArgs() exampleArgs {
	var args exampleArgs
	flag.StringVar(&args.sourceToken, "source-token", "", "project token used by SDK client")
	flag.StringVar(&args.endpoint, "endpoint", "https://example.sensorswave.com", "track endpoint base url")
	flag.StringVar(&args.anonID, "anon-id", "", "anonymous id")
	flag.StringVar(&args.loginID, "login-id", "", "login id")
	flag.Parse()

	if args.sourceToken == "" {
		log.Fatal("source-token is required")
	}

	if args.anonID == "" || args.loginID == "" {
		now := time.Now().UnixNano()
		if args.anonID == "" {
			args.anonID = fmt.Sprintf("anon-%d", now)
		}
		if args.loginID == "" {
			args.loginID = fmt.Sprintf("user-%d", now)
		}
	}

	return args
}

func runIdentify(client sensorswave.Client, user sensorswave.User) error {
	// Both AnonID and LoginID are required for Identify.
	return client.Identify(user)
}

func runTrackEvent(client sensorswave.Client, user sensorswave.User) error {
	return client.TrackEvent(user, "purchaseEvent", sensorswave.Properties{
		"product_id": "SKU-001",
		"price":      19.9,
		"currency":   "USD",
	})
}

func runProfileSet(client sensorswave.Client, user sensorswave.User) error {
	return client.ProfileSet(user, sensorswave.Properties{
		"name":        "Alice",
		"email":       "alice@example.com",
		"signup_date": time.Now().Format(time.RFC3339),
	})
}

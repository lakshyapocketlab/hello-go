// Command error-handling sends a text message and shows how to branch on the SDK's typed errors.
//
//	HELLO_API_KEY=hello_live_xxx go run ./examples/error-handling <phone_number_id> <to> "Hello"
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	hello "github.com/lakshyapocketlab/hello-go"
)

func main() {
	if len(os.Args) != 4 {
		log.Fatal("usage: error-handling <phone_number_id> <to> <text>")
	}
	client, err := hello.NewClient()
	if err != nil {
		log.Fatal(err) // hello.ErrMissingAPIKey when neither WithAPIKey nor HELLO_API_KEY is set
	}

	resp, err := client.Messages.SendText(context.Background(), os.Args[1], os.Args[2], os.Args[3])

	var apiErr *hello.APIError
	switch {
	case err == nil:
		fmt.Println("accepted, Hello ID:", resp.Messages[0].HelloID)
	case hello.IsAuthentication(err):
		log.Fatal("bad or disabled API key: ", err)
	case hello.IsRateLimit(err):
		log.Fatal("rate limited, retry later: ", err)
	case hello.IsNotFound(err):
		log.Fatal("not found, check the phone_number_id: ", err)
	case errors.As(err, &apiErr):
		// Hello's own code (e.g. VALIDATION_ERROR) is the stable thing to branch on; quote
		// FBTraceID when you contact support.
		fmt.Printf("API error: status=%d hello_code=%q meta_code=%d trace=%s message=%s\n",
			apiErr.Status, apiErr.HelloCode, apiErr.Code, apiErr.FBTraceID, apiErr.Message)
		os.Exit(1)
	default:
		log.Fatal("transport error: ", err) // network failure or timeout; retries already ran
	}
}

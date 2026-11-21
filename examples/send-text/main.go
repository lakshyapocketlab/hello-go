// Command send-text sends a WhatsApp text message.
//
//	HELLO_API_KEY=hello_live_xxx go run ./examples/send-text <phone_number_id> <to> "Hello"
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	hello "github.com/lakshyapocketlab/hello-go"
)

func main() {
	if len(os.Args) != 4 {
		log.Fatal("usage: send-text <phone_number_id> <to> <text>")
	}
	client, err := hello.NewClient() // reads HELLO_API_KEY
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Messages.SendText(context.Background(), os.Args[1], os.Args[2], os.Args[3])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("queued/accepted, Hello ID:", resp.Messages[0].HelloID)
}

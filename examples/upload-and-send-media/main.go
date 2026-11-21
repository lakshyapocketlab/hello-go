// Command upload-and-send-media uploads a file and sends it as a document.
//
//	HELLO_API_KEY=hello_live_xxx go run ./examples/upload-and-send-media <phone_number_id> <to> ./invoice.pdf application/pdf
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	hello "github.com/lakshyapocketlab/hello-go"
)

func main() {
	if len(os.Args) != 5 {
		log.Fatal("usage: upload-and-send-media <phone_number_id> <to> <file> <mime_type>")
	}
	phoneNumberID, to, path, mimeType := os.Args[1], os.Args[2], os.Args[3], os.Args[4]

	client, err := hello.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	ctx := context.Background()
	media, err := client.Media.Upload(ctx, phoneNumberID, f, filepath.Base(path), mimeType)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Messages.SendMedia(ctx, phoneNumberID, to, hello.MediaMessage{
		Type: hello.MediaDocument, ID: media.ID, Filename: filepath.Base(path),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("media", media.ID, "sent, Hello ID:", resp.Messages[0].HelloID)
}

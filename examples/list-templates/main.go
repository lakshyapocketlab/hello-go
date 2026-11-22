// Command list-templates prints every APPROVED template of a WhatsApp Business Account,
// following the pagination cursor until the last page.
//
//	HELLO_API_KEY=hello_live_xxx go run ./examples/list-templates <waba_id>
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	hello "github.com/lakshyapocketlab/hello-go"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: list-templates <waba_id>")
	}
	client, err := hello.NewClient() // reads HELLO_API_KEY
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	params := &hello.ListTemplatesParams{Status: "APPROVED", Limit: 50}
	for {
		page, err := client.Templates.List(ctx, os.Args[1], params)
		if err != nil {
			log.Fatal(err)
		}
		for _, t := range page.Data {
			fmt.Printf("%-30s %-8s %s\n", t.Name, t.Language, t.Category)
		}
		next := page.Paging.Cursors.After
		if next == "" {
			break
		}
		params.After = next
	}
}

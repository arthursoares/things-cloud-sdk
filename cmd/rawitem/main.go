package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
)

func main() {
	c := thingscloud.New(thingscloud.APIEndpoint, os.Getenv("THINGS_USERNAME"), os.Getenv("THINGS_PASSWORD"))
	if _, err := c.Verify(); err != nil {
		log.Fatalf("verify: %v", err)
	}
	history, err := c.OwnHistory()
	if err != nil {
		log.Fatalf("own history: %v", err)
	}
	if err := history.Sync(); err != nil {
		log.Fatalf("sync: %v", err)
	}

	target := "2MNjM5gT5hZt2sSEw4PvDb"

	startIndex := 0
	for {
		items, hasMore, err := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})
		if err != nil {
			log.Fatalf("read items: %v", err)
		}

		for _, item := range items {
			if item.UUID == target {
				fmt.Printf("UUID: %s\n", item.UUID)
				fmt.Printf("Kind: %s\n", item.Kind)
				fmt.Printf("Action (parsed): %d\n", item.Action)

				// Also print item as raw JSON
				raw, err := json.MarshalIndent(item, "", "  ")
				if err != nil {
					log.Fatalf("encode item %s: %v", item.UUID, err)
				}
				fmt.Printf("Raw item: %s\n", string(raw))
				return
			}
		}

		if !hasMore {
			break
		}
		startIndex = history.LoadedServerIndex
	}
	fmt.Println("Not found")
}

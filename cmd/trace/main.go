package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
)

func main() {
	target := "2MNjM5gT" // "Book Teeth Cleaning"

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

	startIndex := 0
	for {
		items, hasMore, err := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})
		if err != nil {
			log.Fatalf("read items: %v", err)
		}
		for _, item := range items {
			if strings.HasPrefix(item.UUID, target) {
				var p map[string]interface{}
				if err := json.Unmarshal(item.P, &p); err != nil {
					log.Printf("decode payload for %s: %v", item.UUID, err)
				}
				pJSON, err := json.MarshalIndent(p, "", "  ")
				if err != nil {
					log.Printf("encode payload for %s: %v", item.UUID, err)
					continue
				}
				fmt.Printf("=== %s Action:%d Kind:%s ===\n%s\n\n", item.UUID, item.Action, item.Kind, string(pJSON))
			}
		}
		if !hasMore {
			break
		}
		startIndex = history.LoadedServerIndex
	}
}

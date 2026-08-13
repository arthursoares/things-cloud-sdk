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

	// Get the LAST batch (most recent items)
	fmt.Printf("LatestServerIndex: %d\n", history.LatestServerIndex)

	// Start from near the end
	startIndex := history.LatestServerIndex - 100
	if startIndex < 0 {
		startIndex = 0
	}

	items, _, err := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})
	if err != nil {
		log.Fatalf("read items: %v", err)
	}

	fmt.Printf("\n=== LAST %d ITEMS ===\n", len(items))
	for _, item := range items {
		var p map[string]interface{}
		if err := json.Unmarshal(item.P, &p); err != nil {
			log.Printf("decode payload for %s: %v", item.UUID, err)
		}
		title := p["tt"]
		fmt.Printf("[%s] Action:%d UUID:%s Title:%v\n", item.Kind, item.Action, item.UUID[:8], title)
	}
}

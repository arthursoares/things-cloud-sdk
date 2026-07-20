package main

import (
	"encoding/json"
	"fmt"
	"os"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
)

func main() {
	c := thingscloud.New(thingscloud.APIEndpoint, os.Getenv("THINGS_USERNAME"), os.Getenv("THINGS_PASSWORD"))
	c.Verify()

	history, _ := c.OwnHistory()
	history.Sync()

	// Get the LAST batch (most recent items)
	fmt.Printf("LatestServerIndex: %d\n", history.LatestServerIndex)

	// Start from near the end
	startIndex := history.LatestServerIndex - 100
	if startIndex < 0 {
		startIndex = 0
	}

	items, _, _ := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})

	fmt.Printf("\n=== LAST %d ITEMS ===\n", len(items))
	for _, item := range items {
		var p map[string]interface{}
		json.Unmarshal(item.P, &p)
		title := p["tt"]
		fmt.Printf("[%s] Action:%d UUID:%s Title:%v\n", item.Kind, item.Action, item.UUID[:8], title)
	}
}

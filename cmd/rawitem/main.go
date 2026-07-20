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

	target := "2MNjM5gT5hZt2sSEw4PvDb"

	startIndex := 0
	for {
		items, hasMore, _ := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})

		for _, item := range items {
			if item.UUID == target {
				fmt.Printf("UUID: %s\n", item.UUID)
				fmt.Printf("Kind: %s\n", item.Kind)
				fmt.Printf("Action (parsed): %d\n", item.Action)

				// Also print item as raw JSON
				raw, _ := json.MarshalIndent(item, "", "  ")
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

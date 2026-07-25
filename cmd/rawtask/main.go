package main

import (
	"encoding/json"
	"fmt"
	"os"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
)

func main() {
	c := thingscloud.New(thingscloud.APIEndpoint, os.Getenv("THINGS_USERNAME"), os.Getenv("THINGS_PASSWORD"))

	_, err := c.Verify()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}

	history, err := c.OwnHistory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed loading history: %v\n", err)
		os.Exit(1)
	}

	if err := history.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed syncing history: %v\n", err)
		os.Exit(1)
	}

	// Fetch all and show some recent Task6 items with titles
	startIndex := 0
	count := 0
	for {
		items, hasMore, err := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed loading items: %v\n", err)
			os.Exit(1)
		}
		for _, item := range items {
			if item.Kind == "Task6" && count < 20 {
				var p map[string]interface{}
				if err := json.Unmarshal(item.P, &p); err != nil {
					fmt.Fprintf(os.Stderr, "Failed decoding payload for %s: %v\n", item.UUID, err)
				}
				if title, ok := p["tt"]; ok && title != nil {
					fmt.Printf("UUID: %s, Title: %v, Action: %d\n", item.UUID, title, item.Action)
					count++
				}
			}
		}
		if !hasMore || count >= 20 {
			break
		}
		startIndex = history.LoadedServerIndex
	}
	fmt.Printf("\nFound %d Task6 items with titles\n", count)
}

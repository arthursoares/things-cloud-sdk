package main

import (
	"fmt"
	"os"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
	memory "github.com/arthursoares/things-cloud-sdk/state/memory"
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
	// Fetch all items with pagination
	var allItems []thingscloud.Item
	startIndex := 0
	for {
		items, hasMore, err := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed loading items: %v\n", err)
			os.Exit(1)
		}
		allItems = append(allItems, items...)
		if !hasMore {
			break
		}
		startIndex = history.LoadedServerIndex
	}
	items := allItems
	fmt.Fprintf(os.Stderr, "Fetched %d items\n", len(items))

	state := memory.NewState()
	if err := state.Update(items...); err != nil {
		fmt.Fprintf(os.Stderr, "Failed updating state: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== TASKS (%d) ===\n", len(state.Tasks))
	for _, task := range state.Tasks {
		fmt.Printf("- %s (InTrash:%v, Status:%d)\n", task.Title, task.InTrash, task.Status)
	}
}

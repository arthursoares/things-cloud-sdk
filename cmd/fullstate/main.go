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

	// Fetch all items
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

	state := memory.NewState()
	if err := state.Update(allItems...); err != nil {
		fmt.Fprintf(os.Stderr, "Failed updating state: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== STATE SUMMARY ===\n")
	fmt.Printf("Areas: %d\n", len(state.Areas))
	fmt.Printf("Tags: %d\n", len(state.Tags))
	fmt.Printf("Tasks: %d\n", len(state.Tasks))
	fmt.Printf("ChecklistItems: %d\n", len(state.CheckListItems))

	fmt.Printf("\n=== AREAS ===\n")
	for _, area := range state.Areas {
		fmt.Printf("- %s\n", area.Title)
	}

	fmt.Printf("\n=== TAGS ===\n")
	for _, tag := range state.Tags {
		fmt.Printf("- %s\n", tag.Title)
	}

	fmt.Printf("\n=== ALL TASKS (including trashed/completed) ===\n")
	for _, task := range state.Tasks {
		status := "open"
		if task.Status == 3 {
			status = "completed"
		}
		if task.InTrash {
			status = "trashed"
		}
		fmt.Printf("- [%s] %s (project:%v)\n", status, task.Title, task.Type == thingscloud.TaskTypeProject)
	}
}

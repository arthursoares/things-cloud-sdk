package main

import (
	"fmt"
	"os"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
	memory "github.com/arthursoares/things-cloud-sdk/state/memory"
)

func main() {
	c := thingscloud.New(thingscloud.APIEndpoint, os.Getenv("THINGS_USERNAME"), os.Getenv("THINGS_PASSWORD"))
	c.Verify()

	history, _ := c.OwnHistory()
	history.Sync()

	state := memory.NewState()

	// Fetch and update incrementally
	startIndex := 0
	totalTask6 := 0
	for {
		items, hasMore, _ := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})

		for _, item := range items {
			if item.Kind == "Task6" {
				totalTask6++
			}
		}

		state.Update(items...)

		if !hasMore {
			break
		}
		startIndex = history.LoadedServerIndex
	}

	fmt.Printf("Total Task6 items processed: %d\n", totalTask6)
	fmt.Printf("Final state.Tasks count: %d\n", len(state.Tasks))

	fmt.Println("\nAll tasks in state:")
	for uuid, task := range state.Tasks {
		fmt.Printf("  %s: %s (trash:%v status:%d)\n", uuid[:8], task.Title, task.InTrash, task.Status)
	}
}

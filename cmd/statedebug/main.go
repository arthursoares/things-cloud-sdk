package main

import (
	"fmt"
	"log"
	"os"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
	memory "github.com/arthursoares/things-cloud-sdk/state/memory"
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

	state := memory.NewState()

	// Fetch and update incrementally
	startIndex := 0
	totalTask6 := 0
	for {
		items, hasMore, err := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})
		if err != nil {
			log.Fatalf("read items: %v", err)
		}

		for _, item := range items {
			if item.Kind == "Task6" {
				totalTask6++
			}
		}

		if err := state.Update(items...); err != nil {
			log.Fatalf("update state: %v", err)
		}

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

package main

import (
	"encoding/json"
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
	target := "2MNjM5gT" // Book Teeth Cleaning

	startIndex := 0
	for {
		items, hasMore, err := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})
		if err != nil {
			log.Fatalf("read items: %v", err)
		}

		// Check state before update
		if t, ok := state.Tasks[target+"5hZt2sSEw4PvDb"]; ok {
			fmt.Printf("BEFORE batch: Task exists, title=%q trash=%v\n", t.Title, t.InTrash)
		}

		if err := state.Update(items...); err != nil {
			log.Fatalf("update state: %v", err)
		}

		// Check state after update
		if t, ok := state.Tasks[target+"5hZt2sSEw4PvDb"]; ok {
			fmt.Printf("AFTER batch: Task exists, title=%q trash=%v\n", t.Title, t.InTrash)
		}

		// Look for our target in this batch
		for _, item := range items {
			if item.UUID == target+"5hZt2sSEw4PvDb" {
				var p map[string]interface{}
				if err := json.Unmarshal(item.P, &p); err != nil {
					log.Printf("decode payload for %s: %v", item.UUID, err)
				}
				fmt.Printf("Found item: Kind=%s Action=%d tr=%v\n", item.Kind, item.Action, p["tr"])
			}
		}

		if !hasMore {
			break
		}
		startIndex = history.LoadedServerIndex
	}

	fmt.Printf("\nFinal state has %d tasks\n", len(state.Tasks))
	if t, ok := state.Tasks[target+"5hZt2sSEw4PvDb"]; ok {
		fmt.Printf("Target task: %q trash=%v status=%d\n", t.Title, t.InTrash, t.Status)
	} else {
		fmt.Println("Target task NOT in final state")
	}
}

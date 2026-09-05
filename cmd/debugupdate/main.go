package main

import (
	"fmt"
	"log"
	"os"
	"strings"

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
	target := "2MNjM5gT"

	startIndex := 0
	for {
		items, hasMore, err := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})
		if err != nil {
			log.Fatalf("read items: %v", err)
		}

		for _, item := range items {
			if strings.HasPrefix(item.UUID, target) {
				fmt.Printf("Processing: UUID=%s Kind=%s Action=%d\n", item.UUID, item.Kind, item.Action)

				// Try updating just this one item
				err := state.Update(item)
				if err != nil {
					fmt.Printf("ERROR: %v\n", err)
				}

				// Check if it's in state now
				if t, ok := state.Tasks[item.UUID]; ok {
					fmt.Printf("  -> In state: title=%q\n", t.Title)
				} else {
					fmt.Printf("  -> NOT in state!\n")
				}
			}
		}

		if !hasMore {
			break
		}
		startIndex = history.LoadedServerIndex
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
	memory "github.com/arthursoares/things-cloud-sdk/state/memory"
)

func main() {
	c := thingscloud.New(thingscloud.APIEndpoint, os.Getenv("THINGS_USERNAME"), os.Getenv("THINGS_PASSWORD"))
	c.Verify()
	history, _ := c.OwnHistory()
	history.Sync()

	state := memory.NewState()
	target := "2MNjM5gT"

	startIndex := 0
	for {
		items, hasMore, _ := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})

		for _, item := range items {
			if strings.HasPrefix(item.UUID, target) {
				var p map[string]interface{}
				json.Unmarshal(item.P, &p)
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

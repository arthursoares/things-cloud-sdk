package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
)

func main() {
	target := "2MNjM5gT" // "Book Teeth Cleaning"

	c := thingscloud.New(thingscloud.APIEndpoint, os.Getenv("THINGS_USERNAME"), os.Getenv("THINGS_PASSWORD"))
	c.Verify()

	history, _ := c.OwnHistory()
	history.Sync()

	startIndex := 0
	for {
		items, hasMore, _ := history.Items(thingscloud.ItemsOptions{StartIndex: startIndex})
		for _, item := range items {
			if strings.HasPrefix(item.UUID, target) {
				var p map[string]interface{}
				json.Unmarshal(item.P, &p)
				pJSON, _ := json.MarshalIndent(p, "", "  ")
				fmt.Printf("=== %s Action:%d Kind:%s ===\n%s\n\n", item.UUID, item.Action, item.Kind, string(pJSON))
			}
		}
		if !hasMore {
			break
		}
		startIndex = history.LoadedServerIndex
	}
}

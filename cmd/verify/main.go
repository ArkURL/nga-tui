package main

import (
	"fmt"

	"github.com/ArkURL/nga-tui/internal/api"
)

func main() {
	c := api.NewClient()
	res, err := c.GetThreadContent(47389174, 1)
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	fmt.Printf("ok replies=%d rows=%d\n", len(res.Replies), res.Rows)
}

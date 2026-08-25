package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/ArkURL/nga-tui/internal/api"
)

func main() {
	c := api.NewClient()
	// 对比：不带 rand vs 带 rand，各取 3 次
	for _, withRand := range []bool{false, true} {
		for i := 0; i < 3; i++ {
			p := url.Values{"tid": {"47389174"}, "__output": {"11"}}
			if withRand {
				p.Set("rand", strconv.FormatInt(time.Now().UnixNano()%1000000, 10))
			}
			body, err := c.Get("/read.php", p)
			if err != nil {
				fmt.Printf("rand=%v 第%d次 HTTP ERR: %v\n", withRand, i+1, err)
				continue
			}
			fmt.Printf("rand=%v 第%d次 len=%d valid=%v\n", withRand, i+1, len(body), json.Valid(body))
			time.Sleep(300 * time.Millisecond)
		}
	}
}

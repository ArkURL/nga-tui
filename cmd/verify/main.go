package main

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/ArkURL/nga-tui/internal/api"
)

func main() {
	c := api.NewClient()
	// 探测一批版面的 __F.sub_forums
	for _, fid := range []string{"7", "428", "-7", "32", "429", "467", "335", "318", "510567"} {
		body, err := c.Get("/thread.php", url.Values{"fid": {fid}, "__output": {"11"}})
		if err != nil {
			fmt.Printf("fid=%-8s HTTP ERR: %v\n", fid, err)
			continue
		}
		var root map[string]json.RawMessage
		json.Unmarshal(api.FixJSON(body), &root)
		var data map[string]json.RawMessage
		json.Unmarshal(root["data"], &data)
		fRaw, ok := data["__F"]
		if !ok {
			fmt.Printf("fid=%-8s 无 __F\n", fid)
			continue
		}
		var f map[string]json.RawMessage
		json.Unmarshal(fRaw, &f)
		var name string
		json.Unmarshal(f["name"], &name)
		sub := string(f["sub_forums"])
		if sub == "null" {
			sub = "null"
		}
		fmt.Printf("fid=%-8s name=%-12s sub_forums=%s\n", fid, name, sub)
	}
}

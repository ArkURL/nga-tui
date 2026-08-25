package api

import (
	"fmt"
	"net/url"

	"github.com/ArkURL/nga-tui/internal/model"
)

// GetCategories 拉取版面分类（三级：分类→分组→版面）。
func (c *Client) GetCategories() ([]model.Category, error) {
	params := url.Values{}
	params.Set("__lib", "home")
	params.Set("__act", "category")
	params.Set("__output", "11")

	var cats []model.Category
	err := c.fetchJSON("/app_api.php", params, func(body []byte) error {
		if err := decodeResponse(body, &cats); err != nil {
			return fmt.Errorf("解析版面分类: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cats, nil
}

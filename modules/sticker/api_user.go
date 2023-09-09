package sticker

import "github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"

type Category struct {
	Category string `json:"category"`
}

const emojiTypeName = "emoji"

func NewCategory() *Category {
	return &Category{
		Category: emojiTypeName,
	}
}

func (s *Sticker) getCategory(c *wkhttp.Context) {
	category := NewCategory()
	data := []*Category{}
	data = append(data, category)
	c.Response(data)
}

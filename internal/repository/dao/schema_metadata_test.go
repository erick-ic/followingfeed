package dao

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

// TestDAOModelsDoNotDeclareIndexes 防止在 DAO 模型中重新引入第二份索引定义。
// 物理表结构和索引以 migrations 中的 SQL 为唯一事实源。
func TestDAOModelsDoNotDeclareIndexes(t *testing.T) {
	models := []struct {
		name  string
		model any
	}{
		{"users", &User{}},
		{"articles", &Article{}},
		{"publish_articles", &PublishArticle{}},
		{"follows", &Follow{}},
		{"interactives", &Interactive{}},
		{"user_like_bizs", &UserLikeBiz{}},
		{"user_collection_bizs", &UserCollectionBiz{}},
	}

	for _, tc := range models {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := schema.Parse(tc.model, &sync.Map{}, schema.NamingStrategy{})
			require.NoError(t, err)
			require.Empty(t, parsed.ParseIndexes())
		})
	}
}

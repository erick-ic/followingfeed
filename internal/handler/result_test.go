package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPageResultUsesEmptyArrayForNoItems(t *testing.T) {
	result := NewPageResult[string](nil, 1, 10, 0)
	data, err := json.Marshal(result)
	require.NoError(t, err)
	assert.JSONEq(t, `{"items":[],"page":1,"pageSize":10,"total":0,"totalPages":0}`, string(data))
}

func TestPaginationRejectsDeepAndOverflowingPages(t *testing.T) {
	for _, page := range []int{1001, int(^uint(0) >> 1)} {
		require.NotEmpty(t, ValidatePagination(page, 50))
	}
	require.Empty(t, ValidatePagination(1000, 50))
	require.Equal(t, 1000, NewPageResult[string](nil, 1, 10, 1000000).TotalPages)
}

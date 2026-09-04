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

package handler

import "followingfeed/pkg/ginx"

type Result = ginx.Result

// PageResult 是所有列表接口共享的分页响应。
type PageResult[T any] struct {
	Items      []T   `json:"items"`
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

func NewPageResult[T any](items []T, page, pageSize int, total int64) PageResult[T] {
	if items == nil {
		items = make([]T, 0)
	}
	return PageResult[T]{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}
}

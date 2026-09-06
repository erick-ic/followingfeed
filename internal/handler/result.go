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
	totalPages := int(total / int64(pageSize))
	if total%int64(pageSize) != 0 {
		totalPages++
	}
	if totalPages > MaxPage {
		totalPages = MaxPage
	}
	return PageResult[T]{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}

// MaxPage 将 offset 分页限制在受控范围，避免整数溢出与任意深分页。
const MaxPage = 1000

func ValidatePagination(page, pageSize int) string {
	if page < 1 || pageSize < 1 {
		return "page 和 pageSize 必须大于 0"
	}
	if pageSize > 50 {
		return "pageSize 不能超过 50"
	}
	if page > MaxPage {
		return "page 不能超过 1000"
	}
	return ""
}

package migrations

import (
	"fmt"
	"strings"
)

// splitSQLStatements 支持当前迁移使用的引号、反引号及行/块注释。
// 存储过程等需要 DELIMITER 的脚本应交给专用迁移工具，不在此处静默误解析。
func splitSQLStatements(script string) ([]string, error) {
	if strings.Contains(strings.ToUpper(script), "DELIMITER ") {
		return nil, fmt.Errorf("不支持包含 DELIMITER 的迁移脚本")
	}

	statements := make([]string, 0)
	var current strings.Builder
	var quote byte
	lineComment := false
	blockComment := false

	for index := 0; index < len(script); index++ {
		char := script[index]
		next := byte(0)
		if index+1 < len(script) {
			next = script[index+1]
		}

		if lineComment {
			current.WriteByte(char)
			if char == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			current.WriteByte(char)
			if char == '*' && next == '/' {
				current.WriteByte(next)
				index++
				blockComment = false
			}
			continue
		}
		if quote != 0 {
			current.WriteByte(char)
			if char == '\\' && quote != '`' && next != 0 {
				current.WriteByte(next)
				index++
				continue
			}
			if char == quote {
				if next == quote {
					current.WriteByte(next)
					index++
				} else {
					quote = 0
				}
			}
			continue
		}

		switch {
		case char == '-' && next == '-':
			current.WriteByte(char)
			current.WriteByte(next)
			index++
			lineComment = true
		case char == '#':
			current.WriteByte(char)
			lineComment = true
		case char == '/' && next == '*':
			current.WriteByte(char)
			current.WriteByte(next)
			index++
			blockComment = true
		case char == '\'' || char == '"' || char == '`':
			current.WriteByte(char)
			quote = char
		case char == ';':
			appendStatement(&statements, &current)
		default:
			current.WriteByte(char)
		}
	}

	if quote != 0 {
		return nil, fmt.Errorf("引号内容没有正确结束")
	}
	if blockComment {
		return nil, fmt.Errorf("块注释没有正确结束")
	}
	appendStatement(&statements, &current)
	return statements, nil
}

func appendStatement(statements *[]string, current *strings.Builder) {
	statement := strings.TrimSpace(current.String())
	current.Reset()
	if statement != "" {
		*statements = append(*statements, statement)
	}
}

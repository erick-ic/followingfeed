package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitSQLStatements(t *testing.T) {
	script := "-- a semicolon in a comment; must not split\n" +
		"INSERT INTO examples(value) VALUES ('a;b');\n" +
		"/* another ; comment */\n" +
		"ALTER TABLE `examples` ADD COLUMN \"quoted;name\" VARCHAR(32);"

	statements, err := splitSQLStatements(script)
	require.NoError(t, err)
	require.Len(t, statements, 2)
	assert.Contains(t, statements[0], "'a;b'")
	assert.Contains(t, statements[1], "\"quoted;name\"")
}

func TestSplitSQLStatementsRejectsUnsupportedOrBrokenScripts(t *testing.T) {
	_, err := splitSQLStatements("DELIMITER //")
	assert.ErrorContains(t, err, "不支持")

	_, err = splitSQLStatements("INSERT INTO examples VALUES ('broken);")
	assert.ErrorContains(t, err, "没有正确结束")
}

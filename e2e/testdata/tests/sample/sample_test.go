package sample

import "testing"

func TestCreateUser(t *testing.T) { t.Log("created user 1") }

func TestDeleteUser(t *testing.T) { t.Fatal("want 204, got 500") }

func TestPostgres(t *testing.T) { t.Skip("needs a database") }

func TestLineSplitterTruncatesHugeLineAcrossChunksWithoutLosingTheNextLine(t *testing.T) {}

// Subtest names with quotes, as table tests often have.
func TestParseValue(t *testing.T) {
	for _, name := range []string{`"hello"`, `'world'`} {
		t.Run(name, func(t *testing.T) {})
	}
}

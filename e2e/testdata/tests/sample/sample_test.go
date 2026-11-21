package sample

import "testing"

func TestCreateUser(t *testing.T) { t.Log("created user 1") }

func TestDeleteUser(t *testing.T) { t.Fatal("want 204, got 500") }

func TestPostgres(t *testing.T) { t.Skip("needs a database") }

func TestLineSplitterTruncatesHugeLineAcrossChunksWithoutLosingTheNextLine(t *testing.T) {}

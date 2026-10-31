package main

import "testing"

func TestSheetCols(t *testing.T) {
	h := func(s string) [2]string { return [2]string{sheetHead, s} }
	r := func(s string) [2]string { return [2]string{s, ""} }
	rows := [][2]string{h("Keys"), r("a"), r("b"), h("Commands"), r("c"), r("d"), r("e")}
	cols := sheetCols(rows, 4)
	// the heading would be last in the first column: it starts the second
	want := [][]string{{"Keys", "a", "b"}, {"Commands", "c", "d", "e"}}
	if len(cols) != len(want) {
		t.Fatalf("%d columns, want %d: %v", len(cols), len(want), cols)
	}
	for i, col := range cols {
		for j, row := range col {
			name := row[0]
			if name == sheetHead {
				name = row[1]
			}
			if j >= len(want[i]) || name != want[i][j] {
				t.Errorf("column %d row %d = %q, want %v", i, j, name, want[i])
			}
		}
	}
	if got := sheetCols(rows, 100); len(got) != 1 || len(got[0]) != len(rows) {
		t.Errorf("all in one column: %v", got)
	}
	cmds := []exCmd{{names: "find / f", arg: "word", help: "look", run: func(string) error { return nil }}, {names: "help", help: "the list"}}
	rs := sheetRows([][2]string{{"x", "y"}}, cmds)
	if last := rs[len(rs)-1]; last[0] != ":find word" || last[1] != "look  (:/ :f)" {
		t.Errorf("the command's row: %q", last)
	}
}

func TestSheetLayout(t *testing.T) {
	rows := make([][2]string, 30)
	for _, c := range []struct{ rowsMax, colsMax, cols, longest int }{
		{40, 3, 1, 30}, // they fit in one: one, as wide as it can be
		{20, 3, 2, 15}, // two needed: even
		{12, 3, 3, 10},
		{5, 3, 6, 5}, // too many: the rest past what shows
	} {
		cols := sheetLayout(rows, c.rowsMax, c.colsMax)
		longest := 0
		for _, col := range cols {
			longest = max(longest, len(col))
		}
		if len(cols) != c.cols || longest != c.longest {
			t.Errorf("sheetLayout(30 rows, %d, %d): %d columns, the longest %d; want %d, %d", c.rowsMax, c.colsMax, len(cols), longest, c.cols, c.longest)
		}
	}
}

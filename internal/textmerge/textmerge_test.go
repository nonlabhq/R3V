package textmerge

import (
	"reflect"
	"testing"
)

func TestMerge(t *testing.T) {
	for _, c := range []struct {
		name, base, ours, theirs, want string
		clean                          bool
	}{
		{"both add elsewhere", "a\nb\nc\n", "x\na\nb\nc\n", "a\nb\nc\ny\n", "x\na\nb\nc\ny\n", true},
		{"edit different lines", "a\nb\nc\nd\n", "A\nb\nc\nd\n", "a\nb\nc\nD\n", "A\nb\nc\nD\n", true},
		{"same edit both sides", "a\nb\n", "a\nB\n", "a\nB\n", "a\nB\n", true},
		{"one side deletes", "a\nb\nc\n", "a\nc\n", "a\nb\nc\nd\n", "a\nc\nd\n", true},
		{"only theirs", "a\n", "a\n", "a\nb\n", "a\nb\n", true},
		{"no final newline", "a\nb", "a\nB", "z\na\nb", "z\na\nB", true},
		{"from empty, same text", "", "a\n", "a\n", "a\n", true},
		{"same line differently", "a\nb\nc\n", "a\nX\nc\n", "a\nY\nc\n", "", false},
		{"both insert at same place", "a\nc\n", "a\nb1\nc\n", "a\nb2\nc\n", "", false},
	} {
		got, clean, err := Merge([]byte(c.base), []byte(c.ours), []byte(c.theirs))
		if err != nil || clean != c.clean || (clean && string(got) != c.want) {
			t.Errorf("%s: clean=%v %q, want clean=%v %q", c.name, clean, got, c.clean, c.want)
		}
	}
}

func TestHunks(t *testing.T) {
	for _, c := range []struct {
		name, base, ours, theirs string
		hunks                    []Hunk
		choices                  []string
		want                     string
	}{
		{"adjacent edits", "a\nb\nc\n", "A\nb\nc\n", "a\nB\nc\n",
			[]Hunk{{Side{1, 2, "a\nb\n"}, Side{1, 2, "A\nb\n"}, Side{1, 2, "a\nB\n"}}},
			[]string{"ours"}, "A\nb\nc\n"},
		{"insert at same place", "a\nc\n", "a\nb1\nc\n", "a\nb2\nc\n",
			[]Hunk{{Side{2, 0, ""}, Side{2, 1, "b1\n"}, Side{2, 1, "b2\n"}}},
			[]string{"both"}, "a\nb1\nb2\nc\n"},
		{"delete vs change", "a\nb\nc\n", "a\nc\n", "a\nB\nc\n",
			[]Hunk{{Side{2, 1, "b\n"}, Side{2, 0, ""}, Side{2, 1, "B\n"}}},
			[]string{"theirs"}, "a\nB\nc\n"},
		{"crlf", "a\r\nb\r\nc\r\n", "a\r\nX\r\nc\r\n", "a\r\nY\r\nc\r\n",
			[]Hunk{{Side{2, 1, "b\r\n"}, Side{2, 1, "X\r\n"}, Side{2, 1, "Y\r\n"}}},
			[]string{"both"}, "a\r\nX\r\nY\r\nc\r\n"},
		{"no trailing newline", "a\nb", "a\nX", "a\nY",
			[]Hunk{{Side{2, 1, "b"}, Side{2, 1, "X"}, Side{2, 1, "Y"}}},
			[]string{"both"}, "a\nX\nY"},
		{"crlf, no trailing newline", "a\r\nb", "a\r\nX", "a\r\nY",
			[]Hunk{{Side{2, 1, "b"}, Side{2, 1, "X"}, Side{2, 1, "Y"}}},
			[]string{"both"}, "a\r\nX\r\nY"},
		{"two hunks, a clean change between", "1\n2\n3\n4\n5\n6\n7\n", "1\nO2\n3\n4\n5\nO6\n7\n", "1\nT2\n3\nT4\n5\nT6\n7\n",
			[]Hunk{
				{Side{2, 1, "2\n"}, Side{2, 1, "O2\n"}, Side{2, 1, "T2\n"}},
				{Side{6, 1, "6\n"}, Side{6, 1, "O6\n"}, Side{6, 1, "T6\n"}},
			},
			[]string{"ours", "theirs"}, "1\nO2\n3\nT4\n5\nT6\n7\n"},
		{"clean", "a\nb\nc\n", "A\nb\nc\n", "a\nb\nC\n", nil, nil, "A\nb\nC\n"},
	} {
		res := Hunks([]byte(c.base), []byte(c.ours), []byte(c.theirs))
		if got := res.Hunks(); !reflect.DeepEqual(got, c.hunks) {
			t.Errorf("%s: hunks %+v, want %+v", c.name, got, c.hunks)
		}
		got, clean, err := res.Resolve(c.choices)
		if err != nil || !clean || string(got) != c.want {
			t.Errorf("%s: %q %v %v, want %q", c.name, got, clean, err, c.want)
		}
		if len(c.hunks) > 0 {
			if _, _, err := res.Resolve(nil); err == nil {
				t.Errorf("%s: resolved without choices", c.name)
			}
		}
	}
}

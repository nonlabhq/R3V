package manifest

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// A record a code formatter laid out again (lines joined, other indents,
// CRLF, compact) comes back byte for byte; one whose values changed doesn't.
// What comes back is always what the id names: restoring never guesses.
func TestRestore(t *testing.T) {
	m := &Manifest{Version: Format, Parents: []string{hashOf("parent")}, Author: "yi", AuthorID: "a1",
		Time: "2026-10-10T12:37:43Z", Message: "Add rules <and> \"notes\" & more\n\nDetails.", Branch: "main",
		Actor: &Actor{Kind: "agent", Tool: "claude-code"}, Tree: hashOf("tree"), FileCount: 21, TotalSize: 51969,
		External: []FileEntry{{Path: "C:/Samples/kick.wav", Hash: hashOf("kick"), Size: 4}}, Packs: []string{"Core Library"}}
	data := m.Seal()
	var compact bytes.Buffer
	json.Compact(&compact, data)
	joined := strings.Replace(string(data), "[\n    \""+hashOf("parent")+"\"\n  ]", "[\""+hashOf("parent")+"\"]", 1)
	if joined == string(data) {
		t.Fatal("the test's reformatting didn't apply")
	}
	for name, laidOut := range map[string]string{
		"joined":  joined,
		"tabs":    strings.ReplaceAll(string(data), "  ", "\t"),
		"crlf":    strings.ReplaceAll(string(data), "\n", "\r\n"),
		"compact": compact.String(),
		"no end":  strings.TrimSuffix(string(data), "\n"),
		// What it doesn't know is dropped: the id proves what is left is the record.
		"extra field": strings.Replace(string(data), `"main"`, `"main", "extra": 1`, 1),
	} {
		got, ok := Restore(m.ID, []byte(laidOut))
		if !ok || !bytes.Equal(got, data) {
			t.Errorf("%s: not restored (%v)", name, ok)
		}
	}
	for name, changed := range map[string]string{
		"value": strings.Replace(string(data), "51969", "51970", 1),
		"other": strings.Replace(string(data), `"main"`, `"idea"`, 1),
		"junk":  "not json",
	} {
		if _, ok := Restore(m.ID, []byte(changed)); ok {
			t.Errorf("%s: taken as the same record", name)
		}
	}
}

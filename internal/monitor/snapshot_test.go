package monitor

import "testing"

func TestParseTasklistCSV(t *testing.T) {
	data := "\"Cyberpunk2077.exe\",\"22100\",\"Console\",\"1\",\"1,024 K\"\r\n\"RDR2.exe\",\"20200\",\"Console\",\"1\",\"500 K\"\r\n"
	snapshot, err := parseTasklistCSV(data)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Has("cyberpunk2077.EXE") || !snapshot.Has("rdr2.exe") {
		t.Fatalf("processes missing: %#v", snapshot)
	}
}

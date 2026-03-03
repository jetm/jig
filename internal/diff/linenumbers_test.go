package diff

import (
	"testing"
)

func TestParseLineNumbers_SingleHunk(t *testing.T) {
	raw := `diff --git a/foo.go b/foo.go
index abc..def 100644
--- a/foo.go
+++ b/foo.go
@@ -10,4 +10,5 @@ func main() {
 unchanged
-removed
+added1
+added2
 context`

	got := ParseLineNumbers(raw)
	if len(got) != 10 {
		t.Fatalf("len: got %d, want 10", len(got))
	}

	tests := []struct {
		line int
		num  int
		lt   LineType
	}{
		{0, 0, LineHeader},     // diff --git
		{1, 0, LineHeader},     // index
		{2, 0, LineHeader},     // ---
		{3, 0, LineHeader},     // +++
		{4, 0, LineHunkHeader}, // @@
		{5, 10, LineContext},   // unchanged (new line 10)
		{6, 10, LineRemoved},   // removed (old line 10+1=11? no: old starts at 10, context increments both)
		{7, 11, LineAdded},     // added1 (new line 11)
		{8, 12, LineAdded},     // added2 (new line 12)
		{9, 13, LineContext},   // context (new line 13)
	}

	// After hunk header: old=10, new=10
	// Line 5 " unchanged": context -> old=11, new=11
	// Line 6 "-removed":   removed old=11 -> old=12
	// Line 7 "+added1":    added new=11 -> new=12
	// Line 8 "+added2":    added new=12 -> new=13
	// Line 9 " context":   context -> old=13, new=14
	tests[6].num = 11 // removed: old line 11
	tests[9].num = 13 // context: new line 13

	for _, tt := range tests {
		if got[tt.line].Num != tt.num {
			t.Errorf("line %d: Num got %d, want %d", tt.line, got[tt.line].Num, tt.num)
		}
		if got[tt.line].LineType != tt.lt {
			t.Errorf("line %d: LineType got %d, want %d", tt.line, got[tt.line].LineType, tt.lt)
		}
	}
}

func TestParseLineNumbers_MultipleHunks(t *testing.T) {
	raw := `diff --git a/foo.go b/foo.go
index abc..def 100644
--- a/foo.go
+++ b/foo.go
@@ -10,2 +10,2 @@ func A() {
 ctx1
-old1
+new1
@@ -50,2 +50,2 @@ func B() {
 ctx2
-old2
+new2`

	got := ParseLineNumbers(raw)

	// Lines: 0-3 headers, 4 hunk1, 5 ctx1, 6 -old1, 7 +new1,
	//        8 hunk2, 9 ctx2, 10 -old2, 11 +new2
	if got[8].LineType != LineHunkHeader {
		t.Errorf("line 8: want HunkHeader, got %d", got[8].LineType)
	}
	// After second hunk: old=50, new=50
	// Line 9 " ctx2": context new=50 -> old=51, new=51
	if got[9].Num != 50 {
		t.Errorf("line 9: Num got %d, want 50 (new-file after second hunk)", got[9].Num)
	}
	// Line 10 "-old2": removed old=51
	if got[10].Num != 51 || got[10].LineType != LineRemoved {
		t.Errorf("line 10: got Num=%d Type=%d, want Num=51 Type=Removed", got[10].Num, got[10].LineType)
	}
	// Line 11 "+new2": added new=51
	if got[11].Num != 51 || got[11].LineType != LineAdded {
		t.Errorf("line 11: got Num=%d Type=%d, want Num=51 Type=Added", got[11].Num, got[11].LineType)
	}
}
func TestParseLineNumbers_MultipleFiles(t *testing.T) {
	raw := `diff --git a/foo.go b/foo.go
index abc..def 100644
--- a/foo.go
+++ b/foo.go
@@ -5,2 +5,2 @@ func Foo() {
 ctx
+added
diff --git a/bar.go b/bar.go
index 111..222 100644
--- a/bar.go
+++ b/bar.go
@@ -20,2 +20,2 @@ func Bar() {
 ctx
-removed`

	got := ParseLineNumbers(raw)

	// First file: line 5 " ctx" -> new=5
	if got[5].Num != 5 || got[5].LineType != LineContext {
		t.Errorf("first file ctx: got Num=%d Type=%d, want Num=5 Type=Context", got[5].Num, got[5].LineType)
	}

	// Second file starts at index 7 (diff --git)
	if got[7].LineType != LineHeader {
		t.Errorf("second file header: got %d, want Header", got[7].LineType)
	}

	// Second file hunk at index 11 (@@ -20,2 +20,2 @@)
	if got[11].LineType != LineHunkHeader {
		t.Errorf("second hunk header: got %d, want HunkHeader", got[11].LineType)
	}

	// Line 12 " ctx": new=20
	if got[12].Num != 20 {
		t.Errorf("second file ctx: got Num=%d, want 20", got[12].Num)
	}

	// Line 13 "-removed": old=21
	if got[13].Num != 21 || got[13].LineType != LineRemoved {
		t.Errorf("second file removed: got Num=%d Type=%d, want Num=21 Type=Removed", got[13].Num, got[13].LineType)
	}
}

func TestParseLineNumbers_HeadersProduceBlankEntries(t *testing.T) {
	raw := `diff --git a/foo.go b/foo.go
index abc..def 100644
--- a/foo.go
+++ b/foo.go
@@ -1,1 +1,1 @@
-old
+new`

	got := ParseLineNumbers(raw)
	for i := range 4 {
		if got[i].Num != 0 {
			t.Errorf("header line %d: Num got %d, want 0", i, got[i].Num)
		}
		if got[i].LineType != LineHeader {
			t.Errorf("header line %d: LineType got %d, want Header", i, got[i].LineType)
		}
	}
	if got[4].Num != 0 || got[4].LineType != LineHunkHeader {
		t.Errorf("hunk header: got Num=%d Type=%d, want Num=0 Type=HunkHeader", got[4].Num, got[4].LineType)
	}
}

func TestParseLineNumbers_EmptyInput(t *testing.T) {
	got := ParseLineNumbers("")
	if got != nil {
		t.Errorf("empty input: got %v, want nil", got)
	}
}

func TestParseLineNumbers_NonDiffText(t *testing.T) {
	raw := "just some random text\nwith multiple lines\n"
	got := ParseLineNumbers(raw)
	for i, info := range got {
		if info.Num != 0 {
			t.Errorf("non-diff line %d: Num got %d, want 0", i, info.Num)
		}
	}
}

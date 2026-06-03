package diff

import (
	"testing"
)

func FuzzParseLineNumbers(f *testing.F) {
	f.Add("@@ -1,3 +1,3 @@\n line\n-old\n+new\n")
	f.Add("")
	f.Add("diff --git a/foo b/foo\n--- a/foo\n+++ b/foo\n@@ -10,2 +10,2 @@\n context\n-rem\n+add\n")
	f.Add("@@ -0,0 +1 @@\n+first line\n")
	f.Add("not a diff")

	f.Fuzz(func(t *testing.T, s string) {
		result := ParseLineNumbers(s)
		for i, li := range result {
			if li.Num < 0 {
				t.Errorf("result[%d].Num = %d, must be >= 0", i, li.Num)
			}
		}
	})
}

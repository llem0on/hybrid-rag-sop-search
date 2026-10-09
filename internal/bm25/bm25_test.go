package bm25

import "testing"

func TestStemmingMatchesWordForms(t *testing.T) {
	ix := Build([]int64{1, 2}, []string{"prosedur pencadangan data harian", "kebijakan kata sandi"})
	scores, coverage := ix.Search("cadangan data", 10)
	if scores[1] == 0 || scores[2] != 0 {
		t.Fatalf("stemmed match failed: %v", scores)
	}
	if coverage[1] != 1 {
		t.Fatalf("want full coverage, got %v", coverage[1])
	}
}

func TestDottedNumbersStayIntact(t *testing.T) {
	ix := Build([]int64{1, 2}, []string{"sesuai klausul a.8.23 tentang web filtering", "klausul a.8.24"})
	scores, _ := ix.Search("a.8.23", 10)
	if scores[1] == 0 || scores[2] != 0 {
		t.Fatalf("clause number not matched exactly: %v", scores)
	}
}

func TestCoverageCountsDistinctTerms(t *testing.T) {
	ix := Build([]int64{1}, []string{"server server server backup"})
	_, coverage := ix.Search("server firewall", 10)
	if coverage[1] != 0.5 {
		t.Fatalf("want coverage 0.5, got %v", coverage[1])
	}
}

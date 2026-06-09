package semconv

import "testing"

func TestSearchTextMatch(t *testing.T) {
	reg := mustLoad(t)
	res := reg.Search(Query{Text: "http.request.method", Limit: 100})
	if res.Total == 0 {
		t.Fatal("expected matches for http.request.method")
	}
	for _, it := range res.Items {
		if !containsFold(it.Name, "http.request.method") && !containsFold(it.Brief, "http.request.method") {
			t.Errorf("unexpected item in results: %s", it.Name)
		}
	}
}

func TestSearchTypeFacetFilter(t *testing.T) {
	reg := mustLoad(t)
	res := reg.Search(Query{Types: []ItemType{ItemMetric}, Limit: 5000})
	if res.Total == 0 {
		t.Fatal("expected metric items")
	}
	for _, it := range res.Items {
		if it.Type != ItemMetric {
			t.Fatalf("got non-metric item %s (%s)", it.Name, it.Type)
		}
	}
}

func TestSearchStabilityFilter(t *testing.T) {
	reg := mustLoad(t)
	res := reg.Search(Query{Stability: []string{"stable"}, Limit: 5000})
	if res.Total == 0 {
		t.Fatal("expected stable items")
	}
	for _, it := range res.Items {
		if it.Stability != "stable" {
			t.Fatalf("non-stable item %s: %s", it.Name, it.Stability)
		}
	}
}

func TestSearchNamespaceFilter(t *testing.T) {
	reg := mustLoad(t)
	res := reg.Search(Query{Namespace: "http", Limit: 5000})
	if res.Total == 0 {
		t.Fatal("expected http namespace items")
	}
	for _, it := range res.Items {
		if it.Namespace != "http" {
			t.Fatalf("item %s namespace %s != http", it.Name, it.Namespace)
		}
	}
}

func TestSearchPagination(t *testing.T) {
	reg := mustLoad(t)
	page1 := reg.Search(Query{Limit: 10, Offset: 0})
	page2 := reg.Search(Query{Limit: 10, Offset: 10})
	if page1.Total != page2.Total {
		t.Fatal("totals differ across pages")
	}
	if len(page1.Items) != 10 || len(page2.Items) != 10 {
		t.Fatalf("page sizes: %d, %d", len(page1.Items), len(page2.Items))
	}
	if page1.Items[0].Key == page2.Items[0].Key {
		t.Error("pages overlap")
	}
}

func TestFacetsComputedBeforeSelection(t *testing.T) {
	reg := mustLoad(t)
	// Selecting a single type must not zero out the other type facet counts,
	// because facets are computed over the text-matched set pre-selection.
	res := reg.Search(Query{Types: []ItemType{ItemMetric}, Limit: 1})
	var sawAttribute bool
	for _, fc := range res.Facets.Type {
		if fc.Value == string(ItemAttribute) && fc.Count > 0 {
			sawAttribute = true
		}
	}
	if !sawAttribute {
		t.Error("expected attribute facet count to survive a metric-only selection")
	}
}

func TestSearchOffsetBeyondTotal(t *testing.T) {
	reg := mustLoad(t)
	res := reg.Search(Query{Offset: 1 << 30, Limit: 10})
	if len(res.Items) != 0 {
		t.Errorf("expected empty page, got %d", len(res.Items))
	}
}

func containsFold(s, sub string) bool {
	return len(sub) == 0 || indexFold(s, sub) >= 0
}

func indexFold(s, sub string) int {
	ls, lsub := toLower(s), toLower(sub)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		if ls[i:i+len(lsub)] == lsub {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

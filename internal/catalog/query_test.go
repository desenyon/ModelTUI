package catalog

import (
	"reflect"
	"testing"
)

func queryFixture() *Index {
	return BuildIndex(&Catalog{Models: map[string]CanonicalModel{}, Providers: map[string]Provider{
		"b": {ID: "b", Name: "Same", Models: map[string]OfferingModel{"unknown": {ID: "unknown", Name: "Model", Limit: Limit{Context: 1000}}}},
		"a": {ID: "a", Name: "Same", Models: map[string]OfferingModel{
			"paid": {ID: "paid", Name: "Model", Reasoning: true, ToolCall: true, Limit: Limit{Context: 2000}, Cost: &Cost{Input: 2, Output: 1}},
			"free": {ID: "free", Name: "Model", Reasoning: true, Limit: Limit{Context: 500}, Cost: &Cost{}},
		}},
	}}, "test")
}

func TestQueryFiltersIntersectAndRejectInvalidOptions(t *testing.T) {
	q := Query{Provider: "a", Search: "PAID", Capabilities: []string{"reasoning", "tools"}, MinContext: 1500, Sort: "context"}
	got, err := queryFixture().QueryOfferings(q)
	if err != nil || len(got) != 1 || got[0].Model.ID != "paid" {
		t.Fatalf("query result=%+v err=%v", got, err)
	}
	for _, q := range []Query{{Sort: "typo"}, {Capabilities: []string{"typo"}}, {MinContext: -1}, {Limit: -1}} {
		if _, err := queryFixture().QueryOfferings(q); err == nil {
			t.Fatalf("accepted invalid query: %+v", q)
		}
	}
	got, err = queryFixture().QueryOfferings(Query{Provider: "missing"})
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty query should return []: %v %v", got, err)
	}
}

func TestQueryPriceOrderingUnknownLastAndStable(t *testing.T) {
	for _, sortBy := range []string{"input-price", "output-price"} {
		for range 10 {
			got, err := queryFixture().QueryOfferings(Query{Sort: sortBy})
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{got[0].Model.ID, got[1].Model.ID, got[2].Model.ID}
			if !reflect.DeepEqual(ids, []string{"free", "paid", "unknown"}) {
				t.Fatalf("%s: %v", sortBy, ids)
			}
		}
	}
	got, _ := queryFixture().QueryOfferings(Query{Sort: "context", Limit: 1})
	if len(got) != 1 || got[0].Model.ID != "paid" {
		t.Fatalf("limit/context: %v", got)
	}
}

func TestCapabilityFreeExcludesMissingPrices(t *testing.T) {
	got, err := queryFixture().QueryOfferings(Query{Capabilities: []string{"free"}})
	if err != nil || len(got) != 1 || got[0].Model.ID != "free" {
		t.Fatalf("free: %v %v", got, err)
	}
}

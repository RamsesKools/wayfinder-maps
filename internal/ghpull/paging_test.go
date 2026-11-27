package ghpull

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type object = map[string]any

// fixtures copies the full scenario into a temp dir the test can re-page.
func fixtures(t *testing.T) *fakeGH {
	t.Helper()
	f := newFakeGH(t, "full")
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(f.dir)); err != nil {
		t.Fatal(err)
	}
	f.dir = dir
	return f
}

func (f *fakeGH) load(name string) object {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(f.dir, name))
	var v object
	if err == nil {
		err = json.Unmarshal(b, &v)
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return v
}

func (f *fakeGH) save(name string, v any) {
	f.t.Helper()
	b, err := json.Marshal(v)
	if err == nil {
		err = os.WriteFile(filepath.Join(f.dir, name), b, 0o644)
	}
	if err != nil {
		f.t.Fatal(err)
	}
}

func at(v any, path ...string) object {
	for _, k := range path {
		v = v.(object)[k]
	}
	return v.(object)
}

// chunks cuts nodes into pages of the given sizes; the last takes the rest.
func chunks(nodes []any, sizes ...int) [][]any {
	var pages [][]any
	for _, n := range sizes {
		pages, nodes = append(pages, nodes[:n]), nodes[n:]
	}
	return append(pages, nodes)
}

func setPage(c object, pages [][]any, i int) {
	c["nodes"] = pages[i]
	c["pageInfo"] = object{"hasNextPage": i < len(pages)-1, "endCursor": fmt.Sprintf("c%d", i+1)}
}

// pageSubIssues splits the map's sub-issues into pages of the given sizes.
func (f *fakeGH) pageSubIssues(sizes ...int) {
	doc := f.load("map__10.json")
	subs := at(doc, "data", "repository", "issue", "subIssues")
	pages := chunks(subs["nodes"].([]any), sizes...)
	for i := range pages {
		setPage(subs, pages, i)
		name := "map__10.json"
		if i > 0 {
			name = fmt.Sprintf("map__10__c%d.json", i)
		}
		f.save(name, doc)
	}
}

func pullFull(t *testing.T, f *fakeGH) {
	t.Helper()
	p, stdout, stderr := f.puller(nil)
	out := filepath.Join(t.TempDir(), "out")
	if err := p.Pull(Options{Repo: "acme/widgets", Map: 10, Out: out}); err != nil {
		t.Fatal(err)
	}
	assertTree(t, out, filepath.Join("testdata", "full", "golden"))
	if got, want := strings.ReplaceAll(stdout.String(), out, "OUT"), golden(t, "full", "stdout.golden"); got != want {
		t.Errorf("stdout\n--- got\n%s\n--- want\n%s", got, want)
	}
	if got, want := stderr.String(), golden(t, "full", "stderr.golden"); got != want {
		t.Errorf("stderr\n--- got\n%s\n--- want\n%s", got, want)
	}
}

func TestPagedSubIssuesKeepOrder(t *testing.T) {
	f := fixtures(t)
	f.pageSubIssues(3, 3, 3)
	pullFull(t, f)
}

func TestProgressAdvancesOncePerBatch(t *testing.T) {
	f := fixtures(t)
	f.pageSubIssues(4, 4)
	p, _, _ := f.puller(nil)
	var got [][2]int
	err := p.Pull(Options{Repo: "acme/widgets", Map: 10, Out: filepath.Join(t.TempDir(), "out"),
		Progress: func(d, n int) { got = append(got, [2]int{d, n}) }})
	if err != nil {
		t.Fatal(err)
	}
	if want := [][2]int{{0, 1}, {5, 11}, {9, 11}, {11, 11}}; !slices.Equal(got, want) {
		t.Errorf("progress = %v, want %v", got, want)
	}
}

// pageConn splits one ticket's connection into pages of the given sizes.
func (f *fakeGH) pageConn(number int, field string, sizes ...int) {
	f.t.Helper()
	doc := f.load("map__10.json")
	for _, n := range at(doc, "data", "repository", "issue", "subIssues")["nodes"].([]any) {
		ticket := n.(object)
		if ticket["number"] != float64(number) {
			continue
		}
		c := at(ticket, field)
		pages := chunks(c["nodes"].([]any), sizes...)
		for i := 1; i < len(pages); i++ {
			more := object{}
			setPage(more, pages, i)
			f.save(fmt.Sprintf("more__%s__I_%d__c%d.json", field, number, i), object{"data": object{"node": object{"page": more}}})
		}
		setPage(c, pages, 0)
		f.save("map__10.json", doc)
		return
	}
	f.t.Fatalf("no ticket #%d", number)
}

func TestOverflowingConnectionsArePaged(t *testing.T) {
	for _, tc := range []struct {
		name   string
		number int
		field  string
		sizes  []int
	}{
		{"comments", 11, "comments", []int{0, 1}},
		{"blockers", 14, "blockedBy", []int{1}},
		// #14 is a task on its second page of labels, #16 an implement ticket.
		{"labels", 14, "labels", []int{0, 1}},
		{"labels before PRs", 16, "labels", []int{0}},
		// The merged PR, and the warning about two PRs, need the second page.
		{"linked PRs", 15, "closedByPullRequestsReferences", []int{1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fixtures(t)
			f.pageConn(tc.number, tc.field, tc.sizes...)
			pullFull(t, f)
		})
	}
}

func TestUnusedOverflowIsNotFetched(t *testing.T) {
	f := fixtures(t)
	f.pageConn(12, "comments", 0)                       // open
	f.pageConn(11, "closedByPullRequestsReferences", 0) // not an implement ticket
	pullFull(t, f)
	for _, c := range f.calls {
		if slices.ContainsFunc(c.args, func(a string) bool { return strings.HasPrefix(a, "id=") }) {
			t.Errorf("fetched a page the mirror does not use: %v", c.args)
		}
	}
}

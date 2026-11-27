package ghpull

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RamsesKools/wayfinder-maps/internal/wayfinder"
)

// setComment replaces a ticket's comments with one.
func (f *fakeGH) setComment(number int, body string) {
	f.t.Helper()
	doc := f.load("map__10.json")
	for _, n := range at(doc, "data", "repository", "issue", "subIssues")["nodes"].([]any) {
		if ticket := n.(object); ticket["number"] == float64(number) {
			at(ticket, "comments")["nodes"] = []any{object{"body": body}}
			f.save("map__10.json", doc)
			return
		}
	}
	f.t.Fatalf("no ticket #%d", number)
}

// pullOne pulls the full scenario and reads one ticket back as the viewer would.
func pullOne(t *testing.T, f *fakeGH, name string) (*wayfinder.Ticket, string) {
	t.Helper()
	p, _, _ := f.puller(nil)
	out := filepath.Join(t.TempDir(), "out")
	if err := p.Pull(Options{Repo: "acme/widgets", Map: 10, Out: out}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "tickets", name))
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := wayfinder.ParseTicket(name, name, string(b))
	if err != nil {
		t.Fatal(err)
	}
	return ticket, string(b)
}

func TestCommentOpeningWithTopHeadingStillCloses(t *testing.T) {
	for _, tc := range []struct {
		number int
		file   string
		want   wayfinder.Status
	}{
		{11, "11-should-invoices-be-immutable.md", wayfinder.StatusResolved},
		{13, "13-prototype-a-credit-note-flow.md", wayfinder.StatusOutOfScope},
	} {
		f := fixtures(t)
		f.setComment(tc.number, "# Report\r\n\r\nDetails.\r\n")
		if ticket, text := pullOne(t, f, tc.file); ticket.Status != tc.want {
			t.Errorf("#%d is %s, want %s:\n%s", tc.number, ticket.Status, tc.want, text)
		}
	}
}

func TestCommentHeadingsKeepTheirHierarchy(t *testing.T) {
	f := fixtures(t)
	f.setComment(11, "# One\n## Two\n##### Five\n###### Six")
	_, text := pullOne(t, f, "11-should-invoices-be-immutable.md")
	if want := "## Answer\n\n### One\n#### Two\n###### Five\n###### Six\n"; !strings.HasSuffix(text, want) {
		t.Errorf("want it to end with %q:\n%s", want, text)
	}
}

func TestCommentCodeBlocksAreLeftAlone(t *testing.T) {
	f := fixtures(t)
	// The shell comment is not a heading, so "## Two" moves down only one level.
	f.setComment(11, "## Two\n\n```bash\n# run it\n## twice\n```\n\n~~~\n# tilde\n~~~")
	_, text := pullOne(t, f, "11-should-invoices-be-immutable.md")
	if want := "## Answer\n\n### Two\n\n```bash\n# run it\n## twice\n```\n\n~~~\n# tilde\n~~~\n"; !strings.HasSuffix(text, want) {
		t.Errorf("want it to end with %q:\n%s", want, text)
	}
}

package ghpull

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Kind classifies why a gh call failed.
type Kind int

const (
	KindOther     Kind = iota
	KindMissing        // gh is not installed or not on PATH
	KindNoAuth         // gh has no credentials at all
	KindForbidden      // HTTP 401, 403 or 404: bad credentials or no access
)

// GHError is a failed gh call. Runners return it so Pull can explain the
// failure in terms the user can act on.
type GHError struct {
	Kind     Kind
	Status   int    // HTTP status, when gh reported one
	Endpoint string // the API path or "graphql"
	Detail   string // gh's own message, trimmed
}

func (e *GHError) Error() string {
	if e.Detail == "" {
		return "gh " + e.Endpoint + " failed"
	}
	return "gh " + e.Endpoint + ": " + e.Detail
}

var httpStatusRE = regexp.MustCompile(`\(HTTP (\d{3})\)`)

// classify builds a GHError from a failed gh run.
func classify(args []string, runErr error, stderr string) *GHError {
	e := &GHError{Endpoint: endpointOf(args), Detail: strings.TrimSpace(stderr)}
	var exit *exec.ExitError
	switch {
	case errors.Is(runErr, exec.ErrNotFound):
		e.Kind, e.Detail = KindMissing, ""
	case e.Detail == "":
		e.Detail = runErr.Error()
	}
	if m := httpStatusRE.FindStringSubmatch(e.Detail); m != nil {
		e.Status, _ = strconv.Atoi(m[1])
	}
	switch {
	case e.Kind == KindMissing:
	case strings.Contains(e.Detail, "gh auth login") || errors.As(runErr, &exit) && exit.ExitCode() == 4:
		e.Kind = KindNoAuth
	case e.Status == 401 || e.Status == 403 || e.Status == 404:
		e.Kind = KindForbidden
	}
	return e
}

// endpointOf picks the API path out of gh arguments.
func endpointOf(args []string) string {
	if len(args) > 0 && args[0] == "api" {
		for i := 1; i < len(args); i++ {
			switch a := args[i]; {
			case a == "-f" || a == "-F" || a == "--jq":
				i++
			case !strings.HasPrefix(a, "-"):
				return a
			}
		}
	}
	return strings.Join(args[:min(len(args), 2)], " ")
}

// explain turns a gh failure into a message that says what to do next. who
// names the credentials in use, e.g. "gh account 'alice'".
func explain(err error, who string) error {
	var ge *GHError
	if !errors.As(err, &ge) {
		return err
	}
	switch ge.Kind {
	case KindMissing:
		return userErr("gh is not installed or not on PATH. Install it from https://cli.github.com, then run: gh auth login")
	case KindNoAuth:
		return userErr("gh is not logged in. Run: gh auth login (or set GH_TOKEN)")
	case KindForbidden:
		switch ge.Status {
		case 401:
			return userErr("GitHub rejected %s (HTTP 401). The login may have expired; check with: gh auth status", who)
		default:
			return userErr("cannot read %s as %s (HTTP %d). Check the repo name and issue number, and that this account can see the repo; check with: gh auth status", ge.Endpoint, who, ge.Status)
		}
	}
	return fmt.Errorf("%v", ge)
}

// Explain is explain for callers outside Pull; who names the credentials, e.g.
// "gh account 'alice'", or "" for the active account.
func Explain(err error, who string) error {
	if who == "" {
		who = "the active gh account"
	}
	return explain(err, who)
}

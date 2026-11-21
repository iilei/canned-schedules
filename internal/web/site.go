package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/iilei/canned-schedules/internal/cli"
	"github.com/iilei/canned-schedules/internal/diag"
	"github.com/iilei/canned-schedules/schema"
)

// Defaults of site.json.
const (
	defaultCatalogDir = "schedules"
	defaultMaxLength  = 8000
)

// placeholder matches {word} in a URL template.
var placeholder = regexp.MustCompile(`\{[^{}]*\}`)

// atLocation matches the location jsonschema puts before its messages.
var atLocation = regexp.MustCompile(`^at '[^']*': `)

// placeholders are the ones the page fills in.
var placeholders = []string{"{path}", "{content}", "{name}"}

// Site is a self-hosted playground's site.json, checked and with its defaults filled in.
type Site struct {
	// Propose is nil when the site sets no proposal targets.
	Propose *Propose `json:"propose,omitempty"`
	Name    string   `json:"name,omitempty"`
	// Args are the site's flags; the page passes them first, like a locked link's.
	Args    []string `json:"args"`
	Catalog Catalog  `json:"catalog"`
}

// Catalog is where the catalog's schedules live and which exist.
type Catalog struct {
	Dir   string   `json:"dir"`
	Names []string `json:"names"`
}

// Propose holds the URL templates for proposing a valid schedule; an empty template means no such target.
type Propose struct {
	Update    *Target `json:"update,omitempty"`
	Create    string  `json:"create,omitempty"`
	Issue     string  `json:"issue,omitempty"`
	Fork      string  `json:"fork,omitempty"`
	MaxLength int     `json:"max-length"`
}

// Target is a URL template and whether the page fills {content} into it (otherwise it copies the schedule).
type Target struct {
	URL     string `json:"url"`
	Prefill bool   `json:"prefill"`
}

// SiteResult is what CheckSite returns to the page: the site, or why it can't be used.
type SiteResult struct {
	Site   *Site    `json:"site"`
	Errors []string `json:"errors"`
}

// Encode returns the result as a JSON string for JavaScript.
func (r *SiteResult) Encode() string { return encode(r) }

// CheckSite checks a site.json against its schema, its URL templates and its flags (the checks of a link's ?arg=,
// then the CLI's parser). A site with any error is not used at all: a half-applied setup would check schedules
// differently from CI without saying so.
func CheckSite(raw []byte) SiteResult {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return SiteResult{Errors: []string{"site.json is not JSON: " + diag.Sanitize(err.Error())}}
	}

	if errs := validateSite(doc); len(errs) > 0 {
		return SiteResult{Errors: errs}
	}

	var in struct {
		Propose *struct {
			Create    string          `json:"create"`
			Issue     string          `json:"issue"`
			Fork      string          `json:"fork"`
			Update    json.RawMessage `json:"update"`
			MaxLength int             `json:"max-length"`
		} `json:"propose"`
		Name    string   `json:"name"`
		Args    []string `json:"args"`
		Catalog Catalog  `json:"catalog"`
	}

	if err := json.Unmarshal(raw, &in); err != nil {
		return SiteResult{Errors: []string{"site.json: " + diag.Sanitize(err.Error())}}
	}

	site := &Site{Name: in.Name, Args: slices.Clip(in.Args), Catalog: in.Catalog}
	if site.Args == nil {
		site.Args = []string{}
	}

	if site.Catalog.Dir == "" {
		site.Catalog.Dir = defaultCatalogDir
	}

	if site.Catalog.Names == nil {
		site.Catalog.Names = []string{}
	}

	var errs []string

	if slices.Contains(strings.Split(site.Catalog.Dir, "/"), "..") {
		errs = append(errs, "catalog.dir: .. is not allowed")
	}

	errs = append(errs, checkSiteArgs(site.Args)...)

	if in.Propose != nil {
		p, more := propose(in.Propose.Create, in.Propose.Issue, in.Propose.Update, in.Propose.MaxLength)
		site.Propose = p

		if p.Fork = in.Propose.Fork; p.Fork != "" {
			more = append(more, checkTemplate("propose.fork", p.Fork)...)

			if placeholder.MatchString(p.Fork) {
				more = append(more, "propose.fork: a plain URL, without placeholders")
			}
		}
		errs = append(errs, more...)
	}

	if len(errs) > 0 {
		return SiteResult{Errors: errs}
	}

	return SiteResult{Site: site, Errors: []string{}}
}

func validateSite(doc any) []string {
	sch, err := schema.Compile(schema.SiteSchemaURL, schema.SiteSchema)
	if err != nil {
		return []string{"site schema: " + err.Error()}
	}

	err = sch.Validate(doc)

	var ve *jsonschema.ValidationError
	if err == nil || !errors.As(err, &ve) {
		return nil
	}

	var errs []string

	for _, leaf := range leaves(ve) {
		where := "site.json"
		if len(leaf.InstanceLocation) > 0 {
			where = strings.Join(leaf.InstanceLocation, ".")
		}

		// jsonschema repeats the location ("at '/propose/create': …"); where already says it.
		msg := atLocation.ReplaceAllString(leaf.Error(), "")
		errs = append(errs, where+": "+diag.Sanitize(msg))
	}

	return errs
}

func leaves(ve *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(ve.Causes) == 0 {
		return []*jsonschema.ValidationError{ve}
	}

	var out []*jsonschema.ValidationError
	for _, c := range ve.Causes {
		out = append(out, leaves(c)...)
	}

	return out
}

// checkSiteArgs applies a link's rules to the site's flags (the same flags, for the same reason: the page has
// controls for the others), then lets the CLI's parser check their values.
func checkSiteArgs(args []string) []string {
	diags := checkLink(args)
	if !diags.HasErrors() {
		_, diags = cli.Parse(args)
	}

	var errs []string

	for _, d := range diags {
		if d.Severity == diag.Error {
			errs = append(
				errs,
				"args: "+strings.Replace(d.Message, "from a link: ?arg= takes", "from site.json: args take", 1),
			)
		}
	}

	return errs
}

func propose(create, issue string, rawUpdate json.RawMessage, maxLength int) (*Propose, []string) {
	p := &Propose{Create: create, Issue: issue, MaxLength: maxLength}
	if p.MaxLength == 0 {
		p.MaxLength = defaultMaxLength
	}

	if len(rawUpdate) > 0 {
		p.Update = &Target{Prefill: true}

		// The schema allows a string or {url, prefill}.
		if err := json.Unmarshal(rawUpdate, &p.Update.URL); err != nil {
			var obj struct {
				Prefill *bool  `json:"prefill"`
				URL     string `json:"url"`
			}

			_ = json.Unmarshal(rawUpdate, &obj)
			p.Update.URL = obj.URL

			if obj.Prefill != nil {
				p.Update.Prefill = *obj.Prefill
			}
		}
	}

	var errs []string

	if create != "" {
		errs = append(errs, checkTemplate("propose.create", create, "{path}", "{content}")...)
	}

	if issue != "" {
		errs = append(errs, checkTemplate("propose.issue", issue, "{content}")...)
	}

	if p.Update != nil {
		need := []string{"{path}"}
		if p.Update.Prefill {
			need = append(need, "{content}")
		}

		errs = append(errs, checkTemplate("propose.update", p.Update.URL, need...)...)
	}

	return p, errs
}

// checkTemplate checks a URL template: only the known placeholders, the required ones present, and with them filled
// in an https URL with a host and without user info or a fragment.
func checkTemplate(where, tmpl string, need ...string) []string {
	var errs []string

	for _, ph := range placeholder.FindAllString(tmpl, -1) {
		if !slices.Contains(placeholders, ph) {
			errs = append(errs, fmt.Sprintf("%s: unknown placeholder %s; use {path}, {content} or {name}", where,
				diag.Sanitize(ph)))
		}
	}

	for _, ph := range need {
		if !strings.Contains(tmpl, ph) {
			errs = append(errs, fmt.Sprintf("%s: needs %s", where, ph))
		}
	}

	u, err := url.Parse(placeholder.ReplaceAllString(tmpl, "x"))

	switch {
	case err != nil:
		errs = append(errs, where+": not a URL")
	case u.Scheme != "https" || u.Host == "":
		errs = append(errs, where+": must be an https URL with a host")
	case u.User != nil:
		errs = append(errs, where+": must not contain user info")
	case u.Fragment != "" || strings.Contains(tmpl, "#"):
		errs = append(errs, where+": must not contain a fragment")
	}

	return errs
}

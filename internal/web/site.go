package web

import "github.com/iilei/canned-schedules/internal/site"

// SiteResult is what CheckSite returns to the page: the site, or why it can't be used.
type SiteResult struct {
	Site   *site.Site `json:"site"`
	Errors []string   `json:"errors"`
}

// Encode returns the result as a JSON string for JavaScript.
func (r *SiteResult) Encode() string { return encode(r) }

// CheckSite checks a self-hosted playground's site.json for the page (cannedSchedules.checkSite); see site.Check.
// Errors is an empty list rather than null, for JavaScript.
func CheckSite(raw []byte) SiteResult {
	s, errs := site.Check(raw)
	if errs == nil {
		errs = []string{}
	}

	return SiteResult{Site: s, Errors: errs}
}

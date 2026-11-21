package cannedschedules

import "github.com/iilei/canned-schedules/internal/site"

// CheckSite checks a self-hosted playground's site.json (schema/site.schema.json): the schema, the URL templates of
// its proposals and its flags, which must be ones a link may set. It returns the problems, empty when the page will
// use the file; the page ignores a site.json with any problem as a whole. Tools that write site.json, such as the
// companion's playground build, check it before writing.
func CheckSite(siteJSON []byte) []string {
	_, problems := site.Check(siteJSON)

	return problems
}

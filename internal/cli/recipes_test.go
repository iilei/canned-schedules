package cli

import (
	"regexp"
	"testing"
)

const (
	awsDocs    = "../../docs/aws.md"
	awsExample = "../../examples/aws/"
)

var fencedBlock = regexp.MustCompile("(?s)```(hcl|json|bash)\n(.*?)```")

// TestAWSRecipesMatchTheExample keeps the recipes in docs/aws.md identical to examples/aws, which mise run example:aws tests.
func TestAWSRecipesMatchTheExample(t *testing.T) {
	t.Parallel()

	section := readFile(t, awsDocs)

	blocks := map[string][]string{}
	for _, m := range fencedBlock.FindAllStringSubmatch(section, -1) {
		blocks[m[1]] = append(blocks[m[1]], m[2])
	}

	if len(blocks["hcl"]) != 2 || len(blocks["json"]) != 1 || len(blocks["bash"]) != 1 {
		t.Fatalf("unexpected recipes in docs/aws.md: %d hcl, %d json, %d bash",
			len(blocks["hcl"]), len(blocks["json"]), len(blocks["bash"]))
	}

	for file, want := range map[string]string{
		"terraform/ec2.tf":      blocks["hcl"][0],
		"terraform/ecs.tf":      blocks["hcl"][1],
		"capacity.json":         blocks["json"][0],
		"cli/apply-schedule.sh": "#!/usr/bin/env bash\n" + blocks["bash"][0],
	} {
		if readFile(t, awsExample+file) != want {
			t.Errorf("examples/aws/%s differs from the recipe in docs/aws.md", file)
		}
	}
}

// TestAWSSchedulesAreTheTemplates keeps the example's schedules identical to the issue templates they come from.
func TestAWSSchedulesAreTheTemplates(t *testing.T) {
	t.Parallel()

	for schedule, template := range map[string]string{
		"friday-peak": "canned_schedule_friday_peak.md",
		"night-shift": "canned_schedule_night_shift.md",
	} {
		_, body, ok := splitFrontmatter(readFile(t, githubTemplates+template))
		if !ok || readFile(t, awsExample+"schedules/"+schedule+".canned-schedule.md") != body {
			t.Errorf("examples/aws/schedules/%s differs from %s", schedule, template)
		}
	}
}

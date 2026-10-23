def is_admonition:
  type == "object"
  and (.GitHubAlert? | type == "object");

def alert_type:
  if (.GitHubAlert.alert_type | type) == "string" then
    .GitHubAlert.alert_type
  elif (.GitHubAlert.alert_type.Custom? | type) == "string" then
    .GitHubAlert.alert_type.Custom
  else
    ""
  end;

def is_base:
  (alert_type | ascii_upcase) == "BASE";

def trim:
  gsub("^\\s+|\\s+$"; "");

def admonition_body:
  if (.GitHubAlert.blocks | type) == "array" then
    .GitHubAlert.blocks
  elif (.GitHubAlert.content | type) == "array" then
    .GitHubAlert.content
  else
    []
  end;

# Match only a single paragraph containing a single inline-code node.
def has_single_code_expression:
  (admonition_body | length) == 1
  and (admonition_body[0] | type == "object")
  and ((admonition_body[0] | keys) == ["Paragraph"])
  and (admonition_body[0].Paragraph | type == "array")
  and (admonition_body[0].Paragraph | length) == 1
  and (admonition_body[0].Paragraph[0] | type == "object")
  and ((admonition_body[0].Paragraph[0] | keys) == ["Code"])
  and (admonition_body[0].Paragraph[0].Code | type == "string");

def schedule_expression:
  admonition_body[0].Paragraph[0].Code | trim;

def validate_schedule_alert($pattern):
  schedule_expression as $expr
  | if ($expr | length) == 0 then
      error("Empty schedule expression for tier '\(alert_type)'")
    elif ($expr | test($pattern)) then
      {tier: alert_type, expression: $expr}
    else
      error("Invalid schedule expression for tier '\(alert_type)': \($expr)")
    end;

def normalize_pattern:
  trim;

.blocks
| if type != "array" then
    error("Expected Markdown AST object with a .blocks array")
  else
    [
      .[]
      | select(is_admonition)
      | select((is_base | not) and has_single_code_expression)
      | validate_schedule_alert($pattern | normalize_pattern)
    ]
  end
| if length == 0 then
    error("No schedule admonitions found")
  else
    .
  end

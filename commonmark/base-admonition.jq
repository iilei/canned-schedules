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
  is_admonition
  and ((alert_type | ascii_upcase) == "BASE");

def base_tier:
  if (.GitHubAlert.blocks | type) == "array" then
    .GitHubAlert.blocks
  elif (.GitHubAlert.content | type) == "array" then
    .GitHubAlert.content
  else
    []
  end;

def valid_base_body:
  (base_tier | length) == 1
  and (base_tier[0] | type == "object")
  and ((base_tier[0] | keys) == ["Paragraph"])
  and (base_tier[0].Paragraph | type == "array")
  and (base_tier[0].Paragraph | length) == 1
  and (base_tier[0].Paragraph[0] | type == "object")
  and ((base_tier[0].Paragraph[0] | keys) == ["Code"])
  and (base_tier[0].Paragraph[0].Code | type == "string")
  and (base_tier[0].Paragraph[0].Code | gsub("^\\s+|\\s+$"; "") | length > 0);

.blocks
| if type != "array" then
    error("Expected Markdown AST object with a .blocks array")
  else
    [.[] | select(is_base)] as $bases
    | if ($bases | length) != 1 then
        error("Expected exactly one [!Base] admonition; found \($bases | length)")
      elif ($bases[0] | valid_base_body) then
        {
          base: (
            $bases[0]
            | base_tier[0].Paragraph[0].Code
            | gsub("^\\s+|\\s+$"; "")
          )
        }
      else
        error("[!Base] must contain exactly one paragraph with one inline-code tier name")
      end
  end

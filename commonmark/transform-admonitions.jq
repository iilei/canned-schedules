# Detect: is this inline a custom alert marker (LinkReference with "!..." label)?
def is_alert_marker:
  (keys[0] == "LinkReference") and
  (.LinkReference.label | map(.Text) | join("") | startswith("!"));

# Extract the alert type (strip leading "!")
def alert_type:
  .LinkReference.label | map(.Text) | join("") | ltrimstr("!");

# Transform a BlockQuote array (that is actually a custom admonition) into a GitHubAlert
def transform_admonition($bq):
  ($bq[0].Paragraph[0] | alert_type) as $type |
  ($bq[0].Paragraph) as $para |
  # Strip the LinkReference (idx 0) and the "\n" separator (idx 1)
  ($para[2:]) as $rest |
  # Rebuild content: keep modified paragraph (if non-empty) + remaining blocks
  (if ($rest | length) == 0 then $bq[1:]
   else [{"Paragraph": $rest}] + $bq[1:]
   end) as $content |
  {"GitHubAlert": {"alert_type": $type, "content": $content}};

# Recursively walk the AST, transforming matching BlockQuotes
def walk:
  if type == "array" then
    map(walk)
  elif type == "object" then
    if has("BlockQuote") and
       (.BlockQuote | length > 0) and
       (.BlockQuote[0] | keys[0] == "Paragraph") and
       (.BlockQuote[0].Paragraph | length > 0) and
       (.BlockQuote[0].Paragraph[0] | is_alert_marker) then
      transform_admonition(.BlockQuote)
    else
      with_entries(.value |= walk)
    end
  else
    .
  end;

walk

[range(0; (.blocks | length)) as $i
 | .blocks[$i] as $cur
 | select($cur | has("Paragraph"))
 | select($cur.Paragraph | any(has("Strong")))
 | # nearest preceding Heading
   ([range($i - 1; -1; -1) as $j
    | .blocks[$j]
    | select(has("Heading"))
   ] | first) as $heading
 | # next List (bounded by next Heading)
   ([range($i+1; (.blocks | length)) as $j
    | .blocks[$j]
    | select(has("List") or has("Heading"))
   ] | first) as $next
 | select($next | has("List"))
 | {
     section: ($heading.Heading.content[0].Text),
     bold:    ($cur.Paragraph | map(.Strong[0].Text) | join("")),
     items:   ($next.List.items
               | map(select(.task == "Complete")
                     | .blocks[0].Paragraph[0].Text))
   }
]
| map(select(.items | length > 0))
| map({"tier": .section, "times": .items, "days": .bold})
| map( .tier |= gsub(":[a-zA-Z0-9_]+:\\s*"; "") )

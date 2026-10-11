# Capacity Schedules

## Configured Schedules

- `occassional_load_b`


## Weekly Schedule Drawings

Configured schedule patterns, rendered in 30-minute slots.

### occassional_load_b


~~~text

[occassional_load_b]
    00:00       06:00       12:00       18:00         
      ║     :     |     :     ║     :     |     :     
MON   ▒▒▒▒▒▒▒▒▒▒▓▓▓▓████████████████████▓▓▓▓▒▒▒▒▒▒▒▒▒▒
TUE   ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
WED   ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
THU   ▒▒▒▒▒▒▒▒▒▒▓▓▓▓████████████████████▓▓▓▓▒▒▒▒▒▒▒▒▒▒
FRI   ▒▒▒▒▒▒▒▒▒▒▓▓▓▓████████████████████▓▓▓▓▒▒▒▒▒▒▒▒▒▒
SAT   ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
SUN   ▒▒▒▒▒▒▒▒▒▒▓▓▓▓████████████████████▓▓▓▓▒▒▒▒▒▒▒▒▒▒
AWS Cron (UTC):
  0 5 ? * MON,THU,FRI,SUN *  Minimum -> Medium
  0 7 ? * MON,THU,FRI,SUN *  Medium -> Maximum
  0 17 ? * MON,THU,FRI,SUN *  Maximum -> Medium
  0 19 ? * MON,THU,FRI,SUN *  Medium -> Minimum
  0 0 ? * TUE,SAT *  Minimum -> Off
  0 0 ? * THU,SUN *  Off -> Minimum

Legend:
  ░ Off
  ▒ Minimum
  ▓ Medium
  █ Maximum

~~~




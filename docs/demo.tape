# Records docs/demo.gif. Run from the repo root with zombie-scanner on PATH:
#   vhs docs/demo.tape
Output docs/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1400
Set Height 310
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms

# run outside the repo so .zombie-scanner.yaml doesnot applies
Hide
Type `cd "$(mktemp -d)" && clear`
Enter
Show

Type "zombie-scanner --region ap-south-1"
Sleep 400ms
Enter
Sleep 7s

Type "zombie-scanner --region ap-south-1 --json | jq .summary.total_monthly_usd"
Sleep 400ms
Enter
Sleep 5s

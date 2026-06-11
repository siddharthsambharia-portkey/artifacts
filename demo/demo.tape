# VHS demo script — record with: vhs demo/demo.tape
Output demo/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"

Type "artifact init live-poll"
Enter
Sleep 1s
Type "cd live-poll"
Enter
Sleep 500ms
Type "artifact deploy --yes"
Enter
Sleep 2s
Type "artifact open --site live-poll"
Enter
Sleep 1s

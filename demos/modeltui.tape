Output assets/demo.gif
Output assets/screenshot-models.png

Set Shell zsh
Set FontSize 16
Set Width 1400
Set Height 860
Set Padding 24
Set Margin 28
Set MarginFill "#1a1218"
Set BorderRadius 14
Set WindowBar Colorful
Set WindowBarSize 36
Set Theme "Catppuccin Mocha"
Set TypingSpeed 0
Set Framerate 24
Set CursorBlink false

Env PATH "/Users/naitikgupta/Projects/ModelTUI/bin:$PATH"

Hide
Type "clear"
Enter
Show

Type "modeltui"
Enter
Sleep 3s
Screenshot assets/screenshot-models.png
Sleep 1s

Type "3"
Sleep 1.5s
Screenshot assets/screenshot-offerings.png
Sleep 500ms

Type "2"
Sleep 1.2s
Screenshot assets/screenshot-providers.png
Sleep 500ms

Down 4
Sleep 400ms
Enter
Sleep 800ms
Screenshot assets/screenshot-detail.png
Sleep 500ms

Type "q"
Sleep 500ms

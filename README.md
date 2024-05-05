A POC CS2 Demo Parser, uses an open source Go library and barebone HTML/CSS/JS for the client frontend for playback.

TODO:
- Equipment to use SVG's.
- Primary weapon SVG's mapped correctly.
- Bomb Planter/Timer
- Cleanup projective rendering.
- Molly/Inferno after thrown.
- ServerEventInfo is lackin, requires you to rewind to the first tick in ther replay to render map.
- Forward/Backward
- Pass demo path as cmdline arg.


How To Use:
- Install GoLang
- Add you demo files to the project folder.
- Modify `demoFile, err := os.Open("demo1.dem")` in `main.go` to your demo file name.
- `go run main.go`
- Open `index.html`
- Hit Play
- Note: The parser will not send events until client is opened and a connection is made.

module github.com/deprecatedluar/akeyshually

go 1.25.1

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/DeprecatedLuar/gohelp v0.0.0-00010101000000-000000000000
	github.com/fsnotify/fsnotify v1.9.0
	github.com/holoplot/go-evdev v0.0.0-20250804134636-ab1d56a1fe83
)

require (
	github.com/DeprecatedLuar/the-satellite/the-lib v0.0.0-20251112110819-0c6893dc3dc4 // indirect
	github.com/mattn/go-runewidth v0.0.12 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/term v0.36.0 // indirect
)

replace github.com/DeprecatedLuar/gohelp => ./pkg/gohelp

replace github.com/DeprecatedLuar/the-satellite/the-lib => ./pkg/the-lib

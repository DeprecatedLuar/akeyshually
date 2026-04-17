module github.com/deprecatedluar/akeyshually

go 1.25.1

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/DeprecatedLuar/gohelp-luar v0.2.0
	github.com/holoplot/go-evdev v0.0.0-20250804134636-ab1d56a1fe83
)

require (
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/term v0.36.0 // indirect
)

replace github.com/DeprecatedLuar/the-satellite/the-lib => ./pkg/the-lib

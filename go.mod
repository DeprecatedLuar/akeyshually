module github.com/deprecatedluar/akeyshually

go 1.25.5

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/DeprecatedLuar/gohelp-luar v0.2.2
	github.com/holoplot/go-evdev v0.0.0-20250804134636-ab1d56a1fe83
)

require (
	github.com/deprecatedluar/luar-daemonator v0.0.0-20260422105652-b0a90064bc50 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/term v0.36.0 // indirect
)

replace github.com/DeprecatedLuar/the-satellite/the-lib => ./pkg/the-lib

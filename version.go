package main

import "runtime/debug"

// Docker sets this from the checked-out Git revision. Native Go builds can use
// the VCS revision recorded automatically by the Go toolchain.
var buildCommit string

func commitID() string {
	revision := buildCommit
	if revision == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					revision = setting.Value
					break
				}
			}
		}
	}
	if len(revision) < 7 {
		return "dev"
	}
	for _, char := range revision {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return "dev"
		}
	}
	return revision[:7]
}

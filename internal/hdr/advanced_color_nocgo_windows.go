//go:build windows && !cgo

package hdr

func advancedColorState() (State, bool) { return Unknown, false }

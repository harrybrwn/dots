// Package generate is an empty package that serves as the root for running 'go generate'.
package generate

//go:generate go run ./cmd/generate-nerdfont -out pkg/nerdfonts/glyfs.gen.go -pkg nerdfonts
//go:generate go run ./cmd/gen --name=dots

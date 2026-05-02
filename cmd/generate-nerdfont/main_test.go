package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestConstName(t *testing.T) {
	if title("calendar") != "Calendar" {
		t.Error("title() is wrong")
	}
	exp := "NfMdCalendarWeekend"
	name := classToConstName("nf-md-calendar_weekend")
	if name != exp {
		t.Errorf("got %q, want %q", name, exp)
	}
}

func TestDownloadGlyphs(t *testing.T) {
	noCache = false
	glyphs, err := getGlyphs()
	if err != nil {
		t.Fatal(err)
	}
	if len(glyphs.Metadata.Date) == 0 {
		t.Error("glyph metadata should have a date")
	}
	if len(glyphs.Metadata.Version) == 0 {
		t.Error("glyphs metadata should have a version")
	}
	versionPrefix := "3.4."
	if !strings.HasPrefix(glyphs.Metadata.Version, versionPrefix) {
		t.Errorf("expected \"%s*\", got %q", versionPrefix, glyphs.Metadata.Version)
	}
	if len(glyphs.GlyphMap) == 0 {
		t.Error("glyphs unmarshalling should yield > 0 glyphs")
	}
}

func Test(t *testing.T) {
	t.Skip()
	icon := "󰞘" // nf-md-arrow_expand_right
	// icon := "" // nf-cod-account

	fmt.Printf("bytes: %d\nrunes: %d\n", len(icon), len([]rune(icon)))
	fmt.Println("icon:", icon)

	printarr("runes: %x", []rune(icon))
	printarr("bytes: %x", []byte(icon))
	printarr("utf16: %x", utf16.Encode([]rune(icon)))
}

func printarr[T any](format string, arr []T) {
	parts := strings.Split(format, ":")
	if len(parts) > 1 {
		for i := range parts {
			parts[i] = strings.Trim(parts[i], " ")
		}
		format = parts[1]
		fmt.Printf("%s: [", parts[0])
	} else {
		format = parts[0]
		fmt.Print("[")
	}
	for i, c := range arr {
		if i == len(arr)-1 {
			fmt.Printf(format, c)
		} else {
			fmt.Printf(format+" ", c)
		}
	}
	fmt.Println("]")
}

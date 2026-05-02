package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type Glyph struct {
	ID, Alias string
	Code      string
	ConstName string
	HexCode   uint32
	Icon      string
}

func glyphsList() (*GlyphsMetadata, []Glyph, error) {
	glyphs, err := getGlyphs()
	if err != nil {
		return nil, nil, err
	}
	cleanupGlyphs(glyphs)
	var (
		classes = make(HashSet[string])
		aliases = findAliases(glyphs.GlyphMap)
		result  = make([]Glyph, 0, len(glyphs.GlyphMap))
	)

	for id, glyph := range glyphs.GlyphMap {
		if classes.LoadPut(id) {
			return nil, nil, fmt.Errorf("%q is a duplicate class name", id)
		}
		iconHex, err := strconv.ParseUint(glyph.Code, 16, 32)
		if err != nil {
			return nil, nil, err
		}
		icon := string(rune(iconHex))
		if icon != glyph.Char {
			slog.Warn("parsed icon is different from given icon",
				"parsed", iconHex, "given", glyph.Char)
		}
		g := Glyph{
			ID:        id,
			ConstName: classToConstName(id),
			Code:      glyph.Code,
			HexCode:   uint32(iconHex),
			Icon:      icon,
		}
		alias, ok := aliases[id]
		if ok {
			g.Alias = alias
		}
		result = append(result, g)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return &glyphs.Metadata, result, nil
}

func getGlyphs() (*GlyphsBlob, error) {
	if noCache {
		return downloadGlyphs()
	}
	path := cachedGlyphsPath()
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return downloadGlyphs()
		}
		return nil, err
	}
	defer file.Close()
	slog.Info("reading cached glyphs")
	return readGlyphs(file)
}

type GlyphsBlob struct {
	Metadata GlyphsMetadata
	GlyphMap map[string]GlyphName
}

func (gb *GlyphsBlob) UnmarshalJSON(b []byte) error {
	tmp := make(map[string]map[string]string)
	err := json.Unmarshal(b, &tmp)
	if err != nil {
		return err
	}
	meta, ok := tmp["METADATA"]
	if !ok {
		return errors.New("no key found: \"METADATA\"")
	}
	delete(tmp, "METADATA")
	gb.Metadata.fromMap(meta)
	gb.GlyphMap = make(map[string]GlyphName, len(tmp))
	for key, m := range tmp {
		gb.GlyphMap[key] = GlyphName{
			Char: m["char"],
			Code: m["code"],
		}
	}
	return nil
}

type GlyphName struct{ Char, Code string }

type GlyphsMetadata struct {
	Website            string
	DevelopmentWebsite string
	Version            string
	Date               string
}

func (gm *GlyphsMetadata) fromMap(m map[string]string) {
	gm.Website = m["website"]
	gm.DevelopmentWebsite = m["development-website"]
	gm.Version = m["version"]
	gm.Date = m["date"]
}

func cachedGlyphsPath() string { return filepath.Join(os.TempDir(), "glyphnames.json") }

func downloadGlyphs() (*GlyphsBlob, error) {
	slog.Info("Downloading glyphs")
	resp, err := http.Get("https://raw.githubusercontent.com/ryanoasis/nerd-fonts/refs/heads/master/glyphnames.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	path := cachedGlyphsPath()
	cachefile, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = cachefile.Close()
		slog.Info("wrote to cached glyph file")
	}()
	return readGlyphs(io.TeeReader(resp.Body, cachefile))
}

func readGlyphs(r io.Reader) (*GlyphsBlob, error) {
	// var buf bytes.Buffer
	// _, err := buf.ReadFrom(r)
	// if err != nil {
	// 	return nil, err
	// }
	res := GlyphsBlob{
		GlyphMap: make(map[string]GlyphName),
	}
	err := json.NewDecoder(r).Decode(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func cleanupGlyphs(glyphs *GlyphsBlob) {
	collidingDuplicates := []string{
		"dev-digital_ocean",
		"md-calendar_week_end",
		"md-calendar_week_end_outline",
		"fa-thumb_tack",
		"fa-paint_brush",
		"fa-eye_dropper",
		"fa-eye_dropper",
		"METADATA",
	}
	for _, k := range collidingDuplicates {
		delete(glyphs.GlyphMap, k)
		delete(glyphs.GlyphMap, "nf-"+k)
	}
	for k := range glyphs.GlyphMap {
		if strings.HasPrefix(k, "nfold") {
			delete(glyphs.GlyphMap, k)
		}
	}
}

func findAliases(glyphs map[string]GlyphName) map[string]string {
	aliases := make(map[string][]string)
	classNames := slices.Sorted(maps.Keys(glyphs))
	for _, id := range classNames {
		glyph := glyphs[id]
		chr := glyph.Char
		dups, ok := aliases[chr]
		if !ok {
			aliases[chr] = []string{id}
		} else {
			aliases[chr] = append(dups, id)
		}
	}
	res := make(map[string]string)
	for _, dups := range aliases {
		if len(dups) > 1 {
			for i := 1; i < len(dups); i++ {
				res[dups[i]] = dups[0]
			}
		}
	}
	return res
}

type HashSet[T comparable] map[T]struct{}

func (hs HashSet[T]) LoadPut(v T) bool {
	_, ok := hs[v]
	hs[v] = struct{}{}
	return ok
}

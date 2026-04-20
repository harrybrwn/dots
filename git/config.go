package git

import (
	"io"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config map[string]any

func (c Config) Exists(key string) bool {
	_, ok := c[key]
	return ok
}

func (g *Git) Config() (Config, error) {
	f, err := os.Open(filepath.Join(g.gitDir, "config"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var _ toml.Unmarshaler
	var c Config
	meta, err := toml.NewDecoder(f).Decode(&c)
	if err != nil {
		return nil, err
	}
	if !meta.IsDefined("init.defaultbranch") {
		c["init.defaultbranch"] = "main"
	}
	return c, nil
}

func (g *Git) ConfigLocal() (Config, error) {
	return g.config("--local", "--list")
}

func (g *Git) ConfigGlobal() (Config, error) {
	return g.config("--global", "--list")
}

func (g *Git) ConfigSet(key, value string) error {
	return run(g.Cmd("config", key, value))
}

func (g *Git) ConfigLocalSet(key, value string) error {
	return run(g.Cmd("config", "--local", key, value))
}

func (g *Git) ConfigGlobalSet(key, value string) error {
	return run(g.Cmd("config", "--global", key, value))
}

func (g *Git) SetArgs(arguments ...string) { g.args = arguments }

func (g *Git) SetOut(out io.Writer) { g.stdout = out }
func (g *Git) SetErr(w io.Writer)   { g.stderr = w }

func (g *Git) SetGlobalConfig(filename string) *Git {
	g.configGlobal = filename
	return g
}

func (g *Git) SetSystemConfig(filename string) *Git {
	g.configSystem = filename
	return g
}

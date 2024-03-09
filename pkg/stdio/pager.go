package stdio

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mattn/go-shellwords"
)

// Page will page the in reader and write it to the out writer by running pager program.
func Page(pager string, out io.Writer, in io.Reader) error {
	args, err := shellwords.Parse(pager)
	if err != nil {
		return err
	}
	pager = args[0]
	args = args[1:]

	cmd := exec.Command(pager, args...)
	cmd.Stdout = out
	pagerIn, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	go func() {
		defer pagerIn.Close()
		_, _ = io.Copy(pagerIn, in)
	}()
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run pager %q: %w", pager, err)
	}
	return nil
}

func FindPager() (pager string) {
	if dotsPager, ok := os.LookupEnv("DOTS_PAGER"); ok {
		switch strings.ToLower(dotsPager) {
		case "false", "0":
			return ""
		default:
			return dotsPager
		}
	}
	p, ok := os.LookupEnv("GIT_PAGER")
	if ok {
		pager = p
	} else {
		p, ok = os.LookupEnv("PAGER")
		if ok {
			pager = p
		}
	}
	return pager
}

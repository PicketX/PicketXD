// m0-report converts actual test events into evidence and checks the M0 gate.
package main

import (
	"fmt"
	"github.com/PicketX/PicketXD/internal/validation/report"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: m0-report ARTIFACT_DIRECTORY")
	}
	dir := os.Args[1]
	kernel, err := os.ReadFile(filepath.Join(dir, "kernel.txt"))
	if err != nil {
		return err
	}
	modules, err := os.ReadFile(filepath.Join(dir, "modules.txt"))
	if err != nil {
		return err
	}
	r := report.Report{Kernel: strings.TrimSpace(string(kernel)), Go: runtime.Version(), Libraries: make(map[string]string)}
	for _, line := range strings.Split(string(modules), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			r.Libraries[f[0]] = f[1]
		}
	}
	for _, name := range []string{"nft.json", "ct.json"} {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		results, err := report.ReadEvents(f)
		f.Close()
		if err != nil {
			return err
		}
		r.Scenarios = append(r.Scenarios, results...)
	}
	out, err := os.Create(filepath.Join(dir, "report.json"))
	if err != nil {
		return err
	}
	err = report.Write(out, r)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := report.CheckRequired(r, report.RequiredScenarios); err != nil {
		return err
	}
	fmt.Printf("M0 observed kernel=%s Go=%s required=%d passed=%d\n", r.Kernel, r.Go, len(report.RequiredScenarios), len(report.RequiredScenarios))
	return nil
}

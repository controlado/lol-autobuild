package architecture_test

import (
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const modulePath = "github.com/controlado/lol-autobuild"

type packageMeta struct {
	ImportPath string
	Imports    []string
}

type importRule struct {
	Name          string
	PackagePrefix string
	Forbidden     []string
}

func TestInternalImportBoundaries(t *testing.T) {
	t.Parallel()

	packages := listInternalPackages(t)
	rules := []importRule{
		{
			Name:          "autobuild core does not import outer layers or wire packages",
			PackagePrefix: modulePath + "/internal/autobuild",
			Forbidden: []string{
				"encoding/json",
				modulePath + "/cmd/lol-autobuild",
				modulePath + "/internal/app",
				modulePath + "/internal/auth",
				modulePath + "/internal/coachless",
				modulePath + "/internal/config",
				modulePath + "/internal/lcu",
				modulePath + "/internal/secrets",
				modulePath + "/internal/ui",
				modulePath + "/internal/update",
			},
		},
		{
			Name:          "app does not import infrastructure directly",
			PackagePrefix: modulePath + "/internal/app",
			Forbidden: []string{
				modulePath + "/internal/auth",
				modulePath + "/internal/coachless",
				modulePath + "/internal/config",
				modulePath + "/internal/lcu",
				modulePath + "/internal/secrets",
				modulePath + "/internal/ui",
				modulePath + "/internal/update",
			},
		},
	}

	for _, rule := range rules {
		rule := rule
		t.Run(rule.Name, func(t *testing.T) {
			t.Parallel()
			assertNoForbiddenDirectImports(t, packages, rule)
		})
	}
}

func listInternalPackages(t *testing.T) []packageMeta {
	t.Helper()

	cmd := exec.Command("go", "list", "-json", "./internal/...")
	cmd.Dir = filepath.Join("..", "..")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list ./internal/...: %v\n%s", err, raw)
	}

	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	var packages []packageMeta
	for {
		var meta packageMeta
		err := decoder.Decode(&meta)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("parse go list output: %v", err)
		}
		packages = append(packages, meta)
	}

	return packages
}

func assertNoForbiddenDirectImports(t *testing.T, packages []packageMeta, rule importRule) {
	t.Helper()

	forbidden := make(map[string]struct{}, len(rule.Forbidden))
	for _, importPath := range rule.Forbidden {
		forbidden[importPath] = struct{}{}
	}

	for _, pkg := range packages {
		if !matchesPackagePrefix(pkg.ImportPath, rule.PackagePrefix) {
			continue
		}
		for _, importPath := range pkg.Imports {
			if _, ok := forbidden[importPath]; ok {
				t.Fatalf("%s imports forbidden package %s", pkg.ImportPath, importPath)
			}
		}
	}
}

func matchesPackagePrefix(importPath string, prefix string) bool {
	return importPath == prefix || strings.HasPrefix(importPath, prefix+"/")
}

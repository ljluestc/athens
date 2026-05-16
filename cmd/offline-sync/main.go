package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gomods/athens/cmd/proxy/actions"
	"github.com/gomods/athens/pkg/config"
	"github.com/gomods/athens/pkg/offlinesync"
	"github.com/gomods/athens/pkg/storage"
)

type packagesFlag []string

func (p *packagesFlag) String() string { return strings.Join(*p, ",") }
func (p *packagesFlag) Set(v string) error {
	*p = append(*p, v)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usageAndExit()
	}

	switch os.Args[1] {
	case "export":
		runExport(os.Args[2:])
	case "import":
		runImport(os.Args[2:])
	case "delete":
		runDelete(os.Args[2:])
	default:
		usageAndExit()
	}
}

func runExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	configFile := fs.String("config_file", "", "path to athens.toml")
	outPath := fs.String("out", "athens-offline-sync.tar.gz", "output archive path")
	pageSize := fs.Int("page_size", 500, "catalog page size for full export")
	var packages packagesFlag
	fs.Var(&packages, "package", "module@version to export (repeatable), if empty exports full catalog")
	_ = fs.Parse(args)

	backend := mustBackend(*configFile)
	refs := mustParseRefs(packages)

	f := mustCreate(*outPath)
	defer f.Close()

	manifest, err := offlinesync.ExportArchive(context.Background(), backend, refs, *pageSize, f)
	if err != nil {
		fatalf("export failed: %v", err)
	}
	fmt.Printf("exported %d package versions to %s\n", manifest.ItemCount, *outPath)
}

func runImport(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	configFile := fs.String("config_file", "", "path to athens.toml")
	inPath := fs.String("in", "", "input archive path")
	_ = fs.Parse(args)
	if *inPath == "" {
		fatalf("-in is required")
	}

	backend := mustBackend(*configFile)
	f := mustOpen(*inPath)
	defer f.Close()

	manifest, err := offlinesync.ImportArchive(context.Background(), backend, f)
	if err != nil {
		fatalf("import failed: %v", err)
	}
	fmt.Printf("imported %d package versions from %s\n", manifest.ItemCount, *inPath)
}

func runDelete(args []string) {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	configFile := fs.String("config_file", "", "path to athens.toml")
	ignoreMissing := fs.Bool("ignore_missing", true, "ignore missing module versions")
	var packages packagesFlag
	fs.Var(&packages, "package", "module@version to delete (repeatable)")
	_ = fs.Parse(args)

	if len(packages) == 0 {
		fatalf("at least one --package module@version is required")
	}

	backend := mustBackend(*configFile)
	refs := mustParseRefs(packages)

	deleted, err := offlinesync.DeletePackages(context.Background(), backend, refs, *ignoreMissing)
	if err != nil {
		fatalf("delete failed: %v", err)
	}
	fmt.Printf("deleted %d package versions\n", deleted)
}

func mustBackend(configFile string) storage.Backend {
	conf, err := config.Load(configFile)
	if err != nil {
		fatalf("load config: %v", err)
	}
	backend, err := actions.GetStorage(conf.StorageType, conf.Storage, conf.TimeoutDuration(), &http.Client{})
	if err != nil {
		fatalf("init storage backend: %v", err)
	}
	return backend
}

func mustParseRefs(values []string) []offlinesync.PackageRef {
	refs, err := offlinesync.ParsePackageRefs(values)
	if err != nil {
		fatalf("parse package refs: %v", err)
	}
	return refs
}

func mustCreate(path string) *os.File {
	f, err := os.Create(path)
	if err != nil {
		fatalf("create file %s: %v", path, err)
	}
	return f
}

func mustOpen(path string) *os.File {
	f, err := os.Open(path)
	if err != nil {
		fatalf("open file %s: %v", path, err)
	}
	return f
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func usageAndExit() {
	fmt.Fprintln(os.Stderr, "usage: athens-offline-sync <export|import|delete> [flags]")
	os.Exit(2)
}

package offlinesync

import (
	"bytes"
	"context"
	"testing"

	"github.com/gomods/athens/pkg/storage/mem"
)

func TestParsePackageRef(t *testing.T) {
	ref, err := ParsePackageRef("github.com/acme/lib@v1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Module != "github.com/acme/lib" || ref.Version != "v1.2.3" {
		t.Fatalf("unexpected ref: %#v", ref)
	}
}

func TestParsePackageRefInvalid(t *testing.T) {
	_, err := ParsePackageRef("github.com/acme/lib")
	if err == nil {
		t.Fatal("expected error for invalid package ref")
	}
}

func TestExportImportArchiveRoundTrip(t *testing.T) {
	ctx := context.Background()
	st, err := mem.NewStorage()
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	module := "example.com/mod"
	version := "v1.0.0"
	mod := []byte("module example.com/mod\n")
	info := []byte(`{"Version":"v1.0.0"}`)
	zip := []byte("fake zip bytes")

	if err := st.Save(ctx, module, version, mod, bytes.NewReader(zip), nil, info); err != nil {
		t.Fatalf("seed storage: %v", err)
	}

	var archive bytes.Buffer
	manifest, err := ExportArchive(ctx, st, []PackageRef{{Module: module, Version: version}}, 100, &archive)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if manifest.ItemCount != 1 {
		t.Fatalf("expected 1 item, got %d", manifest.ItemCount)
	}

	st2, err := mem.NewStorage()
	if err != nil {
		t.Fatalf("new target storage: %v", err)
	}

	if _, err := ImportArchive(ctx, st2, bytes.NewReader(archive.Bytes())); err != nil {
		t.Fatalf("import: %v", err)
	}

	gotInfo, err := st2.Info(ctx, module, version)
	if err != nil {
		t.Fatalf("get info: %v", err)
	}
	if !bytes.Equal(gotInfo, info) {
		t.Fatalf("info mismatch: got %q want %q", string(gotInfo), string(info))
	}

	gotMod, err := st2.GoMod(ctx, module, version)
	if err != nil {
		t.Fatalf("get mod: %v", err)
	}
	if !bytes.Equal(gotMod, mod) {
		t.Fatalf("mod mismatch: got %q want %q", string(gotMod), string(mod))
	}
}

func TestDeletePackages(t *testing.T) {
	ctx := context.Background()
	st, err := mem.NewStorage()
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	module := "example.com/mod"
	version := "v1.2.3"
	if err := st.Save(ctx, module, version, []byte("module example.com/mod\n"), bytes.NewReader([]byte("zip")), nil, []byte(`{"Version":"v1.2.3"}`)); err != nil {
		t.Fatalf("seed storage: %v", err)
	}

	n, err := DeletePackages(ctx, st, []PackageRef{{Module: module, Version: version}}, false)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected delete count 1, got %d", n)
	}
}

package offlinesync

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/gomods/athens/pkg/storage"
)

const (
	ManifestName = "manifest.json"
	FormatV1     = "athens-offline-sync/v1"
)

type PackageRef struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

type ManifestItem struct {
	Module   string `json:"module"`
	Version  string `json:"version"`
	InfoPath string `json:"info_path"`
	ModPath  string `json:"mod_path"`
	ZipPath  string `json:"zip_path"`
}

type Manifest struct {
	FormatVersion string         `json:"format_version"`
	CreatedAt     time.Time      `json:"created_at"`
	ItemCount     int            `json:"item_count"`
	Items         []ManifestItem `json:"items"`
}

func ParsePackageRef(s string) (PackageRef, error) {
	idx := strings.LastIndex(s, "@")
	if idx <= 0 || idx == len(s)-1 {
		return PackageRef{}, fmt.Errorf("invalid package ref %q, expected module@version", s)
	}
	return PackageRef{Module: s[:idx], Version: s[idx+1:]}, nil
}

func ParsePackageRefs(refs []string) ([]PackageRef, error) {
	result := make([]PackageRef, 0, len(refs))
	for _, r := range refs {
		ref, err := ParsePackageRef(strings.TrimSpace(r))
		if err != nil {
			return nil, err
		}
		result = append(result, ref)
	}
	return uniqueAndSorted(result), nil
}

func ExportArchive(ctx context.Context, backend storage.Backend, refs []PackageRef, pageSize int, w io.Writer) (*Manifest, error) {
	items := uniqueAndSorted(refs)
	if len(items) == 0 {
		cataloger, ok := backend.(storage.Cataloger)
		if !ok {
			return nil, fmt.Errorf("backend does not support catalog export; provide explicit --package refs")
		}
		all, err := collectFromCatalog(ctx, cataloger, pageSize)
		if err != nil {
			return nil, err
		}
		items = all
	}

	manifest := &Manifest{
		FormatVersion: FormatV1,
		CreatedAt:     time.Now().UTC(),
		ItemCount:     len(items),
		Items:         make([]ManifestItem, 0, len(items)),
	}

	for i, p := range items {
		base := fmt.Sprintf("objects/%06d", i+1)
		manifest.Items = append(manifest.Items, ManifestItem{
			Module:   p.Module,
			Version:  p.Version,
			InfoPath: base + "/info.json",
			ModPath:  base + "/go.mod",
			ZipPath:  base + "/source.zip",
		})
	}

	gzw := gzip.NewWriter(w)
	tw := tar.NewWriter(gzw)
	defer func() {
		_ = tw.Close()
		_ = gzw.Close()
	}()

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	if err := writeTarFile(tw, ManifestName, manifestBytes); err != nil {
		return nil, err
	}

	for _, item := range manifest.Items {
		info, err := backend.Info(ctx, item.Module, item.Version)
		if err != nil {
			return nil, fmt.Errorf("read info %s@%s: %w", item.Module, item.Version, err)
		}
		mod, err := backend.GoMod(ctx, item.Module, item.Version)
		if err != nil {
			return nil, fmt.Errorf("read mod %s@%s: %w", item.Module, item.Version, err)
		}
		zipReader, err := backend.Zip(ctx, item.Module, item.Version)
		if err != nil {
			return nil, fmt.Errorf("read zip %s@%s: %w", item.Module, item.Version, err)
		}
		zipBytes, readErr := io.ReadAll(zipReader)
		closeErr := zipReader.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read zip stream %s@%s: %w", item.Module, item.Version, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close zip stream %s@%s: %w", item.Module, item.Version, closeErr)
		}

		if err := writeTarFile(tw, item.InfoPath, info); err != nil {
			return nil, err
		}
		if err := writeTarFile(tw, item.ModPath, mod); err != nil {
			return nil, err
		}
		if err := writeTarFile(tw, item.ZipPath, zipBytes); err != nil {
			return nil, err
		}
	}

	return manifest, nil
}

func ImportArchive(ctx context.Context, backend storage.Backend, r io.Reader) (*Manifest, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("open gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	objects := map[string][]byte{}
	var manifest Manifest
	manifestLoaded := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read tar content %s: %w", hdr.Name, err)
		}
		objects[hdr.Name] = content
		if hdr.Name == ManifestName {
			if err := json.Unmarshal(content, &manifest); err != nil {
				return nil, fmt.Errorf("parse manifest: %w", err)
			}
			manifestLoaded = true
		}
	}

	if !manifestLoaded {
		return nil, fmt.Errorf("archive missing %s", ManifestName)
	}
	if manifest.FormatVersion != FormatV1 {
		return nil, fmt.Errorf("unsupported format_version: %s", manifest.FormatVersion)
	}

	for _, item := range manifest.Items {
		info, ok := objects[item.InfoPath]
		if !ok {
			return nil, fmt.Errorf("missing info blob for %s@%s", item.Module, item.Version)
		}
		mod, ok := objects[item.ModPath]
		if !ok {
			return nil, fmt.Errorf("missing mod blob for %s@%s", item.Module, item.Version)
		}
		zipBytes, ok := objects[item.ZipPath]
		if !ok {
			return nil, fmt.Errorf("missing zip blob for %s@%s", item.Module, item.Version)
		}

		if err := backend.Save(ctx, item.Module, item.Version, mod, bytes.NewReader(zipBytes), nil, info); err != nil {
			return nil, fmt.Errorf("import %s@%s: %w", item.Module, item.Version, err)
		}
	}

	return &manifest, nil
}

func DeletePackages(ctx context.Context, backend storage.Backend, refs []PackageRef, ignoreMissing bool) (int, error) {
	deleted := 0
	for _, p := range uniqueAndSorted(refs) {
		if err := backend.Delete(ctx, p.Module, p.Version); err != nil {
			if ignoreMissing && strings.Contains(strings.ToLower(err.Error()), "not found") {
				continue
			}
			return deleted, fmt.Errorf("delete %s@%s: %w", p.Module, p.Version, err)
		}
		deleted++
	}
	return deleted, nil
}

func collectFromCatalog(ctx context.Context, cataloger storage.Cataloger, pageSize int) ([]PackageRef, error) {
	if pageSize <= 0 {
		pageSize = 500
	}
	token := ""
	out := []PackageRef{}
	for {
		entries, next, err := cataloger.Catalog(ctx, token, pageSize)
		if err != nil {
			return nil, fmt.Errorf("catalog storage: %w", err)
		}
		for _, e := range entries {
			out = append(out, PackageRef{Module: e.Module, Version: e.Version})
		}
		if next == "" {
			break
		}
		token = next
	}
	return uniqueAndSorted(out), nil
}

func uniqueAndSorted(in []PackageRef) []PackageRef {
	uniq := map[string]PackageRef{}
	for _, p := range in {
		key := p.Module + "@" + p.Version
		uniq[key] = p
	}
	out := make([]PackageRef, 0, len(uniq))
	for _, p := range uniq {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Module == out[j].Module {
			return out[i].Version < out[j].Version
		}
		return out[i].Module < out[j].Module
	})
	return out
}

func writeTarFile(tw *tar.Writer, name string, content []byte) error {
	hdr := &tar.Header{
		Name:     name,
		Mode:     0o644,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header %s: %w", name, err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("write tar body %s: %w", name, err)
	}
	return nil
}

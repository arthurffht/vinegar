package netutil

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

// ExtractURL will decompress the given XZ compressed tarball URL
// into dir. The archive is first extracted into a temporary directory
// next to dir, and its contents are only moved into place once the
// extraction fully succeeds, so that an interrupted download never
// leaves a partially extracted tree behind.
func ExtractURL(url string, dir string) error {
	resp, err := get(url)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(dir, ".extract-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := extract(resp.Body, tmp); err != nil {
		return err
	}

	entries, err := os.ReadDir(tmp)
	if err != nil {
		return err
	}
	for _, e := range entries {
		dst := filepath.Join(dir, e.Name())
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(tmp, e.Name()), dst); err != nil {
			return err
		}
	}

	return nil
}

func extract(src io.Reader, dir string) error {
	xr, err := xz.NewReader(src)
	if err != nil {
		return fmt.Errorf("xz: %w", err)
	}
	r := tar.NewReader(xr)
	root := filepath.Clean(dir) + string(os.PathSeparator)

	within := func(name string) (string, error) {
		dest := filepath.Join(dir, name)
		if !strings.HasPrefix(dest, root) {
			return "", fmt.Errorf("illegal package file path: %s", name)
		}
		return dest, nil
	}

	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		dest, err := within(h.Name)
		if err != nil {
			return err
		}

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(h.Linkname, dest); err != nil {
				return err
			}
		case tar.TypeLink:
			target, err := within(h.Linkname)
			if err != nil {
				return err
			}
			if err := os.Link(target, dest); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := extractFile(r, h, dest); err != nil {
				return err
			}
		}
	}

	return nil
}

func extractFile(r io.Reader, h *tar.Header, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(dest,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, h.FileInfo().Mode().Perm())
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return fmt.Errorf("copy: %w", err)
	}
	if err := f.Close(); err != nil {
		return err
	}

	return os.Chtimes(dest, h.AccessTime, h.ModTime)
}

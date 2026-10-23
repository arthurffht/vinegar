package gutil

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"codeberg.org/puregotk/puregotk/v4/gio"
	"codeberg.org/puregotk/puregotk/v4/glib"
)

// ResourceName is the name of the compiled GResource bundle
// installed in the package data directory.
const ResourceName = "vinegar.gresource"

// LoadResources registers the application's GResource bundle. The
// VINEGAR_RESOURCE environment variable takes precedence, as set by
// 'meson devenv', followed by the given installation directory.
func LoadResources(dir string) error {
	candidates := []string{filepath.Join(dir, ResourceName)}
	if p := os.Getenv("VINEGAR_RESOURCE"); p != "" {
		candidates = append([]string{p}, candidates...)
	}

	var errs []error
	for _, name := range candidates {
		r, err := gio.ResourceLoad(name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		gio.ResourcesRegister(r)
		return nil
	}

	return fmt.Errorf("resources not found: %w", errors.Join(errs...))
}

func Resource(elems ...string) string {
	return path.Join(append([]string{"/org/vinegarhq/Vinegar"}, elems...)...)
}

func ResourceData(resource string) []byte {
	gb, err := gio.ResourcesLookupData(resource, gio.GResourceLookupFlagsNoneValue)
	if err != nil {
		panic("gutil: resource " + resource + " not found")
	}

	return Slice[byte](gb.GetData(nil), gb.GetSize())
}

func IdleAdd(bg func()) {
	var idlecb glib.SourceFunc
	idlecb = func(uintptr) bool {
		defer glib.UnrefCallback(&idlecb)
		bg()
		return false
	}
	glib.IdleAdd(&idlecb, 0)
}

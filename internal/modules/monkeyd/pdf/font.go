package pdf

import (
	_ "embed"
)

// bundledTTF is the only font the bot renders with. A minimal container has no
// system fonts, and a single embedded font keeps the PDF identical on every
// host. See fonts/NOTICE.md for provenance and licensing.
//
// Vietnamese text needs the Latin Extended Additional block (ư, ạ, ế, ộ …);
// DejaVu Sans covers it, where a basic-Latin font would silently drop the
// diacritics.
//
//go:embed fonts/DejaVuSans.ttf
var bundledTTF []byte

// BundledFontName labels the embedded font in diagnostics. It is not a path;
// the font is compiled into the binary.
const BundledFontName = "DejaVu Sans (bundled)"

// Font is font data ready to embed in a PDF.
//
// The data is carried as bytes rather than as a path because fpdf joins a font
// path onto its own font directory, which it defaults to "." — turning an
// absolute path into a working-directory-relative one that resolves only when
// the process happens to run from the filesystem root.
type Font struct {
	// Name identifies the font for diagnostics.
	Name string
	Data []byte
}

// BundledFont returns the font compiled into the binary.
func BundledFont() Font {
	return Font{Name: BundledFontName, Data: bundledTTF}
}

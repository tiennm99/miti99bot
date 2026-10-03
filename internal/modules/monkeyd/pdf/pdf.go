package pdf

import (
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

// pointsToMM converts typographic points to millimetres.
const pointsToMM = 25.4 / 72.0

// bodyFont is the internal family name registered with the PDF.
const bodyFont = "body"

// PageSize is a page in millimetres.
type PageSize struct {
	Name string
	W, H float64
}

// PhonePage is the page every export uses.
//
// Phone reading depends far more on page shape than on font size. A phone
// viewer scales a whole page to fit the screen, so a large font on an A4 page
// still ends up tiny: the page is ~3x wider than the screen and gets shrunk to
// match. A page cut to the phone's own aspect ratio fills the screen at 100%,
// which is why this is a small 9:16 page rather than A4 with big type.
var PhonePage = PageSize{Name: "phone", W: 90, H: 160}

// Chapter is a chapter ready to render.
type Chapter struct {
	Heading    string
	Paragraphs []string
}

// Options controls the exported PDF.
type Options struct {
	Page        PageSize
	Margin      float64 // mm
	Font        Font    // BundledFont()
	FontSize    float64 // pt
	LineSpacing float64 // multiple of font size
	Title       string
	SourceURL   string
}

// footerReserve is the vertical space kept clear for the page number.
const footerReserve = 6.0

// Write renders the chapters to a PDF at path.
func Write(path string, opts Options, chapters []Chapter) error {
	pdf := fpdf.NewCustom(&fpdf.InitType{
		UnitStr: "mm",
		Size:    fpdf.SizeType{Wd: opts.Page.W, Ht: opts.Page.H},
	})

	pdf.SetMargins(opts.Margin, opts.Margin, opts.Margin)
	pdf.SetAutoPageBreak(true, opts.Margin+footerReserve)

	// Embeds a subset of the TrueType data, which is what makes the Vietnamese
	// diacritics render instead of falling back to "?". The bytes are passed
	// directly rather than by path: the path-taking variant joins the name onto
	// fpdf's own font directory (default "."), which mangles an absolute path
	// into a working-directory-relative one.
	pdf.AddUTF8FontFromBytes(bodyFont, "", opts.Font.Data)
	pdf.SetFont(bodyFont, "", opts.FontSize)
	pdf.SetTitle(opts.Title, true)

	lineHeight := opts.FontSize * opts.LineSpacing * pointsToMM
	paragraphGap := lineHeight * 0.45
	headingSize := opts.FontSize * 1.35

	addFooter(pdf, opts)
	writeTitlePage(pdf, opts, len(chapters))

	for _, ch := range chapters {
		pdf.AddPage()

		pdf.SetFontSize(headingSize)
		pdf.MultiCell(0, headingSize*1.3*pointsToMM, ch.Heading, "", "L", false)
		pdf.Ln(paragraphGap * 1.6)

		pdf.SetFontSize(opts.FontSize)
		for _, p := range ch.Paragraphs {
			// "J" justifies, which keeps the short measure of a phone page tidy.
			pdf.MultiCell(0, lineHeight, p, "", "J", false)
			pdf.Ln(paragraphGap)
		}
	}

	if err := pdf.OutputFileAndClose(path); err != nil {
		return fmt.Errorf("write pdf %s: %w", path, err)
	}
	return nil
}

// addFooter prints a centred page number, restoring the body font size so the
// footer callback cannot leak its own size into the following content.
func addFooter(pdf *fpdf.Fpdf, opts Options) {
	pdf.SetFooterFunc(func() {
		if pdf.PageNo() <= 1 {
			return
		}
		pdf.SetY(-(opts.Margin + footerReserve*0.6))
		pdf.SetFontSize(opts.FontSize * 0.75)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 4, fmt.Sprintf("%d", pdf.PageNo()-1), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFontSize(opts.FontSize)
	})
}

func writeTitlePage(pdf *fpdf.Fpdf, opts Options, chapterCount int) {
	pdf.AddPage()
	pdf.SetY(opts.Page.H * 0.30)

	titleSize := opts.FontSize * 1.9
	pdf.SetFontSize(titleSize)
	pdf.MultiCell(0, titleSize*1.35*pointsToMM, strings.ToUpper(opts.Title), "", "C", false)

	pdf.Ln(opts.FontSize * pointsToMM * 2)
	pdf.SetFontSize(opts.FontSize * 0.85)
	pdf.SetTextColor(90, 90, 90)
	pdf.MultiCell(0, opts.FontSize*1.4*pointsToMM,
		fmt.Sprintf("%d chương", chapterCount), "", "C", false)
	if opts.SourceURL != "" {
		pdf.MultiCell(0, opts.FontSize*1.4*pointsToMM, opts.SourceURL, "", "C", false)
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFontSize(opts.FontSize)
}

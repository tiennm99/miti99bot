# Bundled font

`DejaVuSans.ttf` is embedded into the binary and used when no font is supplied
and no suitable system font is found. It covers the Latin Extended Additional
block, which is what Vietnamese diacritics need — a font with only basic Latin
coverage silently drops them.

The following is recorded in the font file's own name table:

- Version: `Version 2.37`
- Copyright: `Copyright (c) 2003 by Bitstream, Inc. All Rights Reserved.`
  `Copyright (c) 2006 by Tavmjong Bah. All Rights Reserved.`
  `DejaVu changes are in public domain`
- License information: <http://dejavu.sourceforge.net/wiki/index.php/License>

The DejaVu fonts are free and redistributable, which is why they ship with most
Linux distributions. This copy came from Alpine's `font-dejavu` package, which
does not include the license text as a separate file. For a vendored copy of the
full license text, take `LICENSE` from the upstream DejaVu release and add it to
this directory.

To swap the bundled font, replace `DejaVuSans.ttf` and update this file. Verify
the replacement covers Vietnamese first — `TestBundledFontCoversVietnamese` in
`../font_test.go` checks a representative set of characters.

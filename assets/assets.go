// Package assets embeds TouchUp's fonts, icons and app icon.
package assets

import (
	"embed"
	_ "embed"
)

// Fonts holds the Inter typeface (SIL Open Font License, fonts/OFL.txt).
//
//go:embed fonts/*.ttf
var Fonts embed.FS

// Icons holds Lucide icons (ISC License, icons/LICENSE).
//
//go:embed icons/*.svg
var Icons embed.FS

// AppIcon is the window and app icon.
//
//go:embed icon.png
var AppIcon []byte

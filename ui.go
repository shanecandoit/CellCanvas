package main

import (
	"image/color"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
)

type UI struct {
	face font.Face
}

func NewUI() *UI {
	ui := &UI{}

	// Try to load local RobotoMono TTF from res/
	b, err := os.ReadFile("res/Roboto-Regular.ttf")
	if err != nil {
		log.Printf("could not read font file: %v; falling back to basic font", err)
		ui.face = basicfont.Face7x13
		return ui
	}
	tt, err := opentype.Parse(b)
	if err != nil {
		log.Printf("could not parse ttf: %v; falling back to basic font", err)
		ui.face = basicfont.Face7x13
		return ui
	}
	face, err := opentype.NewFace(tt, &opentype.FaceOptions{Size: 14, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		log.Printf("could not create font face: %v; falling back to basic font", err)
		ui.face = basicfont.Face7x13
		return ui
	}
	ui.face = face
	return ui
}

// Draw renders HUD and editing text overlay
func (ui *UI) Draw(screen *ebiten.Image, g *Game) {
	// Use the actual logical screen height so the HUD sits at the bottom
	// even when the window is resized.
	screenH := screen.Bounds().Dy()
	drawTextAt(screen, ui.face, "Right-drag to pan - Left-drag title to move - Drag corner to resize", 8, screenH-28, color.White)
	drawTextAt(screen, ui.face, "Arrows to move - Enter to edit - Tab switch panel", 8, screenH-14, color.White)

	if g.editing {
		// top text bar background
		sw := screen.Bounds().Dx()
		ebitenutil.DrawRect(screen, 0, 0, float64(sw), 34, color.RGBA{0x11, 0x11, 0x16, 0xff})
		padding := 8
		// draw the edit buffer in the bar
		drawTextAt(screen, ui.face, g.editBuffer, padding, 6, color.White)

		// draw caret if visible
		if g.caretVisible {
			// compute width of runes before cursor to place caret
			rs := []rune(g.editBuffer)
			if g.editCursor < 0 {
				g.editCursor = 0
			}
			if g.editCursor > len(rs) {
				g.editCursor = len(rs)
			}
			pre := string(rs[:g.editCursor])
			bounds, _ := font.BoundString(ui.face, pre)
			caretX := int((bounds.Max.X - bounds.Min.X) >> 6)
			// caret height roughly ascent+descent
			ascent := ui.face.Metrics().Ascent.Round()
			descent := ui.face.Metrics().Descent.Round()
			caretH := ascent + descent
			caretY := 6
			ebitenutil.DrawRect(screen, float64(padding+caretX), float64(caretY), 2, float64(caretH), color.White)
		}

		// position editing text over the selected cell
		if g.activePanel >= 0 && g.activePanel < len(g.canvas.panels) {
			p := g.canvas.panels[g.activePanel]
			sx := float64(p.X) + g.canvas.camX + float64(g.selCol*p.CellW)
			sy := float64(p.Y) + g.canvas.camY + float64(g.selRow*p.CellH)
			drawTextAt(screen, ui.face, g.editBuffer, int(sx)+6, int(sy)+6, color.White)
		}
	}
}

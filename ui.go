package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
)

type UI struct {
	face font.Face
	// double-click tracking (moved here from Canvas)
	lastClickPanel int
	lastClickRow   int
	lastClickCol   int
	lastClickTime  int64 // unix ms
	dblClickMs     int64
	// recent mouse click log (most-recent first)
	clickLog []string
}

func NewUI() *UI {
	ui := &UI{}
	ui.lastClickPanel = -1
	ui.lastClickRow = -1
	ui.lastClickCol = -1
	ui.lastClickTime = 0
	ui.dblClickMs = 400

	ui.clickLog = []string{}

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

// Update handles editing input, caret blinking, and commit/cancel while editing.
func (ui *UI) Update(g *Game) {
	// Log mouse clicks (left and right) with panel/cell when detectable.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		btn := "L"
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
			btn = "R"
		}
		mx, my := ebiten.CursorPosition()
		// try to detect panel/cell under cursor
		found := false
		for i := len(g.canvas.panels) - 1; i >= 0; i-- {
			p := g.canvas.panels[i]
			baseX := int(float64(p.X) + g.canvas.camX)
			baseY := int(float64(p.Y) + g.canvas.camY)
			w := p.Cols * p.CellW
			h := p.Rows * p.CellH
			if mx >= baseX && mx <= baseX+w && my >= baseY && my <= baseY+h {
				col := (mx - baseX) / p.CellW
				row := (my - baseY) / p.CellH
				ui.addClickLog(fmt.Sprintf("%s click @ %d,%d  panel=%d row=%d col=%d", btn, mx, my, i, row, col))
				found = true
				break
			}
		}
		if !found {
			ui.addClickLog(fmt.Sprintf("%s click @ %d,%d  (no panel)", btn, mx, my))
		}
	}
	// global shortcuts: Save (Ctrl+S) and Open (Ctrl+O)
	ctrlPressed := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	if ctrlPressed && inpututil.IsKeyJustPressed(ebiten.KeyS) {
		// default state file
		statePath := "state.yml"
		if err := g.canvas.SaveState(statePath); err != nil {
			log.Printf("Save failed: %v", err)
		} else {
			log.Printf("Saved to %s", statePath)
		}
	}
	if ctrlPressed && inpututil.IsKeyJustPressed(ebiten.KeyO) {
		statePath := "state.yml"
		if err := g.canvas.LoadState(statePath); err != nil {
			log.Printf("Open failed: %v", err)
		} else {
			log.Printf("Loaded from %s", statePath)
		}
	}
	// start editing when Enter is pressed (only when not already editing)
	if !g.editing {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if g.activePanel >= 0 && g.activePanel < len(g.canvas.panels) {
				g.editing = true
				g.editBuffer = g.canvas.panels[g.activePanel].Cells[g.selRow][g.selCol]
				g.editCursor = len([]rune(g.editBuffer))
				g.blinkCounter = 0
				g.caretVisible = true
			}
		}
		return
	}

	// blink caret timer
	g.blinkCounter++
	if g.blinkCounter%30 == 0 {
		g.caretVisible = !g.caretVisible
	}

	// handle typed characters, inserting at cursor
	for _, r := range ebiten.InputChars() {
		if r == '\b' {
			if g.editCursor > 0 {
				rs := []rune(g.editBuffer)
				rs = append(rs[:g.editCursor-1], rs[g.editCursor:]...)
				g.editBuffer = string(rs)
				g.editCursor--
				g.blinkCounter = 0
				g.caretVisible = true
			}
		} else {
			rs := []rune(g.editBuffer)
			rs = append(rs[:g.editCursor], append([]rune{r}, rs[g.editCursor:]...)...)
			g.editBuffer = string(rs)
			g.editCursor++
			g.blinkCounter = 0
			g.caretVisible = true
		}
	}

	// navigation and editing keys
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		if g.editCursor > 0 {
			g.editCursor--
			g.blinkCounter = 0
			g.caretVisible = true
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		if g.editCursor < len([]rune(g.editBuffer)) {
			g.editCursor++
			g.blinkCounter = 0
			g.caretVisible = true
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if g.editCursor > 0 {
			rs := []rune(g.editBuffer)
			rs = append(rs[:g.editCursor-1], rs[g.editCursor:]...)
			g.editBuffer = string(rs)
			g.editCursor--
			g.blinkCounter = 0
			g.caretVisible = true
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		rs := []rune(g.editBuffer)
		if g.editCursor < len(rs) {
			rs = append(rs[:g.editCursor], rs[g.editCursor+1:]...)
			g.editBuffer = string(rs)
			g.blinkCounter = 0
			g.caretVisible = true
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		g.editCursor = 0
		g.blinkCounter = 0
		g.caretVisible = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
		g.editCursor = len([]rune(g.editBuffer))
		g.blinkCounter = 0
		g.caretVisible = true
	}

	// commit/cancel
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if g.activePanel >= 0 && g.activePanel < len(g.canvas.panels) {
			g.canvas.panels[g.activePanel].Cells[g.selRow][g.selCol] = g.editBuffer
		}
		g.editing = false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.editing = false
	}
}

// OnCellClick handles click events on a cell and detects double-clicks to begin editing.
func (ui *UI) OnCellClick(g *Game, panel, row, col int) {
	now := time.Now().UnixNano() / 1e6
	if ui.lastClickPanel == panel && ui.lastClickRow == row && ui.lastClickCol == col && now-ui.lastClickTime <= ui.dblClickMs {
		// double-click: start editing
		if panel >= 0 && panel < len(g.canvas.panels) {
			g.editing = true
			g.editBuffer = g.canvas.panels[panel].Cells[row][col]
			g.editCursor = len([]rune(g.editBuffer))
			g.blinkCounter = 0
			g.caretVisible = true
		}
		// reset last click to avoid immediate retrigger
		ui.lastClickPanel = -1
	} else {
		ui.lastClickPanel = panel
		ui.lastClickRow = row
		ui.lastClickCol = col
		ui.lastClickTime = now
	}
}

// addClickLog prepends a timestamped entry to the click log and keeps it bounded.
func (ui *UI) addClickLog(s string) {
	ts := time.Now().Format("15:04:05.000")
	entry := fmt.Sprintf("%s  %s", ts, s)
	// prepend
	ui.clickLog = append([]string{entry}, ui.clickLog...)
	if len(ui.clickLog) > 10 {
		ui.clickLog = ui.clickLog[:10]
	}
}

// Draw renders HUD and editing text overlay
func (ui *UI) Draw(screen *ebiten.Image, g *Game) {
	// Use the actual logical screen height so the HUD sits at the bottom
	// even when the window is resized.
	screenH := screen.Bounds().Dy()
	drawTextAt(screen, ui.face, "Right-drag to pan - Left-drag title to move - Drag corner to resize", 8, screenH-42, color.White)
	drawTextAt(screen, ui.face, "Press Ctrl+S to Save - Press Ctrl+O to Open", 8, screenH-28, color.White)
	drawTextAt(screen, ui.face, "Arrows to move - Enter to edit - Tab switch panel", 8, screenH-14, color.White)

	if g.editing {
		// top text bar background
		sw := screen.Bounds().Dx()
		ebitenutil.DrawRect(screen, 0, 0, float64(sw), 34, color.RGBA{0x11, 0x11, 0x16, 0xff})
		padding := 8

		// build a label like: "Edit Panel0 Cell-A1 : "
		label := fmt.Sprintf("Edit Panel%d Cell-%s%d : ", g.activePanel, ColToLetters(g.selCol), g.selRow+1)

		// render the full bracketed string
		full := label + "[" + g.editBuffer + "]"
		drawTextAt(screen, ui.face, full, padding, 6, color.White)

		// draw caret if visible (measured relative to the full label)
		if g.caretVisible {
			rs := []rune(g.editBuffer)
			if g.editCursor < 0 {
				g.editCursor = 0
			}
			if g.editCursor > len(rs) {
				g.editCursor = len(rs)
			}
			pre := label + "[" + string(rs[:g.editCursor])
			b, _ := font.BoundString(ui.face, pre)
			caretX := int((b.Max.X - b.Min.X) >> 6)
			ascent := ui.face.Metrics().Ascent.Round()
			descent := ui.face.Metrics().Descent.Round()
			caretH := ascent + descent
			caretY := 6
			ebitenutil.DrawRect(screen, float64(padding+caretX), float64(caretY), 2, float64(caretH), color.White)
		}

		// position editing text over the selected cell (visual feedback)
		if g.activePanel >= 0 && g.activePanel < len(g.canvas.panels) {
			p := g.canvas.panels[g.activePanel]
			sx := float64(p.X) + g.canvas.camX + float64(g.selCol*p.CellW)
			sy := float64(p.Y) + g.canvas.camY + float64(g.selRow*p.CellH)
			drawTextAt(screen, ui.face, g.editBuffer, int(sx)+PanelInnerPadding, int(sy)+PanelInnerPadding, color.White)
		}
	}

	// Draw recent mouse click log at bottom-right
	if len(ui.clickLog) > 0 {
		sw := screen.Bounds().Dx()
		sh := screen.Bounds().Dy()
		boxW := 352
		boxH := len(ui.clickLog)*16 + 8
		x := sw - boxW - 8
		y := sh - boxH - 8
		// background box
		ebitenutil.DrawRect(screen, float64(x-8), float64(y-6), float64(boxW+16), float64(boxH+12), color.RGBA{0x0c, 0x0c, 0x0e, 0xee})
		for i, line := range ui.clickLog {
			drawTextAt(screen, ui.face, line, x, y+i*14, color.RGBA{0xdd, 0xdd, 0xdd, 0xff})
		}
	}

	// draw right-click context menu if visible
	g.contextMenu.Draw(screen, ui.face)
}

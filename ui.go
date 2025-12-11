package main

import (
	"fmt"
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
	// double-click tracking for header name button
	lastClickHeaderPanel int
	lastClickHeaderTime  int64
}

func NewUI() *UI {
	ui := &UI{}
	ui.lastClickPanel = -1
	ui.lastClickRow = -1
	ui.lastClickCol = -1
	ui.lastClickTime = 0
	ui.dblClickMs = 400

	ui.clickLog = []string{}
	ui.lastClickHeaderPanel = -1
	ui.lastClickHeaderTime = 0

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
	ui.handleClickLogging(g)
	ui.handleShortcuts(g)

	// Early return if not editing
	if !g.input.editing && !g.input.editingPanelName {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if g.input.activePanel >= 0 && g.input.activePanel < len(g.canvas.panels) {
				g.input.editing = true
				g.input.editBuffer = g.canvas.panels[g.input.activePanel].GetCell(g.input.selCol, g.input.selRow)
				g.input.editCursor = len([]rune(g.input.editBuffer))
				g.input.blinkCounter = 0
				g.input.caretVisible = true
			}
		}
		return
	}

	ui.handleCaretBlink(g)
	ui.handleTextInput(g)
	ui.handleEditingNavigation(g)
	ui.handleCommitCancel(g)
}

// handleClickLogging logs mouse clicks with panel/cell information
func (ui *UI) handleClickLogging(g *Game) {
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
				if !p.Loaded {
					ui.addClickLog(fmt.Sprintf("%s click @ %d,%d  panel=%d (loading)", btn, mx, my, i))
					found = true
					break
				}
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
}

// handleShortcuts processes global keyboard shortcuts (Ctrl+S, Ctrl+O)
func (ui *UI) handleShortcuts(g *Game) {
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
}

// handleCaretBlink updates the caret blink timer
func (ui *UI) handleCaretBlink(g *Game) {
	g.input.blinkCounter++
	if g.input.blinkCounter%30 == 0 {
		g.input.caretVisible = !g.input.caretVisible
	}
}

// handleTextInput processes typed characters and inserts them at cursor position
func (ui *UI) handleTextInput(g *Game) {
	for _, r := range ebiten.InputChars() {
		if r == '\b' {
			if g.input.editingPanelName {
				if g.input.editPanelCursor > 0 {
					rs := []rune(g.input.editPanelBuffer)
					rs = append(rs[:g.input.editPanelCursor-1], rs[g.input.editPanelCursor:]...)
					g.input.editPanelBuffer = string(rs)
					g.input.editPanelCursor--
					ui.resetCaret(g)
				}
			} else {
				if g.input.editCursor > 0 {
					rs := []rune(g.input.editBuffer)
					rs = append(rs[:g.input.editCursor-1], rs[g.input.editCursor:]...)
					g.input.editBuffer = string(rs)
					g.input.editCursor--
					ui.resetCaret(g)
				}
			}
		} else {
			if g.input.editingPanelName {
				rs := []rune(g.input.editPanelBuffer)
				rs = append(rs[:g.input.editPanelCursor], append([]rune{r}, rs[g.input.editPanelCursor:]...)...)
				g.input.editPanelBuffer = string(rs)
				g.input.editPanelCursor++
				ui.resetCaret(g)
			} else {
				rs := []rune(g.input.editBuffer)
				rs = append(rs[:g.input.editCursor], append([]rune{r}, rs[g.input.editCursor:]...)...)
				g.input.editBuffer = string(rs)
				g.input.editCursor++
				ui.resetCaret(g)
			}
		}
	}
}

// handleEditingNavigation processes arrow keys, backspace, delete, home, end
func (ui *UI) handleEditingNavigation(g *Game) {
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		if g.input.editingPanelName {
			if g.input.editPanelCursor > 0 {
				g.input.editPanelCursor--
				ui.resetCaret(g)
			}
		} else {
			if g.input.editCursor > 0 {
				g.input.editCursor--
				ui.resetCaret(g)
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		if g.input.editingPanelName {
			if g.input.editPanelCursor < len([]rune(g.input.editPanelBuffer)) {
				g.input.editPanelCursor++
				ui.resetCaret(g)
			}
		} else {
			if g.input.editCursor < len([]rune(g.input.editBuffer)) {
				g.input.editCursor++
				ui.resetCaret(g)
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if g.input.editingPanelName {
			if g.input.editPanelCursor > 0 {
				rs := []rune(g.input.editPanelBuffer)
				rs = append(rs[:g.input.editPanelCursor-1], rs[g.input.editPanelCursor:]...)
				g.input.editPanelBuffer = string(rs)
				g.input.editPanelCursor--
				ui.resetCaret(g)
			}
		} else {
			if g.input.editCursor > 0 {
				rs := []rune(g.input.editBuffer)
				rs = append(rs[:g.input.editCursor-1], rs[g.input.editCursor:]...)
				g.input.editBuffer = string(rs)
				g.input.editCursor--
				ui.resetCaret(g)
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		if g.input.editingPanelName {
			rs := []rune(g.input.editPanelBuffer)
			if g.input.editPanelCursor < len(rs) {
				rs = append(rs[:g.input.editPanelCursor], rs[g.input.editPanelCursor+1:]...)
				g.input.editPanelBuffer = string(rs)
				ui.resetCaret(g)
			}
		} else {
			rs := []rune(g.input.editBuffer)
			if g.input.editCursor < len(rs) {
				rs = append(rs[:g.input.editCursor], rs[g.input.editCursor+1:]...)
				g.input.editBuffer = string(rs)
				ui.resetCaret(g)
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		if g.input.editingPanelName {
			g.input.editPanelCursor = 0
		} else {
			g.input.editCursor = 0
		}
		ui.resetCaret(g)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
		if g.input.editingPanelName {
			g.input.editPanelCursor = len([]rune(g.input.editPanelBuffer))
		} else {
			g.input.editCursor = len([]rune(g.input.editBuffer))
		}
		ui.resetCaret(g)
	}
}

// handleCommitCancel processes Enter and Escape keys to commit or cancel editing
func (ui *UI) handleCommitCancel(g *Game) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if g.input.editingPanelName {
			if g.input.editPanelIndex >= 0 && g.input.editPanelIndex < len(g.canvas.panels) {
				oldName := g.canvas.panels[g.input.editPanelIndex].Name
				g.canvas.panels[g.input.editPanelIndex].Name = g.input.editPanelBuffer
				if ui != nil && oldName != g.canvas.panels[g.input.editPanelIndex].Name {
					if g.canvas.panels[g.input.editPanelIndex].Name == "" {
						ui.addClickLog(fmt.Sprintf("Panel %d name cleared", g.input.editPanelIndex+1))
					} else {
						ui.addClickLog(fmt.Sprintf("Panel %d named: %s", g.input.editPanelIndex+1, g.canvas.panels[g.input.editPanelIndex].Name))
					}
				}
			}
			g.input.editingPanelName = false
		} else {
			if g.input.activePanel >= 0 && g.input.activePanel < len(g.canvas.panels) {
				g.canvas.panels[g.input.activePanel].SetCell(g.input.selCol, g.input.selRow, g.input.editBuffer)
			}
			g.input.editing = false
		}
	}
	// Only cancel editing with ESC if context menu is not visible
	// (context menu handles ESC first to close itself)
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && !g.contextMenu.visible {
		g.input.editing = false
		g.input.editingPanelName = false
	}
}

// resetCaret resets the caret blink timer and makes it visible
func (ui *UI) resetCaret(g *Game) {
	g.input.blinkCounter = 0
	g.input.caretVisible = true
}

// OnCellClick handles click events on a cell and detects double-clicks to begin editing.
// Single-click commits any active edits and selects the cell.
// Double-click starts editing the cell.
func (ui *UI) OnCellClick(g *Game, panel, row, col int) {
	// First, commit any active cell edit
	if g.input.editing && !g.input.editingPanelName {
		if g.input.activePanel >= 0 && g.input.activePanel < len(g.canvas.panels) {
			g.canvas.panels[g.input.activePanel].SetCell(g.input.selCol, g.input.selRow, g.input.editBuffer)
		}
		g.input.editing = false
	}

	now := time.Now().UnixNano() / 1e6
	if ui.lastClickPanel == panel && ui.lastClickRow == row && ui.lastClickCol == col && now-ui.lastClickTime <= ui.dblClickMs {
		// double-click: start editing
		if panel >= 0 && panel < len(g.canvas.panels) {
			g.input.editing = true
			g.input.editBuffer = g.canvas.panels[panel].GetCell(col, row)
			g.input.editCursor = len([]rune(g.input.editBuffer))
			g.input.blinkCounter = 0
			g.input.caretVisible = true
		}
		// reset last click to avoid immediate retrigger
		ui.lastClickPanel = -1
	} else {
		// single-click: just update selection (edits already committed above)
		ui.lastClickPanel = panel
		ui.lastClickRow = row
		ui.lastClickCol = col
		ui.lastClickTime = now
	}
}

// OnPanelNameClick handles clicks on the header name button and detects double-clicks
// to start editing the panel name.
func (ui *UI) OnPanelNameClick(g *Game, panel int) {
	now := time.Now().UnixNano() / 1e6
	if ui.lastClickHeaderPanel == panel && now-ui.lastClickHeaderTime <= ui.dblClickMs {
		// double-click: start editing panel name
		if panel >= 0 && panel < len(g.canvas.panels) {
			g.input.editingPanelName = true
			g.input.editPanelIndex = panel
			g.input.editPanelBuffer = g.canvas.panels[panel].Name
			g.input.editPanelCursor = len([]rune(g.input.editPanelBuffer))
			g.input.blinkCounter = 0
			g.input.caretVisible = true
		}
		// reset last click to avoid immediate retrigger
		ui.lastClickHeaderPanel = -1
	} else {
		ui.lastClickHeaderPanel = panel
		ui.lastClickHeaderTime = now
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
	drawTextAt(screen, ui.face, "Right-drag to pan - Left-drag title to move - Drag corner to resize", 8, screenH-42, ColorText)
	drawTextAt(screen, ui.face, "Press Ctrl+S to Save - Press Ctrl+O to Open", 8, screenH-28, ColorText)
	drawTextAt(screen, ui.face, "Arrows to move - Enter to edit - Tab switch panel", 8, screenH-14, ColorText)

	if g.input.editing { // only show top overlay when editing a cell; panel name edits render inline
		// top text bar background
		sw := screen.Bounds().Dx()
		ebitenutil.DrawRect(screen, 0, 0, float64(sw), 34, ColorOverlayBg)
		padding := 8

		// build a label like: "Edit Panel0 Cell-A1 : " or "Edit PanelX Name : "
		label := ""
		if g.input.editingPanelName {
			label = fmt.Sprintf("Edit Panel%d Name : ", g.input.editPanelIndex)
		} else {
			label = fmt.Sprintf("Edit Panel%d Cell-%s%d : ", g.input.activePanel, ColToLetters(g.input.selCol), g.input.selRow+1)
		}

		// render the full bracketed string
		full := label + "["
		if g.input.editingPanelName {
			full += g.input.editPanelBuffer
		} else {
			full += g.input.editBuffer
		}
		full += "]"
		drawTextAt(screen, ui.face, full, padding, 6, ColorText)

		// draw caret if visible (measured relative to the full label)
		if g.input.caretVisible {
			if g.input.editingPanelName {
				rs := []rune(g.input.editPanelBuffer)
				if g.input.editPanelCursor < 0 {
					g.input.editPanelCursor = 0
				}
				if g.input.editPanelCursor > len(rs) {
					g.input.editPanelCursor = len(rs)
				}
				pre := label + "[" + string(rs[:g.input.editPanelCursor])
				b, _ := font.BoundString(ui.face, pre)
				caretX := int((b.Max.X - b.Min.X) >> 6)
				ascent := ui.face.Metrics().Ascent.Round()
				descent := ui.face.Metrics().Descent.Round()
				caretH := ascent + descent
				caretY := 6
				ebitenutil.DrawRect(screen, float64(padding+caretX), float64(caretY), 2, float64(caretH), ColorText)
			} else {
				rs := []rune(g.input.editBuffer)
				if g.input.editCursor < 0 {
					g.input.editCursor = 0
				}
				if g.input.editCursor > len(rs) {
					g.input.editCursor = len(rs)
				}
				pre := label + "[" + string(rs[:g.input.editCursor])
				b, _ := font.BoundString(ui.face, pre)
				caretX := int((b.Max.X - b.Min.X) >> 6)
				ascent := ui.face.Metrics().Ascent.Round()
				descent := ui.face.Metrics().Descent.Round()
				caretH := ascent + descent
				caretY := 6
				ebitenutil.DrawRect(screen, float64(padding+caretX), float64(caretY), 2, float64(caretH), ColorText)
			}
		}

		// position editing text over the selected cell (visual feedback)
		// Position editing text over the selected cell only for normal cell edits.
		if !g.input.editingPanelName {
			if g.input.activePanel >= 0 && g.input.activePanel < len(g.canvas.panels) {
				p := g.canvas.panels[g.input.activePanel]
				sx := float64(p.X) + g.canvas.camX + float64(g.input.selCol*p.CellW)
				sy := float64(p.Y) + g.canvas.camY + float64(g.input.selRow*p.CellH)
				drawTextAt(screen, ui.face, g.input.editBuffer, int(sx)+PanelInnerPadding, int(sy)+PanelInnerPadding, ColorText)
			}
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
		ebitenutil.DrawRect(screen, float64(x-8), float64(y-6), float64(boxW+16), float64(boxH+12), ColorLogBg)
		for i, line := range ui.clickLog {
			drawTextAt(screen, ui.face, line, x, y+i*14, ColorTextDim)
		}
	}

	// draw right-click context menu if visible
	g.contextMenu.Draw(screen, ui.face)
}

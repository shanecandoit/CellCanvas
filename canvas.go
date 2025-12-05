package main

import (
	"fmt"
	"image/color"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

type Panel struct {
	X, Y     int
	Cols     int
	Rows     int
	CellW    int
	CellH    int
	Cells    map[string]string // sparse map of cells keyed A1-style
	Filename string
	Loaded   bool
}

// panelGap is the minimum spacing (in pixels) to keep between panels.
// This uses PanelPaddingX to calculate spacing.
const panelGap = PanelPaddingX

// axisHysteresis prevents immediate axis switching; the other axis must have
// a violation larger by this many pixels before we switch to it.
const axisHysteresis = 2

// Canvas contains panels and camera + interaction state for moving/resizing
type Canvas struct {
	panels     []Panel
	camX, camY float64

	movingPanel   int
	resizingPanel int
	moveOffsetX   int
	moveOffsetY   int
	// SaveManager manages background CSV loads and applies them on the UI thread.
	saveManager *SaveManager
}

func NewCanvas() *Canvas {
	c := &Canvas{movingPanel: -1, resizingPanel: -1}
	// Start with no sample/demo panels by default. Panels will be created
	// by state load or user actions.
	c.panels = []Panel{}
	c.saveManager = NewSaveManager()
	return c
}

// RemovePanelAt removes the panel at index i from the canvas if valid.
// It does not delete any files on disk; it simply drops the panel from
// the canvas slice so SaveState will no longer reference it.
func (c *Canvas) RemovePanelAt(i int) {
	if i < 0 || i >= len(c.panels) {
		return
	}
	c.panels = append(c.panels[:i], c.panels[i+1:]...)
}

// drawTextAt draws text using the provided face. If face is nil, falls back to ebitenutil.DebugPrintAt.
func drawTextAt(screen *ebiten.Image, face font.Face, s string, x, y int, col color.Color) {
	if face == nil {
		ebitenutil.DebugPrintAt(screen, s, x, y)
		return
	}
	// text.Draw expects y to be baseline; DebugPrintAt uses top-left.
	// Adjust by ascent so text appears where DebugPrintAt placed it.
	ascent := face.Metrics().Ascent.Round()
	text.Draw(screen, s, face, x, y+ascent, col)
}

// Update handles panel interactions: picking, moving, resizing, selection
func (c *Canvas) Update(g *Game) {
	// Process background loads completed by SaveManager (keeps canvas free
	// of channel handling).
	if c.saveManager != nil {
		c.saveManager.ApplyPending(c, g)
	}
	mx, my := ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// check panels from top (last) to bottom (first)
		picked := -1
		for i := len(c.panels) - 1; i >= 0; i-- {
			p := c.panels[i]
			b := p.GetBounds(c.camX, c.camY)
			baseX := b.ContentX
			baseY := b.ContentY
			w := b.ContentW
			h := b.ContentH
			// header area (title bar)
			headerY := baseY - PanelHeaderHeight
			if mx >= baseX && mx <= baseX+w && my >= headerY && my <= headerY+PanelHeaderHeight {
				picked = i
				// start moving
				c.movingPanel = i
				c.moveOffsetX = mx - baseX
				c.moveOffsetY = my - baseY
				break
			}
			// resize corner (bottom-right 16x16)
			if mx >= baseX+w-ResizeHandleSize && mx <= baseX+w && my >= baseY+h-ResizeHandleSize && my <= baseY+h {
				picked = i
				c.resizingPanel = i
				c.moveOffsetX = mx - (baseX + w)
				c.moveOffsetY = my - (baseY + h)
				break
			}
			// click inside panel -> select panel and a cell (only when loaded)
			if mx >= baseX && mx <= baseX+w && my >= baseY && my <= baseY+h {
				if !p.Loaded {
					// ignore clicks inside placeholder panels
					continue
				}
				picked = i
				g.activePanel = i
				// compute selected cell
				cx := mx - baseX
				cy := my - baseY
				col := cx / p.CellW
				row := cy / p.CellH
				if row >= 0 && row < p.Rows && col >= 0 && col < p.Cols {
					g.selRow = row
					g.selCol = col
					// notify UI about the click so it can decide double-click
					if g.ui != nil {
						g.ui.OnCellClick(g, i, row, col)
					}
				}
				break
			}
		}
		_ = picked
	}

	// dragging move
	if c.movingPanel != -1 && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		i := c.movingPanel
		mx, my := ebiten.CursorPosition()
		// new panel origin is cursor minus offset, adjusted by camera
		newX := mx - c.moveOffsetX - int(c.camX)
		newY := my - c.moveOffsetY - int(c.camY)
		c.panels[i].X = newX
		c.panels[i].Y = newY
	}
	// dragging resize
	if c.resizingPanel != -1 && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		i := c.resizingPanel
		mx, my := ebiten.CursorPosition()
		b := c.panels[i].GetBounds(c.camX, c.camY)
		baseX := b.ContentX
		baseY := b.ContentY
		// compute width/height from base to cursor (minus offset)
		w := (mx - baseX - c.moveOffsetX)
		h := (my - baseY - c.moveOffsetY)
		if w < 64 {
			w = 64
		}
		if h < 32 {
			h = 32
		}
		// determine cols/rows from new size
		cols := w / c.panels[i].CellW
		rows := h / c.panels[i].CellH
		if cols < 1 {
			cols = 1
		}
		if rows < 1 {
			rows = 1
		}
		// resize cell grid preserving existing data where possible
		old := c.panels[i]
		newCells := make(map[string]string)
		for key, val := range old.Cells {
			col, row, err := ParseCellRef(key)
			if err != nil {
				continue
			}
			if row >= 0 && row < rows && col >= 0 && col < cols {
				newCells[CellRef(col, row)] = val
			}
		}
		c.panels[i].Cols = cols
		c.panels[i].Rows = rows
		c.panels[i].Cells = newCells
	}

	// release move/resize when mouse released
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		c.movingPanel = -1
		c.resizingPanel = -1
	}

	// resolve a single overlapping panel pair by moving one panel one pixel
	// (skip panels currently being moved/resized by the user)
	c.resolveOneOverlap()
}

// resolveOneOverlap finds the first overlapping panel pair (not being
// actively moved/resized) and moves one panel by a single pixel away from
// the other along the axis of least overlap. This helps separate multiple
// overlapping panels gradually (one pair, one pixel per update).
func (c *Canvas) resolveOneOverlap() {
	for i := 0; i < len(c.panels); i++ {
		// skip if this panel is being interacted with
		if i == c.movingPanel || i == c.resizingPanel {
			continue
		}
		a := c.panels[i]
		aLeft := a.X - PanelPaddingX
		aW := a.Cols*a.CellW + PanelPaddingX*2

		for j := i + 1; j < len(c.panels); j++ {
			// skip if this panel is being interacted with
			if j == c.movingPanel || j == c.resizingPanel {
				continue
			}
			b := c.panels[j]
			bLeft := b.X - PanelPaddingX
			bW := b.Cols*b.CellW + PanelPaddingX*2

			// Only perform horizontal separation: compute X overlap/gap and
			// move panel j by 1 pixel along X away from panel i when the
			// horizontal separation is less than `panelGap` (or panels overlap).
			aRight := aLeft + aW
			bRight := bLeft + bW

			// compute overlapX (positive if overlapping)
			overlapX := min(aRight, bRight) - max(aLeft, bLeft)

			// compute gap when not overlapping
			var gapX int
			if aRight < bLeft {
				gapX = bLeft - aRight
			} else if bRight < aLeft {
				gapX = aLeft - bRight
			} else {
				gapX = -overlapX
			}

			// determine horizontal violation (how much we need to move to reach panelGap)
			var violX int
			if overlapX > 0 {
				violX = overlapX + panelGap
			} else if gapX < panelGap {
				violX = panelGap - gapX
			}

			if violX <= 0 {
				// no horizontal work needed
				continue
			}

			// Only separate panels that actually overlap vertically as well.
			// Compute vertical bounds used elsewhere when drawing (title/header included)
			aTop := a.Y - PanelHeaderHeight
			aH := a.Rows*a.CellH + PanelHeaderHeight + PanelPaddingY*2
			bTop := b.Y - PanelHeaderHeight
			bH := b.Rows*b.CellH + PanelHeaderHeight + PanelPaddingY*2
			overlapY := min(aTop+aH, bTop+bH) - max(aTop, bTop)
			if overlapY <= 0 {
				// panels are vertically separated (one is above/below the other)
				// do not perform horizontal separation in this case
				continue
			}

			// centers to choose direction to move b away from a
			aCx := aLeft + aW/2
			bCx := bLeft + bW/2

			// move along X by 1 pixel away from A
			if aCx < bCx {
				c.panels[j].X += 1
			} else {
				c.panels[j].X -= 1
			}

			// Only move a single pair by a single pixel per Update
			return
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Draw renders the panels; selection overlay is drawn here but editing text is handled by UI
func (c *Canvas) Draw(screen *ebiten.Image, g *Game) {
	for pi, p := range c.panels {
		b := p.GetBounds(c.camX, c.camY)
		baseX := float64(b.ContentX)
		baseY := float64(b.ContentY)

		// panel background
		ebitenutil.DrawRect(screen, float64(b.TotalX), float64(b.TotalY), float64(b.TotalW), float64(b.TotalH), color.RGBA{0x22, 0x22, 0x2a, 0xff})

		// panel title
		drawTextAt(screen, g.ui.face, fmt.Sprintf("Panel %d", pi+1), int(baseX)+PanelInnerPadding, int(baseY-PanelHeaderHeight+2), color.White)

		// draw border
		// draw the 4 border edges using total bounds
		ebitenutil.DrawRect(screen, float64(b.TotalX), float64(b.TotalY), float64(PanelBorderWidth), float64(b.TotalH), color.RGBA{0x44, 0x44, 0x50, 0xff})
		ebitenutil.DrawRect(screen, float64(b.TotalX), float64(b.TotalY), float64(b.TotalW), float64(PanelBorderWidth), color.RGBA{0x44, 0x44, 0x50, 0xff})
		ebitenutil.DrawRect(screen, float64(b.TotalX+b.TotalW-PanelBorderWidth), float64(b.TotalY), float64(PanelBorderWidth), float64(b.TotalH), color.RGBA{0x44, 0x44, 0x50, 0xff})
		ebitenutil.DrawRect(screen, float64(b.TotalX), float64(b.TotalY+b.TotalH-PanelBorderWidth), float64(b.TotalW), float64(PanelBorderWidth), color.RGBA{0x44, 0x44, 0x50, 0xff})

		// If panel isn't loaded yet, draw a loading placeholder and skip
		// drawing cell content.
		if !p.Loaded {
			// placeholder: a darker rectangle and 'Loading...' title
			ebitenutil.DrawRect(screen, float64(b.ContentX), float64(b.ContentY), float64(b.ContentW), float64(b.ContentH), color.RGBA{0x0f, 0x0f, 0x12, 0xff})
			drawTextAt(screen, g.ui.face, "Loading...", int(baseX)+PanelInnerPadding, int(baseY)+PanelInnerPadding, color.White)
		} else {
			for r := 0; r < p.Rows; r++ {
				for ccol := 0; ccol < p.Cols; ccol++ {
					x := baseX + float64(ccol*p.CellW)
					y := baseY + float64(r*p.CellH)
					// cell bg (slightly lighter card)
					ebitenutil.DrawRect(screen, x, y, float64(p.CellW-1), float64(p.CellH-1), color.RGBA{0x18, 0x18, 0x1c, 0xff})
					// cell text (light)
					// if this is the active cell being edited, show the live edit buffer
					key := CellRef(ccol, r)
					txt := ""
					if v, ok := p.Cells[key]; ok {
						txt = v
					}
					if g != nil && g.editing && g.activePanel == pi && r == g.selRow && ccol == g.selCol {
						txt = g.editBuffer
					}
					drawTextAt(screen, g.ui.face, txt, int(x)+PanelInnerPadding, int(y)+PanelInnerPadding, color.White)
				}
			}
		}

		// draw selection for active panel
		if pi == g.activePanel {
			sx := baseX + float64(g.selCol*p.CellW)
			sy := baseY + float64(g.selRow*p.CellH)
			// selection overlay
			ebitenutil.DrawRect(screen, sx, sy, float64(p.CellW-1), float64(p.CellH-1), color.RGBA{0x66, 0x88, 0xff, 0x66})
		}

		// draw resize handle
		rx := baseX + float64(p.Cols*p.CellW) - ResizeHandleSize
		ry := baseY + float64(p.Rows*p.CellH) - ResizeHandleSize
		ebitenutil.DrawRect(screen, rx, ry, float64(ResizeHandleSize), float64(ResizeHandleSize), color.RGBA{0x55, 0x55, 0x66, 0xff})
	}
}

func NewPanel(x, y, cols, rows int) Panel {
	// NewPanel creates a panel with default empty content. We no longer
	// pre-fill sample values so newly created panels are blank.
	return Panel{X: x, Y: y, Cols: cols, Rows: rows, CellW: defaultCellW, CellH: defaultCellH, Cells: make(map[string]string), Filename: "", Loaded: true}
}

// NewBlankPanel creates a panel with provided dimensions but no cell content
// (empty map). Use this for freshly created blank panels via the UI.
func NewBlankPanel(x, y, cols, rows int) Panel {
	return Panel{X: x, Y: y, Cols: cols, Rows: rows, CellW: defaultCellW, CellH: defaultCellH, Cells: make(map[string]string), Filename: "", Loaded: true}
}

// GetCell returns the string stored at the given col,row (zero-based).
// Returns empty string when not present.
func (p *Panel) GetCell(col, row int) string {
	if p == nil {
		return ""
	}
	if p.Cells == nil {
		return ""
	}
	key := CellRef(col, row)
	if v, ok := p.Cells[key]; ok {
		return v
	}
	return ""
}

// SetCell writes a value at the given col,row. Empty values remove the entry
// to keep the structure sparse.
func (p *Panel) SetCell(col, row int, val string) {
	if p == nil {
		return
	}
	if p.Cells == nil {
		p.Cells = make(map[string]string)
	}
	key := CellRef(col, row)
	if val == "" {
		delete(p.Cells, key)
	} else {
		p.Cells[key] = val
	}
}

// AddPanelAt appends a new blank panel positioned at given world coordinates
func (c *Canvas) AddPanelAt(x, y int) {
	// new panels are 5x5 blank by default
	p := NewBlankPanel(x, y, 5, 5)
	p.X = x
	p.Y = y
	c.panels = append(c.panels, p)
}

// AddPanelFromCSV loads a CSV file into a new panel positioned at x,y.
// Returns an error if loading the CSV fails.
func (c *Canvas) AddPanelFromCSV(path string, x, y int) error {
	p := NewBlankPanel(x, y, 1, 1)
	if err := loadPanelCSV(path, &p); err != nil {
		return err
	}
	// store filename as base name
	p.Filename = filepath.Base(path)
	p.X = x
	p.Y = y
	p.Loaded = true
	c.panels = append(c.panels, p)
	return nil
}

// scheduleBackgroundLoad is implemented in model.go so CSV loading
// and related data handling remain grouped with the model I/O code.

package main

import (
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
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
	Name     string
}

// panelGap is the minimum spacing (in pixels) to keep between panels.
// This uses PanelPaddingX to calculate spacing.
const panelGap = PanelPaddingX

// panel header center name button defaults
// panel header center name button defaults are now in theme.go

// axisHysteresis prevents immediate axis switching; the other axis must have
// a violation larger by this many pixels before we switch to it.
const axisHysteresis = 2

// Canvas contains panels and camera + interaction state for moving/resizing
type Canvas struct {
	panels     []Panel
	camX, camY float64
	// SaveManager manages background CSV loads and applies them on the UI thread.
	saveManager *SaveManager
}

// CanvasDrawState encapsulates all external state required to render the canvas.
// This decouples the Canvas.Draw method from the monolithic Game struct.
type CanvasDrawState struct {
	Face             font.Face
	ActivePanel      int
	SelRow, SelCol   int
	Editing          bool
	EditBuffer       string
	EditingPanelName bool
	EditPanelIndex   int
	EditPanelBuffer  string
	CaretVisible     bool
	EditCursor       int
	EditPanelCursor  int
}

func NewCanvas() *Canvas {
	c := &Canvas{}
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

// Update handles background loads and overlap resolution.
// Interaction logic has been moved to InputManager.HandleCanvasInteraction.
func (c *Canvas) Update(g *Game, lockedPanels map[int]bool) {
	// Process background loads completed by SaveManager (keeps canvas free
	// of channel handling).
	if c.saveManager != nil {
		c.saveManager.ApplyPending(c, func(msg string) {
			if g.ui != nil {
				g.ui.addClickLog(msg)
			}
		})
	}

	// resolve a single overlapping panel pair by moving one panel one pixel
	// (skip panels currently being moved/resized by the user)
	c.resolveOneOverlap(lockedPanels)
}

// resolveOneOverlap finds the first overlapping panel pair (not being
// actively moved/resized) and moves one panel by a single pixel away from
// the other along the axis of least overlap. This helps separate multiple
// overlapping panels gradually (one pair, one pixel per update).
func (c *Canvas) resolveOneOverlap(lockedPanels map[int]bool) {
	for i := 0; i < len(c.panels); i++ {
		// skip if this panel is being interacted with
		if lockedPanels[i] {
			continue
		}
		a := c.panels[i]
		aLeft := a.X - PanelPaddingX
		aW := a.Cols*a.CellW + PanelPaddingX*2

		for j := i + 1; j < len(c.panels); j++ {
			// skip if this panel is being interacted with
			if lockedPanels[j] {
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
func (c *Canvas) Draw(screen *ebiten.Image, state CanvasDrawState) {
	// Delegate drawing to the Renderer
	renderer := NewRenderer()
	renderer.DrawCanvas(screen, c, state)
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

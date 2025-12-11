package main

import (
	"log"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/sqweek/dialog"
)

// InputManager handles input detection and updates game state accordingly.
// It owns transient input-related state like drag tracking.
// The Game retains higher-level state like selection and editing; the
// ContextMenu manages its own visibility and selection state.
type InputManager struct {
	dragging      bool
	lastMouseX    int
	lastMouseY    int
	rightPressedX int
	rightPressedY int

	// Canvas interaction state
	movingPanel   int
	resizingPanel int
	moveOffsetX   int
	moveOffsetY   int

	// selection (moved from Game)
	activePanel    int
	selRow, selCol int

	// editing (moved from Game)
	editing      bool
	editBuffer   string
	editCursor   int
	blinkCounter int
	caretVisible bool
	// panel name editing
	editingPanelName bool
	editPanelBuffer  string
	editPanelCursor  int
	editPanelIndex   int
}

func NewInputManager() *InputManager {
	return &InputManager{
		movingPanel:      -1,
		resizingPanel:    -1,
		activePanel:      0,
		selRow:           0,
		selCol:           0,
		editingPanelName: false,
		editPanelIndex:   -1,
	}
}

func (im *InputManager) HandlePanInput(g *Game) {
	// start/stop dragging with right mouse
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		im.dragging = true
		im.lastMouseX, im.lastMouseY = ebiten.CursorPosition()
		im.rightPressedX, im.rightPressedY = im.lastMouseX, im.lastMouseY
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonRight) {
		// determine if this was a click (small movement) or a drag
		mx, my := ebiten.CursorPosition()
		dx := mx - im.rightPressedX
		dy := my - im.rightPressedY
		// small threshold -> treat as click and open context menu
		if abs(dx) < 6 && abs(dy) < 6 {
			// toggle context menu at cursor
			// Determine which panel (if any) was clicked so menu actions can act on it.
			target := -1
			for i := len(g.canvas.panels) - 1; i >= 0; i-- {
				p := g.canvas.panels[i]
				b := p.GetBounds(g.canvas.camX, g.canvas.camY)
				baseX := b.ContentX
				baseY := b.ContentY
				w := b.ContentW
				h := b.ContentH
				headerY := baseY - PanelHeaderHeight
				if mx >= baseX && mx <= baseX+w && my >= headerY && my <= headerY+h {
					target = i
					break
				}
			}
			g.contextMenu.Show(mx, my, target)
		}
		im.dragging = false
	}
	if im.dragging && ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		mx, my := ebiten.CursorPosition()
		dx := mx - im.lastMouseX
		dy := my - im.lastMouseY
		g.canvas.camX += float64(dx)
		g.canvas.camY += float64(dy)
		im.lastMouseX = mx
		im.lastMouseY = my
	}
}

func (im *InputManager) HandleContextMenuInput(g *Game) {
	// Give the menu a chance to update and return an action
	action := g.contextMenu.Update(g)
	switch action {
	case MenuActionNone:
		// nothing to do
	case MenuActionNewBlankPanel:
		// compute world coords (screen - cam)
		wx := int(float64(g.contextMenu.x) - g.canvas.camX)
		wy := int(float64(g.contextMenu.y) - g.canvas.camY)
		g.canvas.AddPanelAt(wx, wy)
	case MenuActionLoadPanelFromFile:
		// Determine which panel to load into: context menu target or active panel
		target := g.contextMenu.targetPanel
		if target < 0 {
			target = im.activePanel
		}
		// ask for a CSV file
		path, err := dialog.File().Filter("CSV", "csv").Title("Load Panel CSV").Load()
		if err != nil {
			if err != dialog.ErrCancelled {
				log.Printf("file open failed: %v", err)
			}
			break
		}
		if path == "" {
			break
		}
		absPath, _ := filepath.Abs(path)
		if target < 0 {
			// create a new panel positioned at the context menu world coords
			wx := int(float64(g.contextMenu.x) - g.canvas.camX)
			wy := int(float64(g.contextMenu.y) - g.canvas.camY)
			if err := g.canvas.AddPanelFromCSV(absPath, wx, wy); err != nil {
				log.Printf("add panel from csv failed: %v", err)
				if g.ui != nil {
					g.ui.addClickLog("failed to add: " + filepath.Base(absPath))
				}
			} else {
				if g.ui != nil {
					g.ui.addClickLog("added panel: " + filepath.Base(absPath))
				}
			}
		} else {
			if g.canvas.saveManager != nil {
				g.canvas.saveManager.ScheduleLoad(target, absPath)
				if g.ui != nil {
					g.ui.addClickLog("scheduled load: " + filepath.Base(absPath))
				}
			} else {
				tmp := NewBlankPanel(0, 0, 1, 1)
				if err := loadPanelCSV(absPath, &tmp); err != nil {
					log.Printf("load failed: %v", err)
					if g.ui != nil {
						g.ui.addClickLog("failed to load: " + filepath.Base(absPath))
					}
				} else {
					tmp.X = g.canvas.panels[target].X
					tmp.Y = g.canvas.panels[target].Y
					tmp.Filename = filepath.Base(absPath)
					tmp.Loaded = true
					g.canvas.panels[target] = tmp
					if g.ui != nil {
						g.ui.addClickLog("loaded: " + tmp.Filename)
					}
				}
			}
		}
	case MenuActionSavePanelToFile, MenuActionExportPanelToCSV:
		// Determine the panel to save: context menu target or active panel
		target := g.contextMenu.targetPanel
		if target < 0 {
			target = im.activePanel
		}
		if target < 0 || target >= len(g.canvas.panels) {
			if g.ui != nil {
				g.ui.addClickLog("No panel to save")
			}
			break
		}
		path, err := dialog.File().Filter("CSV", "csv").Title("Save Panel As").Save()
		if err != nil {
			if err != dialog.ErrCancelled {
				log.Printf("file save failed: %v", err)
			}
			break
		}
		if path == "" {
			break
		}
		absPath, _ := filepath.Abs(path)
		if err := savePanelCSV(absPath, &g.canvas.panels[target]); err != nil {
			log.Printf("save failed: %v", err)
			if g.ui != nil {
				g.ui.addClickLog("failed to save: " + filepath.Base(absPath))
			}
		} else {
			// update the panel's filename (use relative path if in same directory)
			g.canvas.panels[target].Filename = filepath.Base(absPath)
			if g.ui != nil {
				g.ui.addClickLog("saved: " + g.canvas.panels[target].Filename)
			}
		}
	case MenuActionDeletePanel:
		// Determine the panel to delete: context menu target or active panel
		target := g.contextMenu.targetPanel
		if target < 0 {
			target = im.activePanel
		}
		if target < 0 || target >= len(g.canvas.panels) {
			if g.ui != nil {
				g.ui.addClickLog("No panel to delete")
			}
			break
		}
		name := g.canvas.panels[target].Filename
		// Remove the panel without touching any CSVs on disk
		g.canvas.RemovePanelAt(target)
		// adjust active panel selection
		if len(g.canvas.panels) == 0 {
			im.activePanel = 0
			im.selRow = 0
			im.selCol = 0
		} else if im.activePanel >= len(g.canvas.panels) {
			im.activePanel = len(g.canvas.panels) - 1
			im.selRow = 0
			im.selCol = 0
		}
		if g.ui != nil {
			if name == "" {
				g.ui.addClickLog("deleted panel")
			} else {
				g.ui.addClickLog("deleted panel: " + name)
			}
		}
	}
}

func (im *InputManager) HandleSelectionNavigation(g *Game) {
	// selection navigation (only when not editing)
	if im.editing {
		return
	}
	// guard: make sure active panel exists
	if im.activePanel < 0 || im.activePanel >= len(g.canvas.panels) {
		return
	}
	p := g.canvas.panels[im.activePanel]
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		if im.selRow > 0 {
			im.selRow--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		if im.selRow < p.Rows-1 {
			im.selRow++
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		if im.selCol > 0 {
			im.selCol--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		if im.selCol < p.Cols-1 {
			im.selCol++
		}
	}
}

func (im *InputManager) HandlePanelSwitching(g *Game) {
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		if len(g.canvas.panels) == 0 {
			return
		}
		im.activePanel = (im.activePanel + 1) % len(g.canvas.panels)
		im.selRow = 0
		im.selCol = 0
	}
}

func (im *InputManager) GetLockedPanels() map[int]bool {
	locked := make(map[int]bool)
	if im.movingPanel != -1 {
		locked[im.movingPanel] = true
	}
	if im.resizingPanel != -1 {
		locked[im.resizingPanel] = true
	}
	return locked
}

func (im *InputManager) HandleCanvasInteraction(g *Game) {
	c := g.canvas
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
			// detect header-centered name button clicks (centered horizontally)
			btnX := baseX + w/2 - PanelNameButtonW/2
			btnY := headerY + (PanelHeaderHeight-PanelNameButtonH)/2
			if mx >= btnX && mx <= btnX+PanelNameButtonW && my >= btnY && my <= btnY+PanelNameButtonH {
				// header name button clicked
				if g.ui != nil {
					g.ui.OnPanelNameClick(g, i)
				}
				picked = i
				break
			}

			if mx >= baseX && mx <= baseX+w && my >= headerY && my <= headerY+PanelHeaderHeight {
				picked = i
				// start moving
				im.movingPanel = i
				im.moveOffsetX = mx - baseX
				im.moveOffsetY = my - baseY
				break
			}
			// resize corner (bottom-right 16x16)
			if mx >= baseX+w-ResizeHandleSize && mx <= baseX+w && my >= baseY+h-ResizeHandleSize && my <= baseY+h {
				picked = i
				im.resizingPanel = i
				im.moveOffsetX = mx - (baseX + w)
				im.moveOffsetY = my - (baseY + h)
				break
			}
			// click inside panel -> select panel and a cell (only when loaded)
			if mx >= baseX && mx <= baseX+w && my >= baseY && my <= baseY+h {
				if !p.Loaded {
					// ignore clicks inside placeholder panels
					continue
				}
				picked = i
				im.activePanel = i
				// compute selected cell
				cx := mx - baseX
				cy := my - baseY
				col := cx / p.CellW
				row := cy / p.CellH
				if row >= 0 && row < p.Rows && col >= 0 && col < p.Cols {
					// notify UI about the click BEFORE updating selection
					// so OnCellClick can commit edits to the OLD cell position
					if g.ui != nil {
						g.ui.OnCellClick(g, i, row, col)
					}
					// NOW update the selection to the new cell
					im.selRow = row
					im.selCol = col
				}
				break
			}
		}
		_ = picked
	}

	// dragging move
	if im.movingPanel != -1 && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		i := im.movingPanel
		// new panel origin is cursor minus offset, adjusted by camera
		newX := mx - im.moveOffsetX - int(c.camX)
		newY := my - im.moveOffsetY - int(c.camY)
		c.panels[i].X = newX
		c.panels[i].Y = newY
	}
	// dragging resize
	if im.resizingPanel != -1 && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		i := im.resizingPanel
		b := c.panels[i].GetBounds(c.camX, c.camY)
		baseX := b.ContentX
		baseY := b.ContentY
		// compute width/height from base to cursor (minus offset)
		w := (mx - baseX - im.moveOffsetX)
		h := (my - baseY - im.moveOffsetY)
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
		im.movingPanel = -1
		im.resizingPanel = -1
	}
}

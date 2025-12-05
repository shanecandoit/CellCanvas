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
}

func NewInputManager() *InputManager {
	return &InputManager{}
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
			target = g.activePanel
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
			target = g.activePanel
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
	}
}

func (im *InputManager) HandleSelectionNavigation(g *Game) {
	// selection navigation (only when not editing)
	if g.editing {
		return
	}
	// guard: make sure active panel exists
	if g.activePanel < 0 || g.activePanel >= len(g.canvas.panels) {
		return
	}
	p := g.canvas.panels[g.activePanel]
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		if g.selRow > 0 {
			g.selRow--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		if g.selRow < p.Rows-1 {
			g.selRow++
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		if g.selCol > 0 {
			g.selCol--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		if g.selCol < p.Cols-1 {
			g.selCol++
		}
	}
}

func (im *InputManager) HandlePanelSwitching(g *Game) {
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		if len(g.canvas.panels) == 0 {
			return
		}
		g.activePanel = (g.activePanel + 1) % len(g.canvas.panels)
		g.selRow = 0
		g.selCol = 0
	}
}

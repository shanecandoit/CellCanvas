package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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
			g.contextMenu.Show(mx, my)
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
		// No hardcoded sample CSVs. LoadPanelFromFile requires a user-provided
		// filename; in this simple UI no filepicker is available, so this
		// action does nothing unless the code is extended to prompt for a
		// filename.
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

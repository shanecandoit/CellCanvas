package main

import (
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	windowWidth  = 1280
	windowHeight = 720
	defaultCellW = 80
	defaultCellH = 24
)

type Game struct {
	canvas *Canvas
	ui     *UI

	// dragging (right-button pan)
	dragging   bool
	lastMouseX int
	lastMouseY int

	// selection
	activePanel    int
	selRow, selCol int

	// editing
	editing      bool
	editBuffer   string
	editCursor   int
	blinkCounter int
	caretVisible bool

	// right-click context menu state
	rightPressedX   int
	rightPressedY   int
	contextVisible  bool
	contextX        int
	contextY        int
	contextSelected int
}

func NewGame() *Game {
	g := &Game{}
	g.canvas = NewCanvas()
	g.ui = NewUI()
	g.activePanel = 0
	g.selRow = 0
	g.selCol = 0
	return g
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func (g *Game) Update() error {
	// start/stop dragging with right mouse
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.dragging = true
		g.lastMouseX, g.lastMouseY = ebiten.CursorPosition()
		g.rightPressedX, g.rightPressedY = g.lastMouseX, g.lastMouseY
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonRight) {
		// determine if this was a click (small movement) or a drag
		mx, my := ebiten.CursorPosition()
		dx := mx - g.rightPressedX
		dy := my - g.rightPressedY
		// small threshold -> treat as click and open context menu
		if abs(dx) < 6 && abs(dy) < 6 {
			// toggle context menu at cursor
			g.contextVisible = true
			g.contextX = mx
			g.contextY = my
			g.contextSelected = -1
		}
		g.dragging = false
	}
	if g.dragging && ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		mx, my := ebiten.CursorPosition()
		dx := mx - g.lastMouseX
		dy := my - g.lastMouseY
		g.canvas.camX += float64(dx)
		g.canvas.camY += float64(dy)
		g.lastMouseX = mx
		g.lastMouseY = my
	}

	// delegate panel mouse interactions to canvas (it will update selection on Game)
	g.canvas.Update(g)

	// handle context menu interactions (hover and clicks)
	if g.contextVisible {
		mx, my := ebiten.CursorPosition()
		// layout
		itemH := 28
		w := 240
		x := g.contextX
		y := g.contextY
		// determine hover index
		if mx >= x && mx <= x+w && my >= y && my <= y+itemH*2 {
			idx := (my - y) / itemH
			if idx < 0 {
				idx = -1
			}
			g.contextSelected = idx
		} else {
			g.contextSelected = -1
		}

		// left click selects or closes
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// click inside menu -> activate
			if g.contextSelected >= 0 {
				switch g.contextSelected {
				case 0:
					// New Blank Panel at world coords
					// compute world coords (screen - cam)
					wx := int(float64(g.contextX) - g.canvas.camX)
					wy := int(float64(g.contextY) - g.canvas.camY)
					g.canvas.AddPanelAt(wx, wy)
				case 1:
					// Try to load panel CSV from sample files (panel_0.csv, panel_1.csv)
					// prefer panel_0.csv if exists
					if err := g.canvas.AddPanelFromCSV("panel_0.csv", int(float64(g.contextX)-g.canvas.camX), int(float64(g.contextY)-g.canvas.camY)); err != nil {
						_ = g.canvas.AddPanelFromCSV("panel_1.csv", int(float64(g.contextX)-g.canvas.camX), int(float64(g.contextY)-g.canvas.camY))
					}
				}
			}
			// close menu in any case
			g.contextVisible = false
		}
		// close menu on Escape
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.contextVisible = false
		}
		// if right-click again, close
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
			g.contextVisible = false
		}
	}

	// selection navigation (only when not editing)
	if !g.editing {
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			if g.selRow > 0 {
				g.selRow--
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			if g.selRow < g.canvas.panels[g.activePanel].Rows-1 {
				g.selRow++
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if g.selCol > 0 {
				g.selCol--
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if g.selCol < g.canvas.panels[g.activePanel].Cols-1 {
				g.selCol++
			}
		}
	}

	// let UI handle editing input, caret and commit/cancel
	g.ui.Update(g)

	// quick panel switching
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		g.activePanel = (g.activePanel + 1) % len(g.canvas.panels)
		g.selRow = 0
		g.selCol = 0
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// dark background
	screen.Fill(color.RGBA{0x12, 0x12, 0x14, 0xff})

	// draw canvas (panels)
	g.canvas.Draw(screen, g)

	// draw UI (HUD, editing overlays)
	g.ui.Draw(screen, g)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	// Return the outside dimensions so the logical screen matches window size.
	// This prevents black bars when the window is resized.
	return outsideWidth, outsideHeight
}

func main() {
	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("CellCanvas - Spreadsheet Panels")
	ebiten.SetWindowResizable(true)
	g := NewGame()
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

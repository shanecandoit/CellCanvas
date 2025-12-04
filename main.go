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

func (g *Game) Update() error {
	// start/stop dragging with right mouse
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.dragging = true
		g.lastMouseX, g.lastMouseY = ebiten.CursorPosition()
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonRight) {
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
	ebiten.SetWindowTitle("CellChain — Canvas Panels Demo")
	ebiten.SetWindowResizable(true)
	g := NewGame()
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

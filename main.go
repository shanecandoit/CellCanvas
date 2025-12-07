package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
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

	input       *InputManager
	contextMenu *ContextMenu

	// selection
	activePanel    int
	selRow, selCol int

	// editing
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

	// context menu (now managed by ContextMenu)
}

func NewGame() *Game {
	g := &Game{}
	g.canvas = NewCanvas()
	g.ui = NewUI()
	g.input = NewInputManager()
	g.contextMenu = NewContextMenu()
	g.activePanel = 0
	g.selRow = 0
	g.selCol = 0
	g.editingPanelName = false
	g.editPanelIndex = -1
	// Attempt to load initial layout from `state.yml` non-blocking.
	// LoadState schedules any CSV loads in the background.
	if err := g.canvas.LoadState("state.yml"); err != nil {
		log.Printf("LoadState: %v", err)
	}
	return g
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func (g *Game) Update() error {
	// input handling
	g.input.HandlePanInput(g)

	// delegate panel mouse interactions to canvas (it will update selection on Game)
	g.canvas.Update(g)

	g.input.HandleContextMenuInput(g)

	g.input.HandleSelectionNavigation(g)

	// let UI handle editing input, caret and commit/cancel
	g.ui.Update(g)

	g.input.HandlePanelSwitching(g)

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// dark background
	screen.Fill(ColorBackground)

	// draw canvas (panels)
	state := CanvasDrawState{
		Face:             g.ui.face,
		ActivePanel:      g.activePanel,
		SelRow:           g.selRow,
		SelCol:           g.selCol,
		Editing:          g.editing,
		EditBuffer:       g.editBuffer,
		EditingPanelName: g.editingPanelName,
		EditPanelIndex:   g.editPanelIndex,
		EditPanelBuffer:  g.editPanelBuffer,
		CaretVisible:     g.caretVisible,
		EditCursor:       g.editCursor,
		EditPanelCursor:  g.editPanelCursor,
	}
	g.canvas.Draw(screen, state)

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

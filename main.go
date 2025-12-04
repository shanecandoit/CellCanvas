package main

import (
	"fmt"
	"image/color"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
)

const (
	windowWidth  = 1280
	windowHeight = 720
	defaultCellW = 80
	defaultCellH = 24
)

type Panel struct {
	X, Y  int
	Cols  int
	Rows  int
	CellW int
	CellH int
	Cells [][]string
}

type Game struct {
	// canvas contains the panels and camera state
	canvas *Canvas

	// UI renderer
	ui *UI

	// dragging (right-button pan)
	dragging   bool
	lastMouseX int
	lastMouseY int

	// selection
	activePanel    int
	selRow, selCol int

	// editing
	editing    bool
	editBuffer string
}

// Canvas contains panels and camera + interaction state for moving/resizing
type Canvas struct {
	panels     []Panel
	camX, camY float64

	movingPanel   int
	resizingPanel int
	moveOffsetX   int
	moveOffsetY   int
}

// UI handles HUD and overlay drawing
type UI struct {
	face font.Face
}

func NewCanvas() *Canvas {
	c := &Canvas{movingPanel: -1, resizingPanel: -1}
	c.panels = append(c.panels, NewPanel(20, 20, 8, 16))
	c.panels = append(c.panels, NewPanel(520, 20, 6, 12))
	return c
}

func NewUI() *UI {
	ui := &UI{}

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
	mx, my := ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// check panels from top (last) to bottom (first)
		picked := -1
		for i := len(c.panels) - 1; i >= 0; i-- {
			p := c.panels[i]
			baseX := int(float64(p.X) + c.camX)
			baseY := int(float64(p.Y) + c.camY)
			w := p.Cols * p.CellW
			h := p.Rows * p.CellH
			// header area (title bar) - 18px high above panel
			headerY := baseY - 20
			if mx >= baseX && mx <= baseX+w && my >= headerY && my <= headerY+20 {
				picked = i
				// start moving
				c.movingPanel = i
				c.moveOffsetX = mx - baseX
				c.moveOffsetY = my - baseY
				break
			}
			// resize corner (bottom-right 16x16)
			if mx >= baseX+w-16 && mx <= baseX+w && my >= baseY+h-16 && my <= baseY+h {
				picked = i
				c.resizingPanel = i
				c.moveOffsetX = mx - (baseX + w)
				c.moveOffsetY = my - (baseY + h)
				break
			}
			// click inside panel -> select panel and a cell
			if mx >= baseX && mx <= baseX+w && my >= baseY && my <= baseY+h {
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
		baseX := int(float64(c.panels[i].X) + c.camX)
		baseY := int(float64(c.panels[i].Y) + c.camY)
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
		newCells := make([][]string, rows)
		for r := 0; r < rows; r++ {
			newCells[r] = make([]string, cols)
			for ccol := 0; ccol < cols; ccol++ {
				if r < old.Rows && ccol < old.Cols {
					newCells[r][ccol] = old.Cells[r][ccol]
				} else {
					newCells[r][ccol] = ""
				}
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
}

// Draw renders the panels; selection overlay is drawn here but editing text is handled by UI
func (c *Canvas) Draw(screen *ebiten.Image, g *Game) {
	for pi, p := range c.panels {
		baseX := float64(p.X) + c.camX
		baseY := float64(p.Y) + c.camY

		// panel background
		ebitenutil.DrawRect(screen, baseX-4, baseY-20, float64(p.Cols*p.CellW)+8, float64(p.Rows*p.CellH)+28, color.RGBA{0x22, 0x22, 0x2a, 0xff})

		// panel title
		drawTextAt(screen, g.ui.face, fmt.Sprintf("Panel %d", pi), int(baseX)+6, int(baseY-18), color.White)

		// draw border
		ebitenutil.DrawRect(screen, baseX-4, baseY-20, 2, float64(p.Rows*p.CellH)+28, color.RGBA{0x44, 0x44, 0x50, 0xff})
		ebitenutil.DrawRect(screen, baseX-4, baseY-20, float64(p.Cols*p.CellW)+8, 2, color.RGBA{0x44, 0x44, 0x50, 0xff})
		ebitenutil.DrawRect(screen, baseX+float64(p.Cols*p.CellW)+4, baseY-20, 2, float64(p.Rows*p.CellH)+28, color.RGBA{0x44, 0x44, 0x50, 0xff})
		ebitenutil.DrawRect(screen, baseX-4, baseY+float64(p.Rows*p.CellH)+8, float64(p.Cols*p.CellW)+8, 2, color.RGBA{0x44, 0x44, 0x50, 0xff})

		for r := 0; r < p.Rows; r++ {
			for ccol := 0; ccol < p.Cols; ccol++ {
				x := baseX + float64(ccol*p.CellW)
				y := baseY + float64(r*p.CellH)
				// cell bg (slightly lighter card)
				ebitenutil.DrawRect(screen, x, y, float64(p.CellW-1), float64(p.CellH-1), color.RGBA{0x18, 0x18, 0x1c, 0xff})
				// cell text (light)
				txt := p.Cells[r][ccol]
				drawTextAt(screen, g.ui.face, txt, int(x)+6, int(y)+6, color.White)
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
		rx := baseX + float64(p.Cols*p.CellW) - 12
		ry := baseY + float64(p.Rows*p.CellH) - 12
		ebitenutil.DrawRect(screen, rx, ry, 12, 12, color.RGBA{0x55, 0x55, 0x66, 0xff})
	}
}

// Draw renders HUD and editing text overlay
func (ui *UI) Draw(screen *ebiten.Image, g *Game) {
	// Use the actual logical screen height so the HUD sits at the bottom
	// even when the window is resized.
	screenH := screen.Bounds().Dy()
	drawTextAt(screen, ui.face, "Right-drag to pan • Left-drag title to move • Drag corner to resize • Arrows to move • Enter to edit • Tab switch panel", 8, screenH-28, color.White)

	if g.editing {
		// position editing text over the selected cell
		if g.activePanel >= 0 && g.activePanel < len(g.canvas.panels) {
			p := g.canvas.panels[g.activePanel]
			sx := float64(p.X) + g.canvas.camX + float64(g.selCol*p.CellW)
			sy := float64(p.Y) + g.canvas.camY + float64(g.selRow*p.CellH)
			drawTextAt(screen, ui.face, g.editBuffer, int(sx)+6, int(sy)+6, color.White)
		}
	}
}

func NewPanel(x, y, cols, rows int) Panel {
	cells := make([][]string, rows)
	for r := range cells {
		cells[r] = make([]string, cols)
		for c := 0; c < cols; c++ {
			cells[r][c] = fmt.Sprintf("R%dC%d", r, c)
		}
	}
	return Panel{X: x, Y: y, Cols: cols, Rows: rows, CellW: defaultCellW, CellH: defaultCellH, Cells: cells}
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

	// release move/resize when mouse released is handled in canvas.Update

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
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.editing = true
			g.editBuffer = g.canvas.panels[g.activePanel].Cells[g.selRow][g.selCol]
		}
	} else {
		// collecting input characters while editing
		for _, r := range ebiten.InputChars() {
			if r == '\b' {
				if len(g.editBuffer) > 0 {
					g.editBuffer = g.editBuffer[:len(g.editBuffer)-1]
				}
			} else {
				g.editBuffer += string(r)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.canvas.panels[g.activePanel].Cells[g.selRow][g.selCol] = g.editBuffer
			g.editing = false
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.editing = false
		}
	}

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

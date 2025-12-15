package main

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

// Renderer handles all drawing operations for the application.
type Renderer struct{}

// NewRenderer creates a new Renderer instance.
func NewRenderer() *Renderer {
	return &Renderer{}
}

// DrawCanvas renders the entire canvas including all panels.
func (r *Renderer) DrawCanvas(screen *ebiten.Image, c *Canvas, im *InputManager) {
	for pi := range c.panels {
		r.drawPanel(screen, c, &c.panels[pi], pi, im)
	}
}

func (r *Renderer) drawPanel(screen *ebiten.Image, c *Canvas, p *Panel, pi int, im *InputManager) {
	b := p.GetBounds(c.camX, c.camY)

	r.drawPanelBackground(screen, b)
	r.drawPanelHeader(screen, p, b, pi)
	r.drawPanelBorder(screen, b)

	if !p.Loaded {
		r.drawPanelLoading(screen, b)
	} else {
		r.drawPanelContent(screen, p, b, pi, im)
	}

	// Selection and editing are now handled by InputManager.Draw()
	r.drawResizeHandle(screen, p, b)
}

func (r *Renderer) drawPanelBackground(screen *ebiten.Image, b PanelBounds) {
	ebitenutil.DrawRect(screen, float64(b.TotalX), float64(b.TotalY), float64(b.TotalW), float64(b.TotalH), ColorPanelBg)
}

func (r *Renderer) drawPanelHeader(screen *ebiten.Image, p *Panel, b PanelBounds, pi int) {
	baseX := float64(b.ContentX)
	baseY := float64(b.ContentY)

	// panel title
	drawTextAt(screen, nil, fmt.Sprintf("Panel %d", pi+1), int(baseX)+PanelInnerPadding, int(baseY-PanelHeaderHeight+2), ColorText)

	// draw a blank clickable name button centered in the header
	btnX := baseX + float64(b.ContentW)/2 - float64(PanelNameButtonW)/2
	btnY := float64(baseY) - float64(PanelHeaderHeight) + float64((PanelHeaderHeight-PanelNameButtonH)/2)
	ebitenutil.DrawRect(screen, btnX, btnY, float64(PanelNameButtonW), float64(PanelNameButtonH), ColorPanelHeaderBtn)

	// draw panel's alias/name inside the header button
	nameToShow := p.Name

	// Draw text if present
	tx := btnX + float64(PanelNameButtonW)/2 // default center position
	if nameToShow != "" {
		drawTextAt(screen, nil, nameToShow, int(tx), int(btnY), ColorText)
	}

	// Panel name editing is now handled by InputManager.Draw()
}

func (r *Renderer) drawPanelBorder(screen *ebiten.Image, b PanelBounds) {
	ebitenutil.DrawRect(screen, float64(b.TotalX), float64(b.TotalY), float64(PanelBorderWidth), float64(b.TotalH), ColorPanelBorder)
	ebitenutil.DrawRect(screen, float64(b.TotalX), float64(b.TotalY), float64(b.TotalW), float64(PanelBorderWidth), ColorPanelBorder)
	ebitenutil.DrawRect(screen, float64(b.TotalX+b.TotalW-PanelBorderWidth), float64(b.TotalY), float64(PanelBorderWidth), float64(b.TotalH), ColorPanelBorder)
	ebitenutil.DrawRect(screen, float64(b.TotalX), float64(b.TotalY+b.TotalH-PanelBorderWidth), float64(b.TotalW), float64(PanelBorderWidth), ColorPanelBorder)
}

func (r *Renderer) drawPanelLoading(screen *ebiten.Image, b PanelBounds) {
	ebitenutil.DrawRect(screen, float64(b.ContentX), float64(b.ContentY), float64(b.ContentW), float64(b.ContentH), ColorPanelLoading)
	drawTextAt(screen, nil, "Loading...", b.ContentX+PanelInnerPadding, b.ContentY+PanelInnerPadding, ColorText)
}

func (r *Renderer) drawPanelContent(screen *ebiten.Image, p *Panel, b PanelBounds, pi int, im *InputManager) {
	baseX := float64(b.ContentX)
	baseY := float64(b.ContentY)
	for row := 0; row < p.Rows; row++ {
		for col := 0; col < p.Cols; col++ {
			x := baseX + float64(col*p.CellW)
			y := baseY + float64(row*p.CellH)
			// cell bg
			ebitenutil.DrawRect(screen, x, y, float64(p.CellW-1), float64(p.CellH-1), ColorCellBg)

			// If this cell is being edited, skip drawing its static content so we don't get double-draw
			if im.editing && !im.editingPanelName && im.activePanel == pi && im.selRow == row && im.selCol == col {
				continue
			}

			// cell text
			key := CellRef(col, row)
			txt := ""
			if v, ok := p.Cells[key]; ok {
				txt = v
			}
			// Editing text is now handled by InputManager.Draw()
			drawTextAt(screen, nil, txt, int(x)+PanelInnerPadding, int(y)+PanelInnerPadding, ColorText)
		}
	}
}

func (r *Renderer) drawPanelSelection(screen *ebiten.Image, p *Panel, b PanelBounds, pi int, state CanvasDrawState) {
	if pi == state.ActivePanel {
		baseX := float64(b.ContentX)
		baseY := float64(b.ContentY)
		sx := baseX + float64(state.SelCol*p.CellW)
		sy := baseY + float64(state.SelRow*p.CellH)
		cellW := float64(p.CellW - 1)
		cellH := float64(p.CellH - 1)
		borderWidth := 2.0

		// Draw blue border instead of filled rectangle
		// Top border
		ebitenutil.DrawRect(screen, sx, sy, cellW, borderWidth, ColorSelection)
		// Bottom border
		ebitenutil.DrawRect(screen, sx, sy+cellH-borderWidth, cellW, borderWidth, ColorSelection)
		// Left border
		ebitenutil.DrawRect(screen, sx, sy, borderWidth, cellH, ColorSelection)
		// Right border
		ebitenutil.DrawRect(screen, sx+cellW-borderWidth, sy, borderWidth, cellH, ColorSelection)
	}
}

func (r *Renderer) drawResizeHandle(screen *ebiten.Image, p *Panel, b PanelBounds) {
	baseX := float64(b.ContentX)
	baseY := float64(b.ContentY)
	rx := baseX + float64(p.Cols*p.CellW) - ResizeHandleSize
	ry := baseY + float64(p.Rows*p.CellH) - ResizeHandleSize
	ebitenutil.DrawRect(screen, rx, ry, float64(ResizeHandleSize), float64(ResizeHandleSize), ColorResizeHandle)
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

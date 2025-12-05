package main

// Layout constants for panel drawing and hit testing
const (
	PanelPaddingX     = 4
	PanelPaddingY     = 4
	PanelHeaderHeight = 20
	PanelBorderWidth  = 2
	ResizeHandleSize  = 12
	PanelInnerPadding = 6
)

// PanelBounds consolidates the geometric layout of a Panel as rendered
// on the screen (given camera offsets).
type PanelBounds struct {
	ContentX, ContentY int
	ContentW, ContentH int
	TotalX, TotalY     int
	TotalW, TotalH     int
}

// GetBounds returns the on-screen bounding rectangles for content and total area of a panel.
func (p *Panel) GetBounds(camX, camY float64) PanelBounds {
	contentX := int(float64(p.X) + camX)
	contentY := int(float64(p.Y) + camY)
	contentW := p.Cols * p.CellW
	contentH := p.Rows * p.CellH
	totalX := contentX - PanelPaddingX
	totalY := contentY - PanelHeaderHeight
	totalW := contentW + PanelPaddingX*2
	totalH := contentH + PanelHeaderHeight + PanelPaddingY*2
	return PanelBounds{
		ContentX: contentX,
		ContentY: contentY,
		ContentW: contentW,
		ContentH: contentH,
		TotalX:   totalX,
		TotalY:   totalY,
		TotalW:   totalW,
		TotalH:   totalH,
	}
}

func (p *Panel) GetScreenRect(camX, camY float64) (x, y, w, h int) {
	b := p.GetBounds(camX, camY)
	return b.TotalX, b.TotalY, b.TotalW, b.TotalH
}

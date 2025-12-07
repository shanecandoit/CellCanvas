package main

import "image/color"

// Color Palette
var (
	ColorBackground     = color.RGBA{0x12, 0x12, 0x14, 0xff} // Main window background
	ColorPanelBg        = color.RGBA{0x22, 0x22, 0x2a, 0xff} // Panel body background
	ColorPanelHeaderBtn = color.RGBA{0x11, 0x11, 0x16, 0xff} // Panel header button background
	ColorPanelBorder    = color.RGBA{0x44, 0x44, 0x50, 0xff} // Panel border
	ColorPanelLoading   = color.RGBA{0x0f, 0x0f, 0x12, 0xff} // Loading placeholder background
	ColorCellBg         = color.RGBA{0x18, 0x18, 0x1c, 0xff} // Cell background
	ColorSelection      = color.RGBA{0x66, 0x88, 0xff, 0x66} // Selection overlay (transparent)
	ColorResizeHandle   = color.RGBA{0x55, 0x55, 0x66, 0xff} // Resize handle
	ColorText           = color.White                        // Standard text
	ColorTextDim        = color.RGBA{0xdd, 0xdd, 0xdd, 0xff} // Dimmed text (logs)
	ColorOverlayBg      = color.RGBA{0x11, 0x11, 0x16, 0xff} // Top overlay background
	ColorLogBg          = color.RGBA{0x0c, 0x0c, 0x0e, 0xee} // Click log background
	ColorMenuBg         = color.RGBA{0x10, 0x10, 0x12, 0xff} // Context menu background
	ColorMenuBorder     = color.RGBA{0x44, 0x44, 0x50, 0xff} // Context menu border
	ColorMenuHighlight  = color.RGBA{0x33, 0x55, 0xff, 0xff} // Context menu hover highlight
)

// Layout Constants
const (
	PanelPaddingX     = 4
	PanelPaddingY     = 4
	PanelHeaderHeight = 20
	PanelBorderWidth  = 2
	ResizeHandleSize  = 12
	PanelInnerPadding = 6

	// UI Layout
	PanelNameButtonW = 96
	PanelNameButtonH = 14
)

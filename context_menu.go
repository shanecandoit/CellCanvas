package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"golang.org/x/image/font"
)

// MenuAction describes what action was selected in the context menu
type MenuAction int

const (
	MenuActionNone MenuAction = iota
	MenuActionNewBlankPanel
	MenuActionLoadPanelFromFile
)

// ContextMenu encapsulates the state and behavior of a right-click context menu
// It provides methods to show/hide, update based on input, and draw itself.
type ContextMenu struct {
	visible  bool
	x, y     int
	items    []string
	selected int
}

func NewContextMenu() *ContextMenu {
	return &ContextMenu{
		visible:  false,
		items:    []string{"New Blank Panel", "Load Panel from File ..."},
		selected: -1,
	}
}

func (cm *ContextMenu) Show(x, y int) {
	cm.visible = true
	cm.x = x
	cm.y = y
	cm.selected = -1
}

func (cm *ContextMenu) Hide() {
	cm.visible = false
	cm.selected = -1
}

// Update returns a MenuAction for any selection triggered, and may hide the menu
// as part of its behavior.
func (cm *ContextMenu) Update(g *Game) MenuAction {
	if !cm.visible {
		return MenuActionNone
	}

	mx, my := ebiten.CursorPosition()
	itemH := 28
	w := 240
	x := cm.x
	y := cm.y

	// determine hover index
	if mx >= x && mx <= x+w && my >= y && my <= y+itemH*len(cm.items) {
		idx := (my - y) / itemH
		if idx < 0 {
			idx = -1
		}
		cm.selected = idx
	} else {
		cm.selected = -1
	}

	// left click selects or closes
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if cm.selected >= 0 {
			switch cm.selected {
			case 0:
				// New Blank Panel at world coords
				cm.visible = false
				return MenuActionNewBlankPanel
			case 1:
				cm.visible = false
				return MenuActionLoadPanelFromFile
			}
		} else {
			cm.visible = false
		}
	}

	// close menu on Escape
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		cm.visible = false
	}
	// if right-click again, close
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		cm.visible = false
	}

	return MenuActionNone
}

func (cm *ContextMenu) Draw(screen *ebiten.Image, face font.Face) {
	if !cm.visible {
		return
	}
	itemH := 28
	w := 240
	x := cm.x
	y := cm.y
	// background with small padding
	bgX := float64(x - 4)
	bgY := float64(y - 4)
	bgW := float64(w + 8)
	bgH := float64(itemH*len(cm.items) + 8)
	ebitenutil.DrawRect(screen, bgX, bgY, bgW, bgH, color.RGBA{0x10, 0x10, 0x12, 0xff})
	// border
	ebitenutil.DrawRect(screen, bgX, bgY, bgW, 2, color.RGBA{0x44, 0x44, 0x50, 0xff})
	ebitenutil.DrawRect(screen, bgX, bgY+bgH-2, bgW, 2, color.RGBA{0x44, 0x44, 0x50, 0xff})
	ebitenutil.DrawRect(screen, bgX, bgY, 2, bgH, color.RGBA{0x44, 0x44, 0x50, 0xff})
	ebitenutil.DrawRect(screen, bgX+bgW-2, bgY, 2, bgH, color.RGBA{0x44, 0x44, 0x50, 0xff})

	for i, it := range cm.items {
		iy := y + i*itemH
		// highlight on hover
		if cm.selected == i {
			ebitenutil.DrawRect(screen, float64(x), float64(iy), float64(w), float64(itemH), color.RGBA{0x33, 0x55, 0xff, 0xff})
			// draw text in white
			drawTextAt(screen, face, it, x+8, iy+6, color.White)
		} else {
			// normal background (transparent) and text
			drawTextAt(screen, face, it, x+8, iy+6, color.White)
		}
	}
}

# CellChain

CellChain is a simple, spreadsheet-like desktop application written in Go using Ebiten. It presents a large canvas composed of movable panels; each panel contains a grid of cells you can navigate and edit. It's designed as a lightweight, interactive demo showcasing how to build UI-like layouts and editable grids.

**Status:** Minimal prototype — focused on core navigation and editing on a large canvas.

**Built with:** Go, Ebiten (`github.com/hajimehoshi/ebiten/v2`)

**Key ideas:**
- **Big canvas:** A single large drawing surface that can be panned and zoomed.
- **Panels:** Logical regions on the canvas; each panel holds a grid of cells.
- **Cells:** Editable text cells; basic navigation and selection supported.

**Features**
- Basic cell selection and navigation using keyboard and mouse.
- Enter-to-edit and confirm/cancel editing.
- Panning the canvas to view different panels.
- Simple cell rendering with row/column headers on each panel.

## Usage / Controls

- **Mouse left-click:** select a cell.
- **Arrow keys:** move active cell.
- **Enter:** start editing the active cell.
- **Esc:** cancel editing.
- **Type when editing:** input cell text, Enter to commit.
- **Mouse drag (or hold Space + drag):** pan the canvas to reveal other panels.

## Developer notes

- The UI is rendered on a single Ebiten window; panels are drawn as rectangular regions with their own row/column offsets.
- Keep cell data in a lightweight in-memory structure (map or slice); serialization for save/load can be added later (JSON, CSV, or custom format).

## Project layout (recommended)
- `main.go` — application entry, Ebiten game loop setup.
- `ui/` — UI helpers, panel and cell rendering code.
- `data/` — cell data structures and persistence helpers.
- `assets/` — any images or fonts used by the UI.

## Extending the app

- Add copy/paste and range selection.
- Add basic formulas or computed cells.
- Add save/load (CSV/JSON) and export features.
- Improve performance for very large canvases (virtualized rendering).

## License

AGPL

Enjoy exploring canvas-driven UIs with Ebiten!

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// statePanel is a compact pointer to panel data stored in YAML state.
type statePanel struct {
	X        int    `yaml:"x"`
	Y        int    `yaml:"y"`
	Filename string `yaml:"file"`
	Name     string `yaml:"name,omitempty"`
}

type stateFile struct {
	CamX   float64      `yaml:"cam_x"`
	CamY   float64      `yaml:"cam_y"`
	Panels []statePanel `yaml:"panels"`
}

// loadResult is used to pass loaded CSV data back into the main loop.
// It is defined in model.go so CSV I/O and background scheduling live in the
// same file.
type loadResult struct {
	idx      int
	p        Panel
	err      error
	filename string
}

// SaveManager coordinates background CSV loads and applies them safely
// on the main thread. It owns the internal channel and does not expose it
// to canvas.go so we keep canvas free of concurrency primitives.
type SaveManager struct {
	loadCh chan loadResult
}

// NewSaveManager creates and initializes a SaveManager.
func NewSaveManager() *SaveManager {
	return &SaveManager{loadCh: make(chan loadResult, 8)}
}

// ScheduleLoad starts a background load of CSV file and will send the
// resulting loadResult on the manager's channel when done.
func (sm *SaveManager) ScheduleLoad(idx int, path string) {
	// spawn a goroutine to do IO and parsing
	go func(i int, pth string) {
		tmp := NewBlankPanel(0, 0, 1, 1)
		err := loadPanelCSV(pth, &tmp)
		sm.loadCh <- loadResult{idx: i, p: tmp, err: err, filename: filepath.Base(pth)}
	}(idx, path)
}

// ApplyPending consumes any completed loads and applies them into the
// provided canvas. It will also optionally log failures via the game's UI.
func (sm *SaveManager) ApplyPending(c *Canvas, g *Game) {
	if sm == nil || sm.loadCh == nil {
		return
	}
	for {
		select {
		case r := <-sm.loadCh:
			if r.err == nil {
				if r.idx >= 0 && r.idx < len(c.panels) {
					// preserve existing X/Y, and copy loaded content
					existing := c.panels[r.idx]
					r.p.X = existing.X
					r.p.Y = existing.Y
					r.p.Filename = r.filename
					r.p.Loaded = true
					c.panels[r.idx] = r.p
				}
			} else {
				if r.idx >= 0 && r.idx < len(c.panels) {
					c.panels[r.idx].Filename = r.filename
					// keep panel as not loaded (placeholder)
					c.panels[r.idx].Loaded = false
				}
				if g != nil && g.ui != nil {
					g.ui.addClickLog(fmt.Sprintf("failed to background load %s: %v", r.filename, r.err))
				}
			}
		default:
			return
		}
	}
}

// SaveState writes a small YAML file describing camera and panel pointers.
// Each panel is saved as a separate CSV file next to the YAML file when the
// panel has no Filename yet (or when force is true). Filenames in the YAML are
// relative to the YAML file.
func (c *Canvas) SaveState(statePath string) error {
	dir := filepath.Dir(statePath)

	sf := stateFile{CamX: c.camX, CamY: c.camY}
	for i := range c.panels {
		p := &c.panels[i]
		// if no filename assigned, create one
		if p.Filename == "" {
			// Use 1-based numbering for generated panel filenames to be more user-friendly
			p.Filename = fmt.Sprintf("panel_%d.csv", i+1)
		}
		// write panel CSV next to state file
		csvPath := p.Filename
		if !filepath.IsAbs(csvPath) {
			csvPath = filepath.Join(dir, csvPath)
		}
		if err := savePanelCSV(csvPath, p); err != nil {
			return err
		}

		sf.Panels = append(sf.Panels, statePanel{X: p.X, Y: p.Y, Filename: p.Filename, Name: p.Name})
	}

	// write YAML
	f, err := os.Create(statePath)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(&sf); err != nil {
		return err
	}
	return nil
}

// LoadState reads YAML state and loads per-panel CSVs referenced by it.
// Missing CSVs are ignored (panel will keep its current contents).
func (c *Canvas) LoadState(statePath string) error {
	b, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	var sf stateFile
	if err := yaml.Unmarshal(b, &sf); err != nil {
		return err
	}

	// ensure we have enough panels
	if len(sf.Panels) > len(c.panels) {
		// append new blank panels to match count
		for i := len(c.panels); i < len(sf.Panels); i++ {
			// New panels created due to state having more panels than current
			// should start empty with a small default size (5x5).
			c.panels = append(c.panels, NewBlankPanel(20+i*32, 20+i*32, 5, 5))
		}
	}

	// load each panel referenced. We do not block while loading CSVs; instead
	// place blank panels at the configured positions and schedule background
	// CSV loads so the UI can appear immediately.
	baseDir := filepath.Dir(statePath)
	for i, sp := range sf.Panels {
		if i >= len(c.panels) {
			break
		}
		p := &c.panels[i]
		p.X = sp.X
		p.Y = sp.Y
		p.Name = sp.Name
		// Make sure the panel is empty/blank until CSV load completes.
		p.Cells = make(map[string]string)
		p.Rows = 5
		p.Cols = 5
		// If the panel references a CSV file, mark it not loaded and
		// schedule loading; if there's no file, the panel is considered
		// ready/active.
		p.Loaded = (sp.Filename == "")
		if sp.Filename != "" {
			csvPath := sp.Filename
			if !filepath.IsAbs(csvPath) {
				csvPath = filepath.Join(baseDir, csvPath)
			}
			p.Filename = sp.Filename
			// schedule background load; safe even if the loader fails
			if c.saveManager != nil {
				c.saveManager.ScheduleLoad(i, csvPath)
			} else {
				// fallback to synchronous load if manager missing
				tmp := NewBlankPanel(0, 0, 1, 1)
				_ = loadPanelCSV(csvPath, &tmp)
				// apply the loaded tmp directly
				tmp.X = p.X
				tmp.Y = p.Y
				tmp.Filename = filepath.Base(csvPath)
				tmp.Loaded = (tmp.Rows > 0 && tmp.Cols > 0) || len(tmp.Cells) > 0
				c.panels[i] = tmp
			}
		}
	}
	c.camX = sf.CamX
	c.camY = sf.CamY
	return nil
}

func savePanelCSV(path string, p *Panel) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	// Determine the last row that contains any non-empty data. We will
	// write rows up to and including that index. This prevents saving
	// trailing empty rows at the bottom of the CSV while preserving
	// intermediate empty rows.
	lastRow := -1
	for r := 0; r < p.Rows; r++ {
		for cidx := 0; cidx < p.Cols; cidx++ {
			if p.GetCell(cidx, r) != "" {
				lastRow = r
				break
			}
		}
	}

	if lastRow == -1 {
		// no data at all; write an empty file
		return nil
	}

	for r := 0; r <= lastRow; r++ {
		row := make([]string, p.Cols)
		for cidx := 0; cidx < p.Cols; cidx++ {
			row[cidx] = p.GetCell(cidx, r)
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func loadPanelCSV(path string, p *Panel) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return err
	}
	if len(records) == 0 {
		// empty file -> zero-sized panel
		p.Rows = 0
		p.Cols = 0
		p.Cells = map[string]string{}
		return nil
	}
	cols := 0
	for _, rec := range records {
		if len(rec) > cols {
			cols = len(rec)
		}
	}
	rows := len(records)
	cells := make(map[string]string)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			v := ""
			if j < len(records[i]) {
				v = records[i][j]
			}
			if v != "" {
				cells[CellRef(j, i)] = v
			}
		}
	}
	p.Rows = rows
	p.Cols = cols
	p.Cells = cells
	return nil
}

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
}

type stateFile struct {
	CamX   float64      `yaml:"cam_x"`
	CamY   float64      `yaml:"cam_y"`
	Panels []statePanel `yaml:"panels"`
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

		sf.Panels = append(sf.Panels, statePanel{X: p.X, Y: p.Y, Filename: p.Filename})
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

	// load each panel referenced
	baseDir := filepath.Dir(statePath)
	for i, sp := range sf.Panels {
		if i >= len(c.panels) {
			break
		}
		p := &c.panels[i]
		p.X = sp.X
		p.Y = sp.Y
		if sp.Filename != "" {
			csvPath := sp.Filename
			if !filepath.IsAbs(csvPath) {
				csvPath = filepath.Join(baseDir, csvPath)
			}
			if err := loadPanelCSV(csvPath, p); err != nil {
				// non-fatal: continue but record filename
				p.Filename = sp.Filename
				continue
			}
			// store the relative filename in panel
			p.Filename = sp.Filename
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
	for r := 0; r < p.Rows; r++ {
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

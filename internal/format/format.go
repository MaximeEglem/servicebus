package format

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type OutputFormat string

const (
	Table OutputFormat = "table"
	JSON  OutputFormat = "json"
	YAML  OutputFormat = "yaml"
)

type ThresholdConfig struct {
	SizePercentage int
	LastAccessDays int
}

type Printer struct {
	format     OutputFormat
	out        io.Writer
	thresholds ThresholdConfig
}

func NewPrinter(f OutputFormat, out io.Writer, t ThresholdConfig) *Printer {
	return &Printer{format: f, out: out, thresholds: t}
}

func ParseFormat(s string) (OutputFormat, error) {
	switch strings.ToLower(s) {
	case "table", "":
		return Table, nil
	case "json":
		return JSON, nil
	case "yaml":
		return YAML, nil
	default:
		return "", fmt.Errorf("unknown output format %q: use table, json, or yaml", s)
	}
}

func (p *Printer) Print(data any) error {
	switch p.format {
	case JSON:
		enc := json.NewEncoder(p.out)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case YAML:
		enc := yaml.NewEncoder(p.out)
		if err := enc.Encode(data); err != nil {
			return err
		}
		return enc.Close()
	default:
		return p.printTable(data)
	}
}

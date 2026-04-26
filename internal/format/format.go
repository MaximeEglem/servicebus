package format

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
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
		return yaml.NewEncoder(p.out).Encode(data)
	default:
		return p.printTable(data)
	}
}

func (p *Printer) printTable(data any) error {
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Slice || v.Len() == 0 {
		return nil
	}

	elemType := v.Type().Elem()
	if elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}

	// Collect exported field names as headers
	var headers []string
	for i := 0; i < elemType.NumField(); i++ {
		f := elemType.Field(i)
		if f.IsExported() {
			headers = append(headers, f.Name)
		}
	}

	tbl := tablewriter.NewTable(p.out,
		tablewriter.WithAlignment(tw.MakeAlign(len(headers), tw.AlignLeft)),
		tablewriter.WithRendition(tw.Rendition{
			Borders: tw.Border{
				Left:   tw.Off,
				Right:  tw.Off,
				Top:    tw.Off,
				Bottom: tw.Off,
			},
		}),
	)

	headerIface := make([]any, len(headers))
	for i, h := range headers {
		headerIface[i] = h
	}
	tbl.Header(headerIface...)

	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		row := make([]any, len(headers))
		for j := range headers {
			row[j] = fmt.Sprintf("%v", elem.Field(j).Interface())
		}
		if err := tbl.Append(row...); err != nil {
			return err
		}
	}
	return tbl.Render()
}

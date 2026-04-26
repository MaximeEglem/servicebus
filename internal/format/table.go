package format

import (
	"fmt"
	"reflect"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// Highlighter is implemented by row types that want ANSI threshold coloring.
type Highlighter interface {
	HighlightColor(t ThresholdConfig) string // returns "red", "yellow", or ""
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

		// Determine color sprint function if the element implements Highlighter
		var sprint func(a ...interface{}) string
		if hl, ok := elem.Addr().Interface().(Highlighter); ok {
			switch hl.HighlightColor(p.thresholds) {
			case "red":
				sprint = color.New(color.FgRed).SprintFunc()
			case "yellow":
				sprint = color.New(color.FgYellow).SprintFunc()
			}
		}

		row := make([]any, len(headers))
		for j := range headers {
			cell := fmt.Sprintf("%v", elem.Field(j).Interface())
			if sprint != nil {
				cell = sprint(cell)
			}
			row[j] = cell
		}
		if err := tbl.Append(row...); err != nil {
			return err
		}
	}
	return tbl.Render()
}

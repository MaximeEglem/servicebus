package format

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// Highlighter is implemented by row types that want ANSI threshold coloring.
type Highlighter interface {
	HighlightColor(t ThresholdConfig) string // returns "red", "yellow", or ""
}

// TableColumner is implemented by row types that want to restrict which
// fields are visible in table output. Return field names as they appear
// in the struct (e.g. "Name", "ActiveMessages"). When not implemented,
// all exported fields are shown.
type TableColumner interface {
	TableColumns() []string
}

type fieldMeta struct {
	name  string
	index int
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

	// Collect exported fields with their struct indices
	var allFields []fieldMeta
	for i := 0; i < elemType.NumField(); i++ {
		f := elemType.Field(i)
		if f.IsExported() {
			allFields = append(allFields, fieldMeta{f.Name, i})
		}
	}

	// If the element type implements TableColumner, restrict to declared columns.
	var fields []fieldMeta
	if v.Len() > 0 {
		elem0 := v.Index(0)
		if elem0.Kind() == reflect.Ptr {
			elem0 = elem0.Elem()
		}
		if tc, ok := elem0.Addr().Interface().(TableColumner); ok {
			visible := make(map[string]int)
			for idx, name := range tc.TableColumns() {
				visible[name] = idx
			}
			ordered := make([]fieldMeta, len(tc.TableColumns()))
			for _, fm := range allFields {
				if idx, ok := visible[fm.name]; ok {
					ordered[idx] = fm
				}
			}
			for _, fm := range ordered {
				if fm.name != "" {
					fields = append(fields, fm)
				}
			}
		}
	}
	if len(fields) == 0 {
		fields = allFields
	}

	tbl := tablewriter.NewTable(p.out,
		tablewriter.WithAlignment(tw.MakeAlign(len(fields), tw.AlignLeft)),
		tablewriter.WithRendition(tw.Rendition{
			Borders: tw.Border{
				Left:   tw.On,
				Right:  tw.On,
				Top:    tw.On,
				Bottom: tw.On,
			},
			Settings: tw.Settings{
				Separators: tw.Separators{
					BetweenColumns: tw.On,
					BetweenRows:    tw.On,
				},
			},
		}),
	)

	tty := isTTYWriter(p.out)
	showBars := isBarsEnabled()
	var numericMax map[int]float64
	if showBars {
		numericMax = computeNumericMaxByField(v, fields)
	}
	if tty {
		accent := color.New(color.FgHiCyan, color.Bold).SprintFunc()
		fmt.Fprintf(p.out, "%s (%d)\n\n", accent(humanizeTypeName(elemType.Name())), v.Len())
	} else {
		fmt.Fprintf(p.out, "%s (%d)\n\n", humanizeTypeName(elemType.Name()), v.Len())
	}

	headerIface := make([]any, len(fields))
	for j, fm := range fields {
		headerIface[j] = humanizeHeader(fm.name)
	}
	_ = tty // tty used for title accent only
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

		row := make([]any, len(fields))
		for j, fm := range fields {
			fv := elem.Field(fm.index)
			cell := formatTableCell(fv.Interface())
			if showBars {
				if max, ok := numericMax[fm.index]; ok && max > 0 {
					if num, ok := numericValueFromReflect(fv); ok {
						cell = formatCellWithBar(cell, num, max)
					}
				}
			}
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

func isTTYWriter(out any) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return (st.Mode() & os.ModeCharDevice) != 0
}

func humanizeTypeName(name string) string {
	if name == "" {
		return "Results"
	}
	return humanizeHeader(name)
}

func humanizeHeader(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	var prev rune
	for i, r := range s {
		if i > 0 {
			if unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
				b.WriteRune(' ')
			}
			if unicode.IsDigit(r) && unicode.IsLetter(prev) {
				b.WriteRune(' ')
			}
		}
		b.WriteRune(r)
		prev = r
	}
	words := strings.Fields(b.String())
	for i, w := range words {
		if len(w) == 0 {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func isBarsEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("SB_TABLE_BARS")))
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return v == "on" || v == "yes" || v == "y" || v == "1"
	}
	return b
}

func computeNumericMaxByField(v reflect.Value, fields []fieldMeta) map[int]float64 {
	maxByField := make(map[int]float64)
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		for _, fm := range fields {
			if num, ok := numericValueFromReflect(elem.Field(fm.index)); ok {
				if num > maxByField[fm.index] {
					maxByField[fm.index] = num
				}
			}
		}
	}
	return maxByField
}

func numericValueFromReflect(v reflect.Value) (float64, bool) {
	if !v.IsValid() {
		return 0, false
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return 0, false
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	default:
		return 0, false
	}
}

func formatCellWithBar(base string, value, max float64) string {
	if max <= 0 || value < 0 {
		return base
	}
	const barWidth = 12
	filled := int((value / max) * barWidth)
	if value > 0 && filled == 0 {
		filled = 1
	}
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("#", filled) + strings.Repeat("-", barWidth-filled)
	return fmt.Sprintf("%s [%s]", base, bar)
}

func formatTableCell(value any) string {
	switch v := value.(type) {
	case time.Time:
		if v.IsZero() {
			return ""
		}
		return v.Format("2006-01-02 15:04:05")
	case *time.Time:
		if v == nil || v.IsZero() {
			return ""
		}
		return v.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprintf("%v", value)
	}
}

package format_test

import (
	"bytes"
	"testing"
	"time"

	"servicebus/internal/format"
)

type row struct {
	Name   string
	Count  int
	Active bool
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	p := format.NewPrinter(format.JSON, &buf, format.ThresholdConfig{})
	data := []row{{Name: "q1", Count: 5, Active: true}}
	if err := p.Print(data); err != nil {
		t.Fatalf("Print: %v", err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte(`"Name": "q1"`)) {
		t.Errorf("JSON output missing Name: q1 pair: %s", out)
	}
}

func TestYAMLOutput(t *testing.T) {
	var buf bytes.Buffer
	p := format.NewPrinter(format.YAML, &buf, format.ThresholdConfig{})
	data := []row{{Name: "q1", Count: 5}}
	if err := p.Print(data); err != nil {
		t.Fatalf("Print: %v", err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("name: q1")) {
		t.Errorf("YAML output missing name: q1: %s", out)
	}
}

func TestTableOutput(t *testing.T) {
	var buf bytes.Buffer
	p := format.NewPrinter(format.Table, &buf, format.ThresholdConfig{})
	data := []row{{Name: "myqueue", Count: 42, Active: true}}
	if err := p.Print(data); err != nil {
		t.Fatalf("Print: %v", err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("myqueue")) {
		t.Errorf("Table output missing myqueue: %s", out)
	}
}

func TestTableOutputFormatsTimeShort(t *testing.T) {
	type rowWithTime struct {
		Name      string
		CreatedAt time.Time
	}

	var buf bytes.Buffer
	p := format.NewPrinter(format.Table, &buf, format.ThresholdConfig{})
	ts := time.Date(2025, 5, 19, 21, 21, 22, 424501200, time.UTC)
	data := []rowWithTime{{Name: "q1", CreatedAt: ts}}

	if err := p.Print(data); err != nil {
		t.Fatalf("Print: %v", err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("2025-05-19 21:21:22")) {
		t.Fatalf("expected short timestamp in table output: %s", out)
	}
	if bytes.Contains(buf.Bytes(), []byte("UTC")) {
		t.Fatalf("expected table output without verbose time zone suffix: %s", out)
	}
}

func TestParseFormat(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  format.OutputFormat
	}{
		{"json", format.JSON},
		{"JSON", format.JSON},
		{"yaml", format.YAML},
		{"table", format.Table},
		{"", format.Table},
	} {
		got, err := format.ParseFormat(tc.input)
		if err != nil {
			t.Errorf("ParseFormat(%q): %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseFormat(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseFormatInvalid(t *testing.T) {
	_, err := format.ParseFormat("xml")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

package executor

import (
	"bytes"
	"sync"
	"testing"
)

func TestLineWriter_SingleLine(t *testing.T) {
	var lines []string
	lw := NewLineWriter(nil, func(line string) {
		lines = append(lines, line)
	})

	_, err := lw.Write([]byte("hello world\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "hello world" {
		t.Errorf("expected 'hello world', got %q", lines[0])
	}
}

func TestLineWriter_MultipleLines(t *testing.T) {
	var lines []string
	lw := NewLineWriter(nil, func(line string) {
		lines = append(lines, line)
	})

	_, err := lw.Write([]byte("line1\nline2\nline3\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	expected := []string{"line1", "line2", "line3"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d", len(expected), len(lines))
	}
	for i, exp := range expected {
		if lines[i] != exp {
			t.Errorf("line %d: expected %q, got %q", i, exp, lines[i])
		}
	}
}

func TestLineWriter_PartialLine(t *testing.T) {
	var lines []string
	lw := NewLineWriter(nil, func(line string) {
		lines = append(lines, line)
	})

	// Write partial line
	_, _ = lw.Write([]byte("hello "))
	if len(lines) != 0 {
		t.Errorf("expected no lines yet, got %d", len(lines))
	}

	// Write more partial
	_, _ = lw.Write([]byte("world"))
	if len(lines) != 0 {
		t.Errorf("expected no lines yet, got %d", len(lines))
	}

	// Complete the line
	_, _ = lw.Write([]byte("!\n"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "hello world!" {
		t.Errorf("expected 'hello world!', got %q", lines[0])
	}
}

func TestLineWriter_Flush(t *testing.T) {
	var lines []string
	lw := NewLineWriter(nil, func(line string) {
		lines = append(lines, line)
	})

	// Write partial line without newline
	_, _ = lw.Write([]byte("partial line"))
	if len(lines) != 0 {
		t.Errorf("expected no lines yet, got %d", len(lines))
	}

	// Flush should send the partial line
	lw.Flush()
	if len(lines) != 1 {
		t.Fatalf("expected 1 line after flush, got %d", len(lines))
	}
	if lines[0] != "partial line" {
		t.Errorf("expected 'partial line', got %q", lines[0])
	}

	// Flush again should not add anything
	lw.Flush()
	if len(lines) != 1 {
		t.Errorf("expected still 1 line after second flush, got %d", len(lines))
	}
}

func TestLineWriter_EmptyLine(t *testing.T) {
	var lines []string
	lw := NewLineWriter(nil, func(line string) {
		lines = append(lines, line)
	})

	_, _ = lw.Write([]byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "" {
		t.Errorf("expected empty string, got %q", lines[0])
	}
}

func TestLineWriter_MultipleEmptyLines(t *testing.T) {
	var lines []string
	lw := NewLineWriter(nil, func(line string) {
		lines = append(lines, line)
	})

	_, _ = lw.Write([]byte("\n\n\n"))
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	for i, l := range lines {
		if l != "" {
			t.Errorf("line %d: expected empty string, got %q", i, l)
		}
	}
}

func TestLineWriter_Downstream(t *testing.T) {
	var downstream bytes.Buffer
	var lines []string

	lw := NewLineWriter(&downstream, func(line string) {
		lines = append(lines, line)
	})

	input := []byte("hello\nworld\n")
	_, err := lw.Write(input)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Check downstream received all bytes
	if downstream.String() != string(input) {
		t.Errorf("downstream: expected %q, got %q", string(input), downstream.String())
	}

	// Check callback received lines
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestLineWriter_DownstreamOnly(t *testing.T) {
	var downstream bytes.Buffer

	// No callback
	lw := NewLineWriter(&downstream, nil)

	input := []byte("hello\nworld\n")
	n, err := lw.Write(input)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(input) {
		t.Errorf("expected to write %d bytes, wrote %d", len(input), n)
	}

	// Check downstream received all bytes
	if downstream.String() != string(input) {
		t.Errorf("downstream: expected %q, got %q", string(input), downstream.String())
	}
}

func TestLineWriter_NoDownstream(t *testing.T) {
	var lines []string
	lw := NewLineWriter(nil, func(line string) {
		lines = append(lines, line)
	})

	input := []byte("hello\n")
	n, err := lw.Write(input)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(input) {
		t.Errorf("expected to write %d bytes, wrote %d", len(input), n)
	}

	if len(lines) != 1 || lines[0] != "hello" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestLineWriter_EmptyWrite(t *testing.T) {
	var lines []string
	lw := NewLineWriter(nil, func(line string) {
		lines = append(lines, line)
	})

	n, err := lw.Write([]byte{})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes written, got %d", n)
	}
	if len(lines) != 0 {
		t.Errorf("expected no lines, got %d", len(lines))
	}
}

func TestLineWriter_Concurrent(t *testing.T) {
	var mu sync.Mutex
	var lines []string

	lw := NewLineWriter(nil, func(line string) {
		mu.Lock()
		lines = append(lines, line)
		mu.Unlock()
	})

	// Write from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, _ = lw.Write([]byte("line\n"))
		}(i)
	}
	wg.Wait()

	mu.Lock()
	count := len(lines)
	mu.Unlock()

	if count != 100 {
		t.Errorf("expected 100 lines, got %d", count)
	}
}

func TestLineWriter_Reset(t *testing.T) {
	var lines []string
	lw := NewLineWriter(nil, func(line string) {
		lines = append(lines, line)
	})

	// Write partial line
	_, _ = lw.Write([]byte("partial"))
	if lw.Buffered() == 0 {
		t.Error("expected buffered data")
	}

	// Reset clears buffer without callback
	lw.Reset()
	if lw.Buffered() != 0 {
		t.Error("expected empty buffer after reset")
	}
	if len(lines) != 0 {
		t.Error("expected no lines after reset")
	}

	// Flush after reset should not add anything
	lw.Flush()
	if len(lines) != 0 {
		t.Error("expected no lines after flush on reset buffer")
	}
}

func TestLineWriter_Buffered(t *testing.T) {
	lw := NewLineWriter(nil, func(line string) {})

	if lw.Buffered() != 0 {
		t.Error("expected 0 buffered initially")
	}

	_, _ = lw.Write([]byte("hello"))
	if lw.Buffered() != 5 {
		t.Errorf("expected 5 buffered, got %d", lw.Buffered())
	}

	_, _ = lw.Write([]byte(" world"))
	if lw.Buffered() != 11 {
		t.Errorf("expected 11 buffered, got %d", lw.Buffered())
	}

	_, _ = lw.Write([]byte("\n"))
	if lw.Buffered() != 0 {
		t.Errorf("expected 0 buffered after newline, got %d", lw.Buffered())
	}
}

func TestLineWriter_BinaryData(t *testing.T) {
	var downstream bytes.Buffer
	var lines []string

	lw := NewLineWriter(&downstream, func(line string) {
		lines = append(lines, line)
	})

	// Binary data with embedded newlines
	input := []byte{0x00, 0x01, 0x02, '\n', 0xFF, 0xFE, '\n'}
	_, err := lw.Write(input)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Downstream should have exact bytes
	if !bytes.Equal(downstream.Bytes(), input) {
		t.Errorf("downstream mismatch")
	}

	// Should have 2 lines
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestLineWriter_LongLine(t *testing.T) {
	var lines []string
	lw := NewLineWriter(nil, func(line string) {
		lines = append(lines, line)
	})

	// Create a very long line (1MB)
	longLine := make([]byte, 1024*1024)
	for i := range longLine {
		longLine[i] = 'x'
	}
	longLine = append(longLine, '\n')

	_, err := lw.Write(longLine)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if len(lines[0]) != 1024*1024 {
		t.Errorf("expected line length %d, got %d", 1024*1024, len(lines[0]))
	}
}

func TestLineWriter_MixedWrites(t *testing.T) {
	var lines []string
	lw := NewLineWriter(nil, func(line string) {
		lines = append(lines, line)
	})

	// Simulate realistic output patterns
	writes := []string{
		"Starting build...\n",
		"Compiling ",
		"main.go",
		"...\n",
		"Compiling utils.go...\n",
		"Build complete",
	}

	for _, w := range writes {
		_, _ = lw.Write([]byte(w))
	}

	// Before flush
	expected := []string{
		"Starting build...",
		"Compiling main.go...",
		"Compiling utils.go...",
	}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d: %v", len(expected), len(lines), lines)
	}
	for i, exp := range expected {
		if lines[i] != exp {
			t.Errorf("line %d: expected %q, got %q", i, exp, lines[i])
		}
	}

	// After flush
	lw.Flush()
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines after flush, got %d", len(lines))
	}
	if lines[3] != "Build complete" {
		t.Errorf("expected 'Build complete', got %q", lines[3])
	}
}

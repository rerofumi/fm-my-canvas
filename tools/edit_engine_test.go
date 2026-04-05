package tools

import (
	"strings"
	"testing"
)

func TestEditEngine_Apply_SingleMatch(t *testing.T) {
	engine := NewEditEngine()

	content := "hello world\nhello universe"
	search := "world"
	replace := "moon"

	result, err := engine.Apply(content, search, replace)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := "hello moon\nhello universe"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEditEngine_Apply_NoMatch(t *testing.T) {
	engine := NewEditEngine()

	content := "hello world"
	search := "moon"
	replace := "sun"

	_, err := engine.Apply(content, search, replace)

	if !IsNoMatch(err) {
		t.Fatalf("expected errNoMatch, got %v", err)
	}
}

func TestEditEngine_Apply_MultipleMatches(t *testing.T) {
	engine := NewEditEngine()

	content := "hello world\nhello world"
	search := "world"
	replace := "moon"

	_, err := engine.Apply(content, search, replace)

	if !IsMultipleMatches(err) {
		t.Fatalf("expected errMultipleMatches, got %v", err)
	}
}

func TestEditEngine_Apply_EmptySearch(t *testing.T) {
	engine := NewEditEngine()

	content := "hello world"
	search := ""
	replace := "moon"

	_, err := engine.Apply(content, search, replace)

	if !IsEmptySearch(err) {
		t.Fatalf("expected errEmptySearch, got %v", err)
	}
}

func TestEditEngine_Apply_Multiline(t *testing.T) {
	engine := NewEditEngine()

	content := "line1\nline2\nline3"
	search := "line2"
	replace := "replaced line"

	result, err := engine.Apply(content, search, replace)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := "line1\nreplaced line\nline3"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEditEngine_Apply_Idempotent(t *testing.T) {
	engine := NewEditEngine()

	content := "hello world"
	search := "world"
	replace := "world"

	result, err := engine.Apply(content, search, replace)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result != content {
		t.Errorf("expected %q, got %q", content, result)
	}
}

func TestEditEngine_Apply_CRLF(t *testing.T) {
	engine := NewEditEngine()

	content := "line1\r\nline2\r\nline3"
	search := "line2"
	replace := "replaced"

	result, err := engine.Apply(content, search, replace)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := "line1\r\nreplaced\r\nline3"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestEditEngine_Apply_CRLF_Mismatch(t *testing.T) {
	engine := NewEditEngine()

	content := "line1\r\nline2\r\nline3"
	search := "line1\nline2"
	replace := "replaced"

	_, err := engine.Apply(content, search, replace)

	if !IsNoMatch(err) {
		t.Fatalf("expected errNoMatch, got %v", err)
	}
}

func TestEditEngine_Apply_FileSizeLimit(t *testing.T) {
	engine := NewEditEngine()

	content := "hello world"
	search := "world"

	largeContent := strings.Repeat("x", maxFileSize+1)
	replace := largeContent

	_, err := engine.Apply(content, search, replace)

	if !IsFileSizeLimit(err) {
		t.Fatalf("expected errFileSizeLimit, got %v", err)
	}
}

func TestEditEngine_FindMatchCount(t *testing.T) {
	engine := NewEditEngine()

	content := "hello world\nhello world\nhello universe"

	count := engine.FindMatchCount(content, "world")
	if count != 2 {
		t.Errorf("expected 2 matches, got %d", count)
	}

	count = engine.FindMatchCount(content, "universe")
	if count != 1 {
		t.Errorf("expected 1 match, got %d", count)
	}

	count = engine.FindMatchCount(content, "moon")
	if count != 0 {
		t.Errorf("expected 0 matches, got %d", count)
	}
}

func TestEditEngine_FindMatchCount_MatchesApply(t *testing.T) {
	engine := NewEditEngine()

	content := "hello world\nhello universe"

	search := "world"
	replace := "moon"

	count := engine.FindMatchCount(content, search)
	if count != 1 {
		t.Fatalf("expected 1 match before apply, got %d", count)
	}

	result, err := engine.Apply(content, search, replace)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if strings.Contains(result, search) {
		t.Errorf("expected search string to be replaced")
	}

	if !strings.Contains(result, replace) {
		t.Errorf("expected replacement string to be present")
	}
}

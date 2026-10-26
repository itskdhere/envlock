package env

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteEnvFile(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	original := []byte("# Header comment\r\n\nVAR1=value1\nVAR2=\"multi\nline\nvalue\"\n  # Indented comment\nEMPTY=\nTRAILING_SPACE=abc   \n")

	if err := WriteEnvFile(envPath, original); err != nil {
		t.Fatalf("WriteEnvFile failed: %v", err)
	}

	readBack, err := ReadEnvFile(envPath)
	if err != nil {
		t.Fatalf("ReadEnvFile failed: %v", err)
	}

	if !bytes.Equal(original, readBack) {
		t.Fatalf("readBack does not match original byte-for-byte:\nGot:\n%s\nExpected:\n%s", string(readBack), string(original))
	}
}

func TestEnsureGitignore(t *testing.T) {
	tempDir := t.TempDir()

	// Initial run
	added, err := EnsureGitignore(tempDir)
	if err != nil {
		t.Fatalf("EnsureGitignore failed: %v", err)
	}
	if len(added) != 6 {
		t.Fatalf("expected 6 entries added, got %d", len(added))
	}

	// Second run should be no-op
	added2, err := EnsureGitignore(tempDir)
	if err != nil {
		t.Fatalf("EnsureGitignore second run failed: %v", err)
	}
	if len(added2) != 0 {
		t.Fatalf("expected 0 entries added on second run, got %d", len(added2))
	}

	// Verify content
	content, _ := os.ReadFile(filepath.Join(tempDir, ".gitignore"))
	str := string(content)
	for _, expected := range []string{".env", ".env.*.local", "*.key", "*.pub", "!env.lock", "!env.*.lock"} {
		if !strings.Contains(str, expected) {
			t.Errorf("gitignore missing expected entry: %s", expected)
		}
	}
}

func TestEnsureGitattributes(t *testing.T) {
	tempDir := t.TempDir()

	// Initial run
	added, err := EnsureGitattributes(tempDir)
	if err != nil {
		t.Fatalf("EnsureGitattributes failed: %v", err)
	}
	if len(added) != 2 {
		t.Fatalf("expected 2 entries added, got %d", len(added))
	}

	// Second run should be no-op
	added2, err := EnsureGitattributes(tempDir)
	if err != nil {
		t.Fatalf("EnsureGitattributes second run failed: %v", err)
	}
	if len(added2) != 0 {
		t.Fatalf("expected 0 entries added on second run, got %d", len(added2))
	}

	// Verify content
	content, _ := os.ReadFile(filepath.Join(tempDir, ".gitattributes"))
	str := string(content)
	if !strings.Contains(str, "env.lock merge=binary") || !strings.Contains(str, "env.*.lock merge=binary") {
		t.Errorf("gitattributes missing binary merge rules: %s", str)
	}
}

func TestResolvePaths(t *testing.T) {
	base := filepath.Clean("/my/project")

	// Default
	p1 := ResolvePaths(base, "", "")
	if p1.EnvFile != filepath.Join(base, ".env") || p1.LockFile != filepath.Join(base, "env.lock") {
		t.Errorf("default paths mismatch: %+v", p1)
	}

	// Named env
	p2 := ResolvePaths(base, "production", "")
	if p2.EnvFile != filepath.Join(base, ".env.production") || p2.LockFile != filepath.Join(base, "env.production.lock") {
		t.Errorf("named env mismatch: %+v", p2)
	}

	// Explicit file
	p3 := ResolvePaths(base, "", "custom.env")
	if p3.EnvFile != filepath.Join(base, "custom.env") || p3.LockFile != filepath.Join(base, "custom.env.lock") {
		t.Errorf("explicit file mismatch: %+v", p3)
	}

	// Explicit .env file
	p4 := ResolvePaths(base, "", ".env")
	if p4.EnvFile != filepath.Join(base, ".env") || p4.LockFile != filepath.Join(base, "env.lock") {
		t.Errorf("explicit .env mismatch: %+v", p4)
	}
}

func TestParse(t *testing.T) {
	src := "\xef\xbb\xbf# comment\r\n" +
		"export EXPORTED=yes\n" +
		"  INDENTED = spaced value   # inline comment\n" +
		"HASH=abc#123\n" +
		"COLOR=#ff0000\n" +
		"EMPTY=\n" +
		`DOUBLE="line1\nline2 \"quoted\" \\n"` + "\n" +
		"SINGLE='raw \\n $HOME # not a comment'\n" +
		"BACKTICK=`it's \"mixed\"`\n" +
		"MULTI=\"-----BEGIN-----\nabc\n-----END-----\" # trailing\n" +
		"DUP=first\n" +
		"DUP=second\n" +
		"LAST=no-newline"

	want := map[string]string{
		"EXPORTED": "yes",
		"INDENTED": "spaced value",
		"HASH":     "abc#123",
		"COLOR":    "#ff0000",
		"EMPTY":    "",
		"DOUBLE":   "line1\nline2 \"quoted\" \\n",
		"SINGLE":   `raw \n $HOME # not a comment`,
		"BACKTICK": `it's "mixed"`,
		"MULTI":    "-----BEGIN-----\nabc\n-----END-----",
		"DUP":      "second",
		"LAST":     "no-newline",
	}

	got, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !maps.Equal(got, want) {
		t.Fatalf("Parse mismatch:\nGot:  %q\nWant: %q", got, want)
	}

	for input, wantLine := range map[string]string{
		"NOEQUALS":                         "line 1:",
		"A=\"multi\nline\"\nBAD KEY=1":     "line 3:",
		"OK=1\nOPEN=\"never closed\nX=1\n": "line 2:",
		"TAIL=\"abc\"def":                  "line 1:",
		"SQ='it\\'s'":                      "line 1:",
		"A=\"oops\nB=1\nC=\"x\"":           "line 3:",
	} {
		if _, err := Parse([]byte(input)); err == nil || !strings.HasPrefix(err.Error(), wantLine) {
			t.Errorf("Parse(%q) error = %v, want prefix %q", input, err, wantLine)
		}
	}
}

func TestCreateDefaultEnv(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	created, err := CreateDefaultEnv(envPath)
	if err != nil {
		t.Fatalf("CreateDefaultEnv failed: %v", err)
	}
	if !created {
		t.Errorf("expected created to be true")
	}

	// Second call should not overwrite
	created2, err := CreateDefaultEnv(envPath)
	if err != nil {
		t.Fatalf("CreateDefaultEnv second run failed: %v", err)
	}
	if created2 {
		t.Errorf("expected created2 to be false")
	}
}

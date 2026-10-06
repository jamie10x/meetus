package upload

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuditKeepsReferencesYoungFilesAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := now.Add(-60 * 24 * time.Hour)
	names := []string{strings.Repeat("a", 32) + ".png", strings.Repeat("b", 32) + ".jpg", strings.Repeat("c", 32) + ".webp"}
	for i, name := range names {
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, []byte("image"), 0600); err != nil {
			t.Fatal(err)
		}
		if i < 2 {
			if err := os.Chtimes(file, old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.Symlink(filepath.Join(dir, names[1]), filepath.Join(dir, strings.Repeat("d", 32)+".png")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unmanaged.txt"), []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := Audit(context.Background(), dir, []string{"https://meetus.uz/uploads/" + names[0] + "?version=1"}, now.Add(-30*24*time.Hour), 1)
	if err != nil {
		t.Fatal(err)
	}
	if report.Files != 3 || report.Bytes != 15 || report.ReferencedFiles != 1 || report.UnreferencedFiles != 2 || report.EligibleFiles != 1 || report.EligibleBytes != 5 || report.IgnoredFiles != 2 || len(report.Candidates) != 1 || report.Candidates[0].Name != names[1] {
		t.Fatalf("unexpected inventory: %+v", report)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 5 {
		t.Fatal("audit modified the directory")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Audit(ctx, dir, nil, now, 0); err == nil {
		t.Fatal("audit ignored cancellation")
	}
}

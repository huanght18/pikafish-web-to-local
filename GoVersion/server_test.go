package main

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestOriginAllowed(t *testing.T) {
	oldCfg := cfg
	t.Cleanup(func() { cfg = oldCfg })
	cfg.AllowedOrigins = []string{"https://xiangqiai.com"}

	for _, origin := range []string{"", "https://xiangqiai.com", "HTTPS://XIANGQIAI.COM/"} {
		if !originAllowed(origin) {
			t.Fatalf("expected origin %q to be allowed", origin)
		}
	}
	if originAllowed("https://example.com") {
		t.Fatal("unexpectedly allowed an unlisted browser origin")
	}
}

func TestRewriteCommand(t *testing.T) {
	oldCfg := cfg
	t.Cleanup(func() { cfg = oldCfg })
	cfg.HashMB = 1024

	got, changed := rewriteCommand("SETOPTION name hash value 384")
	if !changed || got != "setoption name Hash value 1024" {
		t.Fatalf("rewriteCommand() = %q, %v", got, changed)
	}

	original := "info string setoption name Hash value 1"
	got, changed = rewriteCommand(original)
	if changed || got != original {
		t.Fatalf("unrelated command changed to %q", got)
	}
}

func TestValidateConfig(t *testing.T) {
	enginePath := filepath.Join(t.TempDir(), "pikafish.exe")
	if err := os.WriteFile(enginePath, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("valid", func(t *testing.T) {
		candidate := DefaultConfig()
		candidate.EnginePath = enginePath
		if err := ValidateConfig(candidate); err != nil {
			t.Fatalf("ValidateConfig() returned %v", err)
		}
	})

	t.Run("connection limit", func(t *testing.T) {
		candidate := DefaultConfig()
		candidate.EnginePath = enginePath
		candidate.MaxConnections = 0
		if err := ValidateConfig(candidate); err == nil {
			t.Fatal("expected invalid max_connections to be rejected")
		}
	})

	t.Run("log path traversal", func(t *testing.T) {
		candidate := DefaultConfig()
		candidate.EnginePath = enginePath
		candidate.Log.FilePath = filepath.Join("..", "outside.log")
		if err := ValidateConfig(candidate); err == nil {
			t.Fatal("expected escaping log.file_path to be rejected")
		}
	})
}

func TestExeDirUsesWorkingDirectoryDuringGoRunOrTest(t *testing.T) {
	got, err := exeDir()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("exeDir() = %q, want working directory %q", got, want)
	}
}

func TestLoadConfigCreatesEditableFileInWorkingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWorkingDirectory) })
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	candidate, path, created, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected LoadConfig to create a missing config")
	}
	if candidate.EnginePath != `C:\path\to\pikafish-bmi2.exe` {
		t.Fatalf("unexpected placeholder engine path %q", candidate.EnginePath)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("generated config isn't accessible: %v", err)
	}
}

func TestSetupLoggerCreatesLogsDirectoryBesideExecutable(t *testing.T) {
	tempDir := t.TempDir()
	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	oldWriter := log.Writer()
	t.Cleanup(func() {
		log.SetOutput(oldWriter)
		_ = os.Chdir(oldWorkingDirectory)
	})
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	closeLogger, err := setupLogger(LogConfig{
		File:     true,
		FilePath: filepath.Join("nested", "server.log"),
	})
	if err != nil {
		t.Fatal(err)
	}
	log.Print("test")
	if err := closeLogger(); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(tempDir, "logs", "nested", "server.log")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("log file wasn't created below logs/: %v", err)
	}
}

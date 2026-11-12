package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvLoadsFileWithoutOverridingProcessEnvironment(t *testing.T) {
	const processKey = "REMINDERBACKEND_TEST_DOTENV_PRECEDENCE"
	const fileKey = "REMINDERBACKEND_TEST_DOTENV_FILE"

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(
		processKey+"=from-file\n"+fileKey+"=loaded\n",
	), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	t.Setenv(processKey, "from-process")
	originalFileValue, hadFileValue := os.LookupEnv(fileKey)
	if err := os.Unsetenv(fileKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadFileValue {
			_ = os.Setenv(fileKey, originalFileValue)
		} else {
			_ = os.Unsetenv(fileKey)
		}
	})

	if err := LoadEnv(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(processKey); got != "from-process" {
		t.Errorf("%s = %q, want %q", processKey, got, "from-process")
	}
	if got := os.Getenv(fileKey); got != "loaded" {
		t.Errorf("%s = %q, want %q", fileKey, got, "loaded")
	}
}

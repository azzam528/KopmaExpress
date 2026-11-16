package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear relevant env vars
	_ = os.Unsetenv("PORT")
	_ = os.Unsetenv("APP_ENV")
	_ = os.Unsetenv("MONGO_URI")
	_ = os.Unsetenv("DB_NAME")
	_ = os.Unsetenv("MAX_UPLOAD_MB")

	cfg := Load()

	if cfg.Port != "3000" {
		t.Errorf("expected default Port 3000, got %s", cfg.Port)
	}
	if cfg.DBName != "smartcoop" {
		t.Errorf("expected default DBName smartcoop, got %s", cfg.DBName)
	}
	if cfg.MaxUploadMB != 50 {
		t.Errorf("expected default MaxUploadMB 50, got %d", cfg.MaxUploadMB)
	}
	if cfg.Workers <= 0 {
		t.Errorf("expected Workers > 0, got %d", cfg.Workers)
	}
}

func TestLoadCustomEnv(t *testing.T) {
	_ = os.Setenv("PORT", "8080")
	_ = os.Setenv("DB_NAME", "custom_coop")
	_ = os.Setenv("MAX_UPLOAD_MB", "100")
	_ = os.Setenv("WORKERS", "8")
	defer func() {
		_ = os.Unsetenv("PORT")
		_ = os.Unsetenv("DB_NAME")
		_ = os.Unsetenv("MAX_UPLOAD_MB")
		_ = os.Unsetenv("WORKERS")
	}()

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected Port 8080, got %s", cfg.Port)
	}
	if cfg.DBName != "custom_coop" {
		t.Errorf("expected DBName custom_coop, got %s", cfg.DBName)
	}
	if cfg.MaxUploadMB != 100 {
		t.Errorf("expected MaxUploadMB 100, got %d", cfg.MaxUploadMB)
	}
	if cfg.Workers != 8 {
		t.Errorf("expected Workers 8, got %d", cfg.Workers)
	}
}

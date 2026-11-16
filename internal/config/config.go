package config

import (
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	AppEnv      string
	MongoURI    string
	DBName      string
	MaxUploadMB int
	Workers     int
}

// Load loads configuration from environment variables with fallback defaults.
// It also attempts to read a local .env file if available.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("[CONFIG] Info: No .env file found or error loading, reading from environment variables")
	}

	return &Config{
		Port:        getEnv("PORT", "3000"),
		AppEnv:      getEnv("APP_ENV", "development"),
		MongoURI:    getEnv("MONGO_URI", "mongodb://localhost:27017/?replicaSet=rs0"),
		DBName:      getEnv("DB_NAME", "smartcoop"),
		MaxUploadMB: getEnvAsInt("MAX_UPLOAD_MB", 50),
		Workers:     getEnvAsInt("WORKERS", runtime.NumCPU()),
	}
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil || val <= 0 {
		return defaultVal
	}
	return val
}


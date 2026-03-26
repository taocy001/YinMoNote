package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// Config defines the application-wide configuration settings.
type Config struct {
	DataDir   string // Directory where note data is stored
	AssetsDir string // Directory for uploaded assets within DataDir
	Port      string // Port for the server to listen on
}

// defaultBaseDir returns ~/.yinmonote as the default base directory for both
// data and config, consistent across macOS and Linux.
func defaultBaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".yinmonote"
	}
	return filepath.Join(home, ".yinmonote")
}

// main is the entry point of the YinMo backend service.
func main() {
	// 1. Data/Config Separation: Load settings from environment.
	// Defaults: data → ~/.yinmonote/data, config → ~/.yinmonote/config.json
	// Override via DATA_DIR and CONFIG_FILE environment variables.
	base := defaultBaseDir()
	config := Config{
		DataDir:   getEnv("DATA_DIR", filepath.Join(base, "notes")),
		AssetsDir: "assets",
		Port:      getEnv("PORT", ":8080"),
	}
	configPath := getEnv("CONFIG_FILE", filepath.Join(base, "config.json"))

	// 2. Object-Oriented Initialization: Create a Library instance
	lib, err := NewNoteLibrary(config.DataDir, config.AssetsDir, configPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize NoteLibrary: %v", err))
	}

	// 3. Determine Port (Env > Config > Default)
	runPort := lib.Config.Port
	if envPort := os.Getenv("PORT"); envPort != "" {
		runPort = envPort
	}
	if runPort == "" {
		runPort = ":8080"
	}
	// Normalise: a bare port number (e.g. "8080") needs a leading colon for
	// net.Listen (":8080"). Full addresses like "127.0.0.1:8080" are left as-is.
	if !strings.Contains(runPort, ":") {
		runPort = ":" + runPort
	}

	// 4. Start background workers
	go lib.StartAutoCommitter()
	go lib.StartTrashPurger()
	go lib.StartGitGC()

	// 5. Server setup
	gin.SetMode(gin.ReleaseMode)
	NewServer(lib).Run(runPort)
}

// getEnv retrieves an environment variable or returns a fallback value if not set.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// ImportWorker contains the standalone processor's storage/runtime limits without API signing or Google credentials.
// DockerBinary and ImageID are set together or not at all; without them the worker processes DOCX natively only.
// WakeAddress is where the API's wake signal arrives; IdlePoll is the longest an idle worker waits without one.
type ImportWorker struct {
	Base                                               Config
	DockerBinary, ImageID                              string
	GlobalWorkers, ActorWorkers                        int
	ActorArtifactMiB, GlobalArtifactMiB, SetsPerImport int
	WakeAddress                                        string
	IdlePoll                                           time.Duration
}

// LoadImportWorker reads an explicitly configured private worker; starting the API never starts this runtime.
func LoadImportWorker() (ImportWorker, error) {
	w := ImportWorker{DockerBinary: os.Getenv("IMPORT_DOCKER_BINARY"), ImageID: os.Getenv("IMPORT_CONVERTER_IMAGE")}
	w.Base.DatabaseURL = os.Getenv("DATABASE_URL")
	if w.Base.DatabaseURL == "" {
		return w, fmt.Errorf("DATABASE_URL is required")
	}
	if err := loadMedia(&w.Base); err != nil {
		return w, err
	}
	if err := loadImports(&w.Base); err != nil {
		return w, err
	}
	if w.Base.ImportBucket == "" {
		return w, fmt.Errorf("worker requires private import storage")
	}
	if (w.DockerBinary != "" || w.ImageID != "") && (!filepath.IsAbs(w.DockerBinary) || !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(w.ImageID)) {
		return w, fmt.Errorf("legacy conversion requires an absolute IMPORT_DOCKER_BINARY and an immutable IMPORT_CONVERTER_IMAGE together")
	}
	for _, entry := range []struct {
		name              string
		target            *int
		fallback, maximum int
	}{
		{"IMPORT_WORKER_GLOBAL_LIMIT", &w.GlobalWorkers, 1, 32},
		{"IMPORT_WORKER_ACTOR_LIMIT", &w.ActorWorkers, 1, 32},
		{"IMPORT_ARTIFACT_ACTOR_MIB", &w.ActorArtifactMiB, 512, 1048576},
		{"IMPORT_ARTIFACT_GLOBAL_MIB", &w.GlobalArtifactMiB, 2048, 104857600},
		{"IMPORT_ARTIFACT_SETS_PER_ITEM", &w.SetsPerImport, 200, 1000},
	} {
		n, err := getenvInt(entry.name, entry.fallback)
		if err != nil || n < 1 || n > entry.maximum {
			return w, fmt.Errorf("%s must be between 1 and %d", entry.name, entry.maximum)
		}
		*entry.target = n
	}
	if w.ActorWorkers > w.GlobalWorkers || w.ActorArtifactMiB > w.GlobalArtifactMiB {
		return w, fmt.Errorf("worker actor limits must not exceed global limits")
	}
	return w, loadWorkerWake(&w)
}

func loadWorkerWake(w *ImportWorker) error {
	w.WakeAddress = getenv("IMPORT_WORKER_WAKE_ADDR", "localhost:8091")
	if _, _, err := net.SplitHostPort(w.WakeAddress); err != nil {
		return fmt.Errorf("IMPORT_WORKER_WAKE_ADDR must be host:port: %w", err)
	}
	var err error
	if w.IdlePoll, err = parseDuration("IMPORT_WORKER_IDLE_POLL", "1h"); err != nil {
		return err
	}
	if w.IdlePoll < time.Minute || w.IdlePoll > 24*time.Hour {
		return fmt.Errorf("IMPORT_WORKER_IDLE_POLL must be between 1m and 24h")
	}
	return nil
}

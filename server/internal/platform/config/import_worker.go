package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// ImportWorker contains the standalone processor's storage/runtime limits without API signing or Google credentials.
type ImportWorker struct {
	Base                                               Config
	DockerBinary, ImageID                              string
	GlobalWorkers, ActorWorkers                        int
	ActorArtifactMiB, GlobalArtifactMiB, SetsPerImport int
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
	if w.Base.ImportBucket == "" || !filepath.IsAbs(w.DockerBinary) || !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(w.ImageID) {
		return w, fmt.Errorf("worker requires private import storage, an absolute IMPORT_DOCKER_BINARY and immutable IMPORT_CONVERTER_IMAGE")
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
	return w, nil
}

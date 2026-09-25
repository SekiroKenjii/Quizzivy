// Command word-convert runs an isolated local Word rendition and writes private artifacts to a new disk directory.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"quizzivy/internal/platform/wordconvert"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	image := flag.String("image", "", "immutable local Docker image ID (sha256:...)")
	work := flag.String("work-dir", "", "existing absolute private disk directory")
	output := flag.String("output", "", "new absolute private artifact directory")
	flag.Parse()
	if flag.NArg() != 1 || !filepath.IsAbs(*output) {
		return fmt.Errorf("usage: word-convert -image sha256:... -work-dir /private/work -output /private/new-artifacts source.doc[x]")
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return err
	}
	converter, err := wordconvert.New(*work, docker, *image, 2*time.Minute)
	if err != nil {
		return err
	}
	source, err := os.Open(flag.Arg(0))
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return wordconvert.ErrSource
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(flag.Arg(0))), ".")
	result, err := converter.Convert(ctx, source, format)
	if err != nil {
		return err
	}
	defer func() { _ = result.Close() }()
	if err := save(*output, result); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Pages     int    `json:"pages"`
		Format    string `json:"sourceFormat"`
		Artifacts int    `json:"artifacts"`
	}{len(result.Manifest.Pages), result.Manifest.SourceFormat, len(result.Artifacts)})
}

func save(output string, result *wordconvert.Result) error {
	if err := os.Mkdir(output, 0o700); err != nil {
		return err
	}
	for _, artifact := range result.Artifacts {
		if err := copyArtifact(output, result, artifact.Name); err != nil {
			_ = os.RemoveAll(output)
			return err
		}
	}
	data, err := json.Marshal(struct {
		Manifest     wordconvert.Manifest   `json:"manifest"`
		SourceSHA256 string                 `json:"sourceSha256"`
		ImageID      string                 `json:"imageId"`
		Artifacts    []wordconvert.Artifact `json:"artifacts"`
	}{result.Manifest, result.SourceSHA256, result.ImageID, result.Artifacts})
	if err == nil {
		err = os.WriteFile(filepath.Join(output, "manifest.json"), data, 0o600)
	}
	if err != nil {
		_ = os.RemoveAll(output)
	}
	return err
}

func copyArtifact(output string, result *wordconvert.Result, name string) error {
	source, err := result.Open(name)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(filepath.Join(output, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

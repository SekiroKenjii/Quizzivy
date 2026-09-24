// Package wordconvert runs an offline, resource-limited Word converter and retains only private, validated artifacts.
package wordconvert

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"quizzivy/internal/platform/word"
)

var (
	ErrConfiguration = errors.New("wordconvert: invalid isolated runtime configuration")
	ErrBusy          = errors.New("wordconvert: converter busy")
	ErrSource        = errors.New("wordconvert: invalid or unsupported source")
	ErrLimit         = errors.New("wordconvert: resource limit exceeded")
	ErrConversion    = errors.New("wordconvert: conversion failed")
	ErrCleanup       = errors.New("wordconvert: container cleanup failed; private job retained")
)

const legacyFormat = "doc"
const nativeFormat = "docx"
const mountOption = "--mount"
const converterSlot = "quizzivy-word-converter-slot"

var imageIdentity = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// Converter admits one document at a time and invokes an immutable local Docker image without granting it the Docker socket or host secrets.
type Converter struct {
	root, executable, image string
	timeout                 time.Duration
	slot                    chan struct{}
}

// New requires a private disk directory, an explicit Docker binary and an immutable image ID; no image is pulled at runtime.
func New(root, executable, image string, timeout time.Duration) (*Converter, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(executable) || !imageIdentity.MatchString(image) || timeout < time.Second || timeout > 3*time.Minute || strings.ContainsAny(root, ",\x00") || os.Getuid() == 0 {
		return nil, ErrConfiguration
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved != filepath.Clean(root) {
		return nil, ErrConfiguration
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return nil, ErrConfiguration
	}
	return &Converter{root: root, executable: executable, image: image, timeout: timeout, slot: make(chan struct{}, 1)}, nil
}

// Convert copies the bounded source into a private job, converts offline and returns artifacts whose caller must Close after storage; originals are never modified.
func (c *Converter) Convert(ctx context.Context, source io.Reader, format string) (*Result, error) {
	if format != legacyFormat && format != nativeFormat {
		return nil, ErrSource
	}
	select {
	case c.slot <- struct{}{}:
		defer func() { <-c.slot }()
	default:
		return nil, ErrBusy
	}
	job, err := os.MkdirTemp(c.root, "conversion-")
	if err != nil {
		return nil, err
	}
	result, err := c.convert(ctx, job, source, format)
	if err != nil && !errors.Is(err, ErrCleanup) {
		_ = os.RemoveAll(job)
	}
	return result, err
}

func (c *Converter) convert(ctx context.Context, job string, source io.Reader, format string) (*Result, error) {
	for _, name := range []string{"input", "work", "fallback-tmp"} {
		if err := os.Mkdir(filepath.Join(job, name), 0o700); err != nil {
			return nil, err
		}
	}
	checksum, err := stageSource(ctx, filepath.Join(job, "input", "source."+format), source, format)
	if err != nil {
		return nil, err
	}
	if err := c.reapStoppedSlot(ctx); err != nil {
		return nil, err
	}
	owner := uuid.NewString()
	work, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	finished := make(chan struct{})
	monitored := make(chan error, 1)
	go monitorDisk(work, job, finished, monitored, cancel)
	cmd := exec.CommandContext(work, c.executable, c.arguments(job, owner, format)...)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	runErr := cmd.Run()
	close(finished)
	limitErr := <-monitored
	owned, err := c.removeOwnedContainer(job, owner)
	if err != nil {
		return nil, err
	}
	if limitErr != nil {
		return nil, limitErr
	}
	if work.Err() != nil {
		return nil, work.Err()
	}
	if runErr != nil {
		if !owned {
			if occupied, err := c.slotOccupied(ctx); err == nil && occupied {
				return nil, ErrBusy
			}
		}
		return nil, ErrConversion
	}
	return readResult(work, job, c.image, checksum, format)
}

func (c *Converter) arguments(job, owner, format string) []string {
	uid := strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
	return []string{"run", "--pull=never", "--name", converterSlot, "--cidfile", filepath.Join(job, "container-id"), "--label", "quizzivy.word-converter=true", "--label", "quizzivy.word-converter-owner=" + owner,
		"--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges",
		"--user", uid, "--pids-limit=64", "--memory=512m", "--memory-swap=512m", "--cpus=1",
		"--shm-size=16m", "--ulimit", "nofile=256:256", "--ulimit", "fsize=134217728:134217728",
		mountOption, "type=bind,src=" + filepath.Join(job, "input") + ",dst=/input,readonly",
		mountOption, "type=bind,src=" + filepath.Join(job, "work") + ",dst=/work",
		mountOption, "type=bind,src=" + filepath.Join(job, "fallback-tmp") + ",dst=/tmp",
		c.image, format}
}

func (c *Converter) removeContainer(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, c.executable, "rm", "--force", id).Run(); err != nil {
		output, inspectErr := exec.CommandContext(ctx, c.executable, "ps", "--all", "--filter", "id="+id, "--format", "{{.ID}}").Output()
		if inspectErr != nil || strings.TrimSpace(string(output)) != "" {
			return ErrCleanup
		}
	}
	return nil
}

func stageSource(ctx context.Context, name string, source io.Reader, format string) (string, error) {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(cancelReader{ctx: ctx, r: source}, (25<<20)+1))
	if err != nil {
		return "", err
	}
	if n > 25<<20 {
		return "", ErrLimit
	}
	var head [8]byte
	if _, err := f.ReadAt(head[:], 0); err != nil {
		return "", ErrSource
	}
	ole := [8]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}
	if format == legacyFormat && head != ole || format == nativeFormat && string(head[:4]) != "PK\x03\x04" {
		return "", ErrSource
	}
	if format == nativeFormat {
		if _, err := word.Inspect(ctx, f, n, word.DefaultLimits()); err != nil {
			return "", ErrSource
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

type cancelReader struct {
	ctx context.Context
	r   io.Reader
}

func (r cancelReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

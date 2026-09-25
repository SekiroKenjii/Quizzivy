package wordconvert

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var containerIdentity = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

func (c *Converter) removeOwnedContainer(job, owner string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(job, "container-id"))
	id := strings.TrimSpace(string(data))
	if err == nil && containerIdentity.MatchString(id) {
		return true, c.removeContainer(id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, c.executable, "ps", "--all", "--filter", "label=quizzivy.word-converter-owner="+owner, "--format", "{{.ID}}").Output()
	if err != nil {
		return false, ErrCleanup
	}
	id = strings.TrimSpace(string(output))
	if id == "" {
		return false, nil
	}
	if !containerIdentity.MatchString(id) {
		return false, ErrCleanup
	}
	return true, c.removeContainer(id)
}

func (c *Converter) reapStoppedSlot(ctx context.Context) error {
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(bounded, c.executable, "ps", "--all", "--filter", "name=^/"+converterSlot+"$", "--filter", "label=quizzivy.word-converter=true", "--filter", "status=exited", "--filter", "status=dead", "--filter", "status=created", "--format", "{{.ID}}").Output()
	if err != nil {
		return ErrConversion
	}
	id := strings.TrimSpace(string(output))
	if id == "" {
		return nil
	}
	if !containerIdentity.MatchString(id) {
		return ErrConversion
	}
	return c.removeContainer(id)
}

func (c *Converter) slotOccupied(ctx context.Context) (bool, error) {
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(bounded, c.executable, "ps", "--all", "--filter", "name=^/"+converterSlot+"$", "--format", "{{.ID}}").Output()
	return len(strings.TrimSpace(string(output))) > 0, err
}

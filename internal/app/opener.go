package app

import (
	"context"
	"os/exec"
)

// Opener opens a link target with the platform's opener (LINK-1). Open blocks until the opener
// has started and exited, so the viewer calls it from a goroutine.
type Opener interface {
	Open(ctx context.Context, target string) error
}

// execOpener runs the program name with the target as its one argument (LINK-2: no shell).
type execOpener struct {
	name string
}

// Open starts the program and waits for it. Stub: not yet implemented.
func (o execOpener) Open(ctx context.Context, target string) error {
	return nil
}

// command is the command Open runs. Stub: an empty command.
func (o execOpener) command(ctx context.Context, target string) *exec.Cmd {
	return &exec.Cmd{}
}

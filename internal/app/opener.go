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

// Open starts the program and waits for it, and returns the first error of either (LINK-2). The
// program's own stdout and stderr are left nil, so they are discarded.
func (o execOpener) Open(ctx context.Context, target string) error {
	cmd := o.command(ctx, target)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

// command is the command Open runs: the program with the target as its one argument, and no
// shell (LINK-2).
func (o execOpener) command(ctx context.Context, target string) *exec.Cmd {
	return exec.CommandContext(ctx, o.name, target)
}

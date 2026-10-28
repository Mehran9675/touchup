package media

import (
	"context"
	"errors"
	"os/exec"
	"sync"

	"github.com/mehran9675/touchup/internal/proc"
)

// ErrNoPlayer is returned when ffplay isn't installed.
var ErrNoPlayer = errors.New("ffplay was not found: install FFmpeg with ffplay to listen to previews")

// Player plays one audio file at a time through ffplay.
type Player struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	playing string
	// OnChange is called from a background goroutine when playback
	// starts or stops, with the file playing or "".
	OnChange func(playing string)
}

// Play stops whatever is playing and starts path.
func (p *Player) Play(tools Tools, path string) error {
	if tools.FFplay == "" {
		return ErrNoPlayer
	}
	p.Stop()
	cmd := proc.Command(context.Background(), tools.FFplay,
		"-nodisp", "-autoexit", "-loglevel", "quiet", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	p.mu.Lock()
	p.cmd, p.playing = cmd, path
	p.mu.Unlock()
	p.notify(path)
	go func() {
		cmd.Wait()
		p.mu.Lock()
		current := p.cmd == cmd
		if current {
			p.cmd, p.playing = nil, ""
		}
		p.mu.Unlock()
		if current {
			p.notify("")
		}
	}()
	return nil
}

// Stop ends playback.
func (p *Player) Stop() {
	p.mu.Lock()
	cmd := p.cmd
	p.cmd, p.playing = nil, ""
	p.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
		p.notify("")
	}
}

// Playing returns the file playing, or "".
func (p *Player) Playing() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}

func (p *Player) notify(path string) {
	if p.OnChange != nil {
		p.OnChange(path)
	}
}

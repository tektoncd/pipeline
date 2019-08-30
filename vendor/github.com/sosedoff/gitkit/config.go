package gitkit

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

type Config struct {
	KeyDir     string            // Directory for server ssh keys. Only used in SSH strategy.
	Dir        string            // Directory that contains repositories
	GitPath    string            // Path to git binary
	GitUser    string            // User for ssh connections
	AutoCreate bool              // Automatically create repostories
	AutoHooks  bool              // Automatically setup git hooks
	Hooks      map[string][]byte // Scripts for hooks/* directory
	Auth       bool              // Require authentication
}

func (c *Config) KeyPath() string {
	return filepath.Join(c.KeyDir, "gitkit.rsa")
}

func (c *Config) Setup() error {
	if _, err := os.Stat(c.Dir); err != nil {
		if err = os.Mkdir(c.Dir, 0755); err != nil {
			return err
		}
	}

	if c.AutoHooks == true {
		return c.setupHooks()
	}

	return nil
}

func (c *Config) setupHooks() error {
	files, err := ioutil.ReadDir(c.Dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		hooksPath := path.Join(c.Dir, file.Name(), "hooks")

		// Cleanup all existing hooks
		hookFiles, err := ioutil.ReadDir(hooksPath)
		if err == nil {
			for _, h := range hookFiles {
				os.Remove(path.Join(hooksPath, h.Name()))
			}
		}

		// Setup new hooks
		for hook, script := range c.Hooks {
			if err := ioutil.WriteFile(path.Join(hooksPath, hook), []byte(script), 0755); err != nil {
				logError("hook-update", err)
				return err
			}
		}
	}

	return nil
}

package main

import (
	"fmt"
	"path/filepath"

	"github.com/startvibecoding/vibecoding/internal/config"
	"github.com/startvibecoding/vibecoding/internal/cron"
)

// openCronStore opens the hermes cron store file.
func openCronStore() *cron.FileCronStore {
	path := filepath.Join(config.ConfigDir(), "hermes-cron.json")
	return cron.NewFileCronStore(path)
}

// setCronEnabled enables or disables a cron job by ID.
func setCronEnabled(id string, enabled bool) error {
	store := openCronStore()
	job, err := store.Get(id)
	if err != nil {
		return err
	}
	job.Enabled = enabled
	if err := store.Update(*job); err != nil {
		return err
	}
	state := "enabled"
	if !enabled {
		state = "disabled"
	}
	fmt.Printf("✅ %s: [%s] %s\n", state, job.ID, job.Name)
	return nil
}

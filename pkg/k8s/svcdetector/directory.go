package svcdetector

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

type directoryMonitoringAppStateManager struct {
	changes chan AppStateChange
}

func (d *directoryMonitoringAppStateManager) Changes() chan AppStateChange {
	return d.changes
}

func parseAppFromPath(fs afero.Fs, path string) (apps.App, error) {
	file, err := fs.Open(path)
	if err != nil {
		return apps.App{}, fmt.Errorf("failed to open file when trying to parse app: %w", err)
	}
	defer file.Close()

	var app apps.App
	err = json.NewDecoder(file).Decode(&app)
	if err != nil {
		return apps.App{}, fmt.Errorf("failed to decode file when trying to parse app: %w", err)
	}

	return apps.App{
		Name:         app.Name,
		Address:      app.Address,
		Peer:         app.Peer,
		OriginalPort: app.OriginalPort,
		TargetLabels: app.TargetLabels,
	}, nil
}

// NewDirectoryMonitoringAppStateManager is used for integration testing
func NewDirectoryMonitoringAppStateManager(location string, fs afero.Fs) AppStateManager { // nolint: gocognit
	changesChan := make(chan AppStateChange)
	lastReadFiles := make(map[string]apps.App)
	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				files := make(map[string]apps.App)
				if walkErr := afero.Walk(fs, location, func(path string, _ os.FileInfo, err error) error {
					if err == nil {
						app, err := parseAppFromPath(fs, path)
						if err != nil {
							logrus.Errorf("Failed to parse app from path %s: %v", path, err)
							return nil
						}
						files[path] = app
					}
					return nil
				}); walkErr != nil {
					logrus.Errorf("Failed to walk directory: %v", walkErr)
					continue
				}

				for file := range files {
					if _, ok := lastReadFiles[file]; !ok {
						changesChan <- AppStateChange{
							App: apps.App{
								Name:    file,
								Address: file,
							},
							State: AppStateChangeAdded,
						}
					}
				}

				for file := range lastReadFiles {
					if _, ok := files[file]; !ok {
						changesChan <- AppStateChange{
							App: apps.App{
								Name:    file,
								Address: file,
							},
							State: AppStateChangeWithdrawn,
						}
					}
				}

			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	return &directoryMonitoringAppStateManager{
		changes: changesChan,
	}
}

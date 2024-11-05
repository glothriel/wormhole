package pairing

import (
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/go-ping/ping"
)

type pinger interface {
	Ping(address string) error
}

type defaultPinger struct{}

func (p *defaultPinger) Ping(address string) error {
	pinger, pingerErr := ping.NewPinger(address)
	if pingerErr != nil {
		return fmt.Errorf("failed to create pinger: %v", pingerErr)
	}
	pinger.Count = 3
	pinger.Timeout = 3 * time.Second
	runErr := pinger.Run()
	if runErr != nil {
		return fmt.Errorf("failed to run pinger: %v", runErr)
	}
	if pinger.Statistics().PacketsRecv == 0 {
		return fmt.Errorf("failed to ping server %s over the tunnel", address)
	}
	return nil
}

type retryingPinger struct {
	pinger pinger
}

// uses avast/retry-go to retry pings
func (p *retryingPinger) Ping(address string) error {
	return retry.Do(func() error {
		return p.pinger.Ping(address)
	}, retry.Attempts(5), retry.Delay(time.Second))
}

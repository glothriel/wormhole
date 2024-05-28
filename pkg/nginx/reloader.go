package nginx

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/avast/retry-go/v4"
	"github.com/mitchellh/go-ps"
)

// Reloader is an interface that allows to reload nginx server
type Reloader interface {
	Reload() error
}

type lowestMatchingProcessIDReloader struct {
}

func (r *lowestMatchingProcessIDReloader) Reload() error {
	max := 1999999999
	nginxMasterPid := max
	p, processListErr := ps.Processes()
	if processListErr != nil {
		return fmt.Errorf("could not list processes: %v", processListErr)
	}
	for _, process := range p {
		if process.Executable() == "nginx" && process.Pid() < nginxMasterPid {
			nginxMasterPid = process.Pid()
		}
	}
	if nginxMasterPid == max {
		return errors.New("no nginx process found")
	}

	if killErr := syscall.Kill(nginxMasterPid, syscall.SIGHUP); killErr != nil {
		return fmt.Errorf("could not reload nginx: %v", killErr)
	}
	return nil
}

type retryingReloader struct {
	child Reloader
	tries int
}

func (r *retryingReloader) Reload() error {
	return retry.Do(
		func() error {
			return r.child.Reload()
		},
		retry.Attempts(uint(r.tries)),
		retry.DelayType(retry.BackOffDelay),
	)
}

// NewRetryingReloader creates a new RetryingReloader
func NewRetryingReloader(child Reloader, tries int) Reloader {
	return &retryingReloader{
		child: child,
		tries: tries,
	}
}

// NewPidBasedReloader creates a new PidBasedReloader
func NewPidBasedReloader() Reloader {
	return &lowestMatchingProcessIDReloader{}
}

// NewDefaultReloader creates a pre-configured reloader, that is retrying 10 times
func NewDefaultReloader() Reloader {
	return NewRetryingReloader(NewPidBasedReloader(), 10)
}

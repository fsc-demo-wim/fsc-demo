package fscctrlr

import (
	"time"

	"github.com/fsc-demo-wim/fsc-demo/common/client"
	log "github.com/sirupsen/logrus"
)

// FscCtrlr struct holds the structure of the
type FscCtrlr struct {
	ConfigFile    *string
	HealthEnabled bool
	Clients       *client.Info
	CtrlrCtrl     *controllerControl

	debug   bool
	timeout time.Duration
}

// Option struct
type Option func(fa *FscCtrlr)

// WithDebug function
func WithDebug(d bool) Option {
	return func(fa *FscCtrlr) {
		fa.debug = d
	}
}

// WithTimeout function
func WithTimeout(dur time.Duration) Option {
	return func(fa *FscCtrlr) {
		fa.timeout = dur
	}
}

// WithConfigFile function
func WithConfigFile(file string) Option {
	return func(fa *FscCtrlr) {
		if file == "" {
			return
		}
		fa.ConfigFile = &file
		log.Info(file)
	}
}

// New function allocates a new fscAgent
func New(opts ...Option) (*FscCtrlr, error) {
	fa := &FscCtrlr{
		ConfigFile: new(string),
	}

	for _, o := range opts {
		o(fa)
	}
	var err error
	fa.Clients, err = client.GetClients(fa.ConfigFile)
	if err != nil {
		return nil, err
	}

	return fa, nil
}

package fscagent

import (
	"time"

	"github.com/henderiw/fsc-demo/common/client"
	log "github.com/sirupsen/logrus"
)

// FscAgent struct holds the structure of the
type FscAgent struct {
	ConfigFile    *string
	HealthEnabled bool
	Clients       *client.Info
	CtrlrCtrl     *controllerControl

	debug   bool
	timeout time.Duration
}

// Cache hold the ephemiral data of the fsc Agent
type Cache struct {
	SriovConfig  *map[string]*string
	MultusConfig *map[string]*string
}

// Device is a struct that contains the information of a device element
type Device struct {
	Kind      string
	ID        string
	IDType    string
	Endpoints map[string]*Endpoint
}

// Endpoint is a struct that contains information of a link endpoint
type Endpoint struct {
	Device *Device
	ID     string
	IDType string
}

// Option struct
type Option func(fa *FscAgent)

// WithDebug function
func WithDebug(d bool) Option {
	return func(fa *FscAgent) {
		fa.debug = d
	}
}

// WithTimeout function
func WithTimeout(dur time.Duration) Option {
	return func(fa *FscAgent) {
		fa.timeout = dur
	}
}

// WithConfigFile function
func WithConfigFile(file string) Option {
	return func(fa *FscAgent) {
		if file == "" {
			return
		}
		fa.ConfigFile = &file
		log.Info(file)
	}
}

// New function allocates a new fscAgent
func New(opts ...Option) (*FscAgent, error) {
	fa := &FscAgent{
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

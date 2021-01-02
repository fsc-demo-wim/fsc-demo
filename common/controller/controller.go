package controller

import "github.com/henderiw/fsc-demo/common/msg"

// Controller interface for the functions of the controller
type Controller interface {
	// Run method
	Run(stopCh chan struct{}, workCh chan msg.CMsg)
}

package agent

import (
	"context"
	"log"

	"github.com/kardianos/service"
)

// ServiceName is the Windows service name this agent is installed under (see bootstrap.ps1).
const ServiceName = "wimp-agent"

// Agent implements service.Interface so it can be managed by the OS service manager.
type Agent struct {
	cfg    *Config
	log    service.Logger
	cancel context.CancelFunc
}

func New(cfg *Config) *Agent {
	return &Agent{cfg: cfg}
}

// Start is called by the service manager on startup. Must return quickly.
func (a *Agent) Start(svc service.Service) error {
	logger, err := svc.Logger(nil)
	if err != nil {
		log.Printf("could not get service logger: %v", err)
	}
	a.log = logger

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	go a.run(ctx)
	return nil
}

// Stop is called by the service manager on shutdown.
func (a *Agent) Stop(svc service.Service) error {
	if a.cancel != nil {
		a.cancel()
	}
	return nil
}

func (a *Agent) run(ctx context.Context) {
	a.logger().Infof("wimp agent starting - version %s", Version)
	a.logger().Infof("control plane: %s  machine_id: %d", a.cfg.ControlPlaneUrl, a.cfg.MachineId)

	a.connect(ctx)

	a.logger().Info("wimp agent stopped")
}

func (a *Agent) logger() service.Logger {
	if a.log != nil {
		return a.log
	}
	return &stdLogger{}
}

// stdLogger is a fallback that writes to stdout when the service logger is unavailable.
type stdLogger struct{}

func (s *stdLogger) Error(v ...interface{}) error   { log.Println(v...); return nil }
func (s *stdLogger) Warning(v ...interface{}) error { log.Println(v...); return nil }
func (s *stdLogger) Info(v ...interface{}) error    { log.Println(v...); return nil }
func (s *stdLogger) Errorf(format string, a ...interface{}) error {
	log.Printf(format, a...)
	return nil
}
func (s *stdLogger) Warningf(format string, a ...interface{}) error {
	log.Printf(format, a...)
	return nil
}
func (s *stdLogger) Infof(format string, a ...interface{}) error {
	log.Printf(format, a...)
	return nil
}

package main

import (
	"log"

	"github.com/kardianos/service"
	"github.com/su3i/wimp/internal/agent"
)

const (
	svcDisplayName = "wimp Agent"
	svcDescription = "wimp Windows Infrastructure Management Agent"
)

func main() {
	cfg, err := agent.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	svcConfig := &service.Config{
		Name:        agent.ServiceName,
		DisplayName: svcDisplayName,
		Description: svcDescription,
	}

	a := agent.New(cfg)

	svc, err := service.New(a, svcConfig)
	if err != nil {
		log.Fatalf("failed to create service: %v", err)
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("service error: %v", err)
	}
}

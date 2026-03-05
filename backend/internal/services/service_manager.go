package services

import (
	"fmt"
	"sync"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// ServiceManager manages installable services.
type ServiceManager struct {
	mu       sync.RWMutex
	services map[string]*models.ServiceInfo
}

// NewServiceManager creates a ServiceManager with default service definitions.
func NewServiceManager() *ServiceManager {
	sm := &ServiceManager{
		services: make(map[string]*models.ServiceInfo),
	}
	sm.services["adguardhome"] = &models.ServiceInfo{
		ID: "adguardhome", Name: "AdGuard Home",
		Description: "Network-wide ad and tracker blocking DNS server",
		State: "not_installed",
	}
	sm.services["wireguard"] = &models.ServiceInfo{
		ID: "wireguard", Name: "WireGuard",
		Description: "Fast, modern VPN tunnel",
		State: "stopped", AutoStart: false,
	}
	sm.services["tailscale"] = &models.ServiceInfo{
		ID: "tailscale", Name: "Tailscale",
		Description: "Zero-config mesh VPN",
		State: "not_installed",
	}
	return sm
}

// ListServices returns all known services.
func (sm *ServiceManager) ListServices() ([]models.ServiceInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var result []models.ServiceInfo
	for _, s := range sm.services {
		result = append(result, *s)
	}
	return result, nil
}

// GetServiceStatus returns the status of a specific service.
func (sm *ServiceManager) GetServiceStatus(serviceID string) (models.ServiceInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if s, ok := sm.services[serviceID]; ok {
		return *s, nil
	}
	return models.ServiceInfo{}, fmt.Errorf("service not found: %s", serviceID)
}

// Install marks a service as installed (stopped).
func (sm *ServiceManager) Install(serviceID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.services[serviceID]
	if !ok {
		return fmt.Errorf("service not found: %s", serviceID)
	}
	if s.State != "not_installed" {
		return fmt.Errorf("service %s already installed", serviceID)
	}
	s.State = "stopped"
	return nil
}

// Remove marks a service as not installed.
func (sm *ServiceManager) Remove(serviceID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.services[serviceID]
	if !ok {
		return fmt.Errorf("service not found: %s", serviceID)
	}
	s.State = "not_installed"
	return nil
}

// Start marks a service as running.
func (sm *ServiceManager) Start(serviceID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.services[serviceID]
	if !ok {
		return fmt.Errorf("service not found: %s", serviceID)
	}
	if s.State == "not_installed" {
		return fmt.Errorf("service %s not installed", serviceID)
	}
	s.State = "running"
	return nil
}

// Stop marks a service as stopped.
func (sm *ServiceManager) Stop(serviceID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.services[serviceID]
	if !ok {
		return fmt.Errorf("service not found: %s", serviceID)
	}
	if s.State == "not_installed" {
		return fmt.Errorf("service %s not installed", serviceID)
	}
	s.State = "stopped"
	return nil
}

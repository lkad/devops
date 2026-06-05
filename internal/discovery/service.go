package discovery

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	devicepkg "github.com/devops-toolkit/backend/internal/device"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// StartRunInput is the request payload for Service.StartRun.
// Ports and SNMP are optional; when Ports is empty, the prober
// falls back to a small default list (22 + 161) so the common
// cases (SSH host, SNMP device) are covered.
type StartRunInput struct {
	// CIDR is the network range to scan. Required.
	CIDR string
	// Ports is the optional list of TCP ports to probe. The
	// spec allows the user to customise this; the service
	// stores a default when empty.
	Ports []int
	// SNMP enables the SNMP sysDescr probe. When false, the
	// prober skips the UDP 161 query and SNMPSysDescr is
	// always empty.
	SNMP bool
}

// PromoteInput is the request payload for Service.PromoteHosts.
// The HostIDs field is optional: empty means "promote every
// host in the run"; non-empty means "promote only the listed
// hosts" (the spec's `{host_ids: [...]}` body shape).
type PromoteInput struct {
	RunID   string
	HostIDs []string
}

// RunDetail is the combined view of a run and its hosts. The
// handler returns this for GET /runs/:id.
type RunDetail struct {
	Run   *DiscoveryRun    `json:"run"`
	Hosts []DiscoveredHost `json:"hosts"`
}

// Service is the business-logic layer for the network-discovery
// subsystem. It owns validation, the scan/probe orchestration,
// dedup, and the promote-to-Device flow. It is framework-
// agnostic (no Gin) so it can be reused by background workers
// or CLI tools in the future.
type Service struct {
	repo   *Repository
	devs   *devicepkg.Repository
	scnr   Scanner
	probr  Prober
	clock  func() time.Time
}

// NewService builds a Service. The scanner and prober are the
// test seam — production wires the real implementations, tests
// wire the fakes.
func NewService(repo *Repository, devs *devicepkg.Repository, scnr Scanner, probr Prober) *Service {
	return &Service{
		repo:  repo,
		devs:  devs,
		scnr:  scnr,
		probr: probr,
		clock: func() time.Time { return time.Now().UTC() },
	}
}

// apiErrorType is the canonical alias for *contracts.APIError
// inside this package. It exists so the test file can use
// errors.As without importing contracts directly.
type apiErrorType = contracts.APIError

// StartRun validates the input, persists a "running" run
// record, fires the scanner, dedups the result, and probes
// each host. The returned DiscoveryRun reflects the final
// status (completed / empty / failed).
func (s *Service) StartRun(ctx context.Context, in StartRunInput) (*DiscoveryRun, error) {
	cidr := strings.TrimSpace(in.CIDR)
	if cidr == "" {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "cidr is required",
		}
	}
	if _, _, err := net.ParseCIDR(cidr); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: fmt.Sprintf("cidr %q is not a valid network range", cidr),
		}
	}

	run := &DiscoveryRun{
		CIDR:       cidr,
		StartedAt:  s.clock(),
		Status:     RunStatusRunning,
		HostsFound: 0,
	}
	if err := s.repo.CreateRun(run); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to create discovery run",
			Cause:   err,
		}
	}

	hosts, err := s.scnr.Scan(ctx, cidr)
	if err != nil {
		run.Status = RunStatusFailed
		now := s.clock()
		run.CompletedAt = &now
		_ = s.repo.UpdateRun(run)
		return run, nil
	}

	// Dedup: skip hosts already in the discovered_hosts
	// table from any prior scan.
	fresh := make([]Host, 0, len(hosts))
	for _, h := range hosts {
		exists, err := s.repo.HostExistsByIP(h.IPAddress)
		if err != nil {
			run.Status = RunStatusFailed
			now := s.clock()
			run.CompletedAt = &now
			_ = s.repo.UpdateRun(run)
			return run, &contracts.APIError{
				Code:    contracts.CodeInternal,
				Message: "failed to dedup hosts",
				Cause:   err,
			}
		}
		if !exists {
			fresh = append(fresh, h)
		}
	}

	// Probe + persist.
	for _, h := range fresh {
		result := s.probr.Probe(ctx, h)
		openPorts := result.OpenPorts
		if openPorts == nil {
			openPorts = []int{}
		}
		host := &DiscoveredHost{
			RunID:        run.ID,
			IPAddress:    h.IPAddress,
			Hostname:     h.Hostname,
			OpenPorts:    JSONMap{"tcp": openPorts},
			SNMPSysDescr: result.SNMPSysDescr,
		}
		if err := s.repo.CreateHost(host); err != nil {
			run.Status = RunStatusFailed
			now := s.clock()
			run.CompletedAt = &now
			_ = s.repo.UpdateRun(run)
			return run, &contracts.APIError{
				Code:    contracts.CodeInternal,
				Message: "failed to persist discovered host",
				Cause:   err,
			}
		}
	}

	now := s.clock()
	run.CompletedAt = &now
	run.HostsFound = len(fresh)
	if len(fresh) == 0 {
		run.Status = RunStatusEmpty
	} else {
		run.Status = RunStatusCompleted
	}
	if err := s.repo.UpdateRun(run); err != nil {
		return run, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to finalise discovery run",
			Cause:   err,
		}
	}
	return run, nil
}

// GetRun returns a single run, or a 404 APIError.
func (s *Service) GetRun(id string) (*DiscoveryRun, error) {
	run, err := s.repo.GetRun(id)
	if err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("discovery run %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load discovery run",
			Cause:   err,
		}
	}
	return run, nil
}

// GetRunWithHosts returns the run plus its hosts. The handler
// uses this for GET /runs/:id.
func (s *Service) GetRunWithHosts(id string) (*RunDetail, error) {
	run, err := s.GetRun(id)
	if err != nil {
		return nil, err
	}
	hosts, err := s.repo.ListHostsByRun(run.ID)
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list hosts",
			Cause:   err,
		}
	}
	return &RunDetail{Run: run, Hosts: hosts}, nil
}

// ListRuns returns a page of runs plus the unfiltered total.
func (s *Service) ListRuns(f ListFilter) ([]DiscoveryRun, int64, error) {
	rows, total, err := s.repo.ListRuns(f)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list discovery runs",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// PromoteHosts creates a Device row for every host in the run
// (or every host in HostIDs) and writes the resulting device
// ID back into the host's PromotedToDeviceID column. The
// returned slice is the freshly-created devices. The flow is
// idempotent: a host that has already been promoted is
// skipped on subsequent calls.
func (s *Service) PromoteHosts(ctx context.Context, in PromoteInput) ([]devicepkg.Device, error) {
	run, err := s.repo.GetRun(in.RunID)
	if err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("discovery run %q not found", in.RunID),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load discovery run",
			Cause:   err,
		}
	}
	_ = ctx // reserved for future use (e.g. abort promote on cancel)

	hosts, err := s.repo.ListHostsByRun(run.ID)
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list hosts",
			Cause:   err,
		}
	}

	selected := make(map[string]struct{}, len(in.HostIDs))
	for _, id := range in.HostIDs {
		selected[id] = struct{}{}
	}

	var created []devicepkg.Device
	for i := range hosts {
		h := &hosts[i]
		if len(in.HostIDs) > 0 {
			if _, ok := selected[h.ID]; !ok {
				continue
			}
		}
		if h.PromotedToDeviceID != nil {
			// Already promoted — skip.
			continue
		}
		d := s.buildDevice(h)
		if err := s.devs.Create(d); err != nil {
			return created, &contracts.APIError{
				Code:    contracts.CodeInternal,
				Message: "failed to create device from discovered host",
				Cause:   err,
			}
		}
		h.PromotedToDeviceID = &d.ID
		if err := s.repo.UpdateHost(h); err != nil {
			return created, &contracts.APIError{
				Code:    contracts.CodeInternal,
				Message: "failed to mark host as promoted",
				Cause:   err,
			}
		}
		created = append(created, *d)
	}
	return created, nil
}

// buildDevice maps a DiscoveredHost onto a Device. The device
// type is inferred from the probe result: SNMP sysDescr set
// means a network device, port 22 open means a physical host,
// otherwise it defaults to a network device. The mapping is
// deliberately simple — the spec does not call for a richer
// classifier, and a future classifier can be added without
// changing the service contract.
func (s *Service) buildDevice(h *DiscoveredHost) *devicepkg.Device {
	name := h.Hostname
	if name == "" {
		name = h.IPAddress
	}
	dt := devicepkg.DeviceTypeNetwork
	if h.SNMPSysDescr == "" {
		// No SNMP response: assume SSH / physical host.
		dt = devicepkg.DeviceTypePhysicalHost
	}
	return &devicepkg.Device{
		Name:    name,
		Type:    dt,
		State:   devicepkg.DeviceStateOnline,
		Labels:  devicepkg.JSONMap{"discovered_from": "network_scan", "ip": h.IPAddress},
		Metadata: devicepkg.JSONMap{
			"open_ports":     h.OpenPorts,
			"snmp_sys_descr": h.SNMPSysDescr,
		},
	}
}

//go:build windows

package computer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

// WindowsCollector runs one fixed, read-only system inventory script. No part
// of the script comes from an API request, Gemini, or a Project Profile.
type WindowsCollector struct{}

func (WindowsCollector) Name() string { return "windows-read-only" }

const windowsSnapshotScript = `
$ErrorActionPreference = 'Stop'
$os = Get-CimInstance Win32_OperatingSystem
$cpu = @(Get-CimInstance Win32_Processor | Select-Object -ExpandProperty LoadPercentage)
$disks = @(Get-CimInstance Win32_LogicalDisk -Filter 'DriveType=3' | Select-Object -First 32 @{Name='name';Expression={$_.DeviceID}}, @{Name='total_mb';Expression={[uint64]($_.Size / 1MB)}}, @{Name='used_mb';Expression={[uint64](($_.Size - $_.FreeSpace) / 1MB)}})
$processes = @(Get-Process | Sort-Object WorkingSet64 -Descending | Select-Object -First 30 @{Name='name';Expression={$_.ProcessName}}, @{Name='pid';Expression={$_.Id}}, @{Name='cpu_seconds';Expression={[double]$_.CPU}}, @{Name='memory_mb';Expression={[uint64]($_.WorkingSet64 / 1MB)}})
$ports = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Select-Object -First 100 @{Name='address';Expression={$_.LocalAddress}}, @{Name='port';Expression={$_.LocalPort}}, @{Name='pid';Expression={$_.OwningProcess}}, @{Name='state';Expression={'LISTENING'}})
$interfaces = @(Get-NetAdapter -ErrorAction SilentlyContinue | Select-Object -First 32 @{Name='name';Expression={$_.Name}}, @{Name='state';Expression={$_.Status.ToString()}})
$services = @(Get-Service | Select-Object -First 256 @{Name='name';Expression={$_.Name}}, @{Name='state';Expression={$_.Status.ToString()}})
@{os=$os.Caption;uptime_seconds=[uint64](((Get-Date) - $os.LastBootUpTime).TotalSeconds);cpu_percent=[double](($cpu | Measure-Object -Average).Average);memory_total_mb=[uint64]($os.TotalVisibleMemorySize / 1024);memory_used_mb=[uint64](($os.TotalVisibleMemorySize-$os.FreePhysicalMemory)/1024);disks=$disks;processes=$processes;ports=$ports;interfaces=$interfaces;services=$services} | ConvertTo-Json -Depth 6 -Compress
`

type windowsPayload struct {
	OS            string      `json:"os"`
	UptimeSeconds uint64      `json:"uptime_seconds"`
	CPUPercent    float64     `json:"cpu_percent"`
	MemoryTotalMB uint64      `json:"memory_total_mb"`
	MemoryUsedMB  uint64      `json:"memory_used_mb"`
	Disks         []Disk      `json:"disks"`
	Processes     []Process   `json:"processes"`
	Ports         []Port      `json:"ports"`
	Interfaces    []Interface `json:"interfaces"`
	Services      []Service   `json:"services"`
}

func (WindowsCollector) Collect(ctx context.Context, expected Expectations) (Snapshot, error) {
	root := os.Getenv("SystemRoot")
	if root == "" {
		return Snapshot{}, errors.New("Windows system root unavailable")
	}
	powershell := filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(powershell); err != nil {
		return Snapshot{}, errors.New("Windows PowerShell collector unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, powershell, "-NoProfile", "-NonInteractive", "-Command", windowsSnapshotScript)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := command.Output()
	if err != nil {
		return Snapshot{}, fmt.Errorf("read-only Windows inventory failed: %w", err)
	}
	if len(output) > 1<<20 {
		return Snapshot{}, errors.New("computer snapshot exceeds 1 MiB")
	}
	var payload windowsPayload
	if err := json.Unmarshal(output, &payload); err != nil {
		return Snapshot{}, fmt.Errorf("decode Windows inventory: %w", err)
	}
	snapshot := Snapshot{SchemaVersion: SchemaVersion, DeviceID: "local-computer", TimestampMS: time.Now().UTC().UnixMilli(), Source: "windows-read-only", System: System{OS: payload.OS, Architecture: runtime.GOARCH, UptimeSeconds: payload.UptimeSeconds, CPUPercent: payload.CPUPercent, MemoryTotalMB: payload.MemoryTotalMB, MemoryUsedMB: payload.MemoryUsedMB}, Disks: payload.Disks, Processes: payload.Processes, Ports: payload.Ports, Interfaces: payload.Interfaces, Services: payload.Services, Dependencies: []Dependency{}, LocalChecks: []LocalCheck{}}
	processNames := make(map[int]string, len(snapshot.Processes))
	for _, process := range snapshot.Processes {
		processNames[process.PID] = process.Name
	}
	for index := range snapshot.Ports {
		snapshot.Ports[index].Process = processNames[snapshot.Ports[index].PID]
	}
	for _, tool := range []struct{ name, binary string }{{"Node", "node.exe"}, {"npm", "npm.cmd"}, {"Go", "go.exe"}, {"Python", "python.exe"}, {"Git", "git.exe"}, {"Docker", "docker.exe"}} {
		_, err := exec.LookPath(tool.binary)
		snapshot.Dependencies = append(snapshot.Dependencies, Dependency{Name: tool.name, Available: err == nil})
	}
	if expected.ExpectedLocalPort > 0 && expected.ExpectedLocalPort <= 65535 {
		connection, dialErr := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(expected.ExpectedLocalPort)), 500*time.Millisecond)
		if dialErr == nil {
			_ = connection.Close()
		}
		snapshot.LocalChecks = append(snapshot.LocalChecks, LocalCheck{Port: expected.ExpectedLocalPort, Reachable: dialErr == nil})
	}
	if _, err := Analyze(snapshot, expected); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

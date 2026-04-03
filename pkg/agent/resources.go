package agent

import (
	"math"

	v1 "github.com/mujhtech/dagryn/pkg/cluster/v1"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

func collectResourceSnapshot(workDir string) *v1.ResourceSnapshot {
	snapshot := &v1.ResourceSnapshot{}

	if vm, err := mem.VirtualMemory(); err == nil {
		snapshot.MemoryBytesAvailable = int64(vm.Available)
		snapshot.MemoryUsagePercent = vm.UsedPercent
	}

	if usage, err := disk.Usage(workDir); err == nil {
		snapshot.DiskBytesAvailable = int64(usage.Free)
	}

	if percents, err := cpu.Percent(0, false); err == nil && len(percents) > 0 {
		cpuUsage := percents[0]
		snapshot.CpuUsagePercent = cpuUsage

		cores, coreErr := cpu.Counts(true)
		if coreErr == nil && cores > 0 {
			snapshot.CpuMillicoresAvailable = estimateMillicoresAvailable(cores, cpuUsage)
		}
	}

	return snapshot
}

func estimateMillicoresAvailable(cores int, cpuUsagePercent float64) int64 {
	if cores <= 0 {
		return 0
	}
	if cpuUsagePercent < 0 {
		cpuUsagePercent = 0
	}
	if cpuUsagePercent > 100 {
		cpuUsagePercent = 100
	}
	total := float64(cores * 1000)
	available := total * (1 - cpuUsagePercent/100)
	if available < 0 {
		return 0
	}
	return int64(math.Round(available))
}

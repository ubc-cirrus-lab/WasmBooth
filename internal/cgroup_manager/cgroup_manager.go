package cgroup_manager

import (
	"bufio"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"webserver/internal/config"
)

type CgroupManager struct {
	Config *config.CgroupManagerConfig
}

func (cm *CgroupManager) Init() {
	cm.AddCPUtoSubtreeControl()
}

func (cm *CgroupManager) CreateCgroup(cgroupName string) {
	start := time.Now()

	cgroupPath := cm.GetThreadCgroupPath(cgroupName)

	err := os.MkdirAll(cgroupPath, 0777)
	if err != nil {
		slog.Error("Failed to create cgroup", "name", cgroupName)
	}

	slog.Debug("Created the cgroup", "cgroupPath", cgroupPath, "time", time.Since(start))
}

func (cm *CgroupManager) ChangeCgroupToThreaded(cgroupName string) {
	start := time.Now()
	formatedPodCgroup := "kubepods-burstable-pod" + strings.ReplaceAll(cm.Config.PodUID, "-", "_") + ".slice"
	cgroupTypeFilePath := filepath.Join("/data/kubepods.slice/kubepods-burstable.slice", formatedPodCgroup, cm.Config.ContainerID, cgroupName, "cgroup.type")
	text := "threaded"

	err := os.WriteFile(cgroupTypeFilePath, []byte(text), 0644)
	if err != nil {
		slog.Error("Failed to write text to file", "reason", err.Error())
	}

	slog.Debug("Changed cgroup to threaded", "cgroupTypeFilePath", cgroupTypeFilePath, "time", time.Since(start))
}

func (cm *CgroupManager) AddCPUtoSubtreeControl() {
	start := time.Now()
	formatedPodCgroup := "kubepods-burstable-pod" + strings.ReplaceAll(cm.Config.PodUID, "-", "_") + ".slice"
	criSubtreeControlFilePath := filepath.Join("/data/kubepods.slice/kubepods-burstable.slice", formatedPodCgroup, cm.Config.ContainerID, "cgroup.subtree_control")
	text := "+cpu"

	err := os.WriteFile(criSubtreeControlFilePath, []byte(text), 0644)
	if err != nil {
		slog.Error("Failed to write text to file", "reason", err.Error())
	}

	slog.Debug("Added CPU to cri subtree_control", "cgroupPath", criSubtreeControlFilePath, "time", time.Since(start))
}

func (cm *CgroupManager) GetThreadCgroupPath(cgroupName string) string {
	formatedPodCgroup := "kubepods-burstable-pod" + strings.ReplaceAll(cm.Config.PodUID, "-", "_") + ".slice"
	return filepath.Join("/data/kubepods.slice/kubepods-burstable.slice", formatedPodCgroup, cm.Config.ContainerID, cgroupName)
}

func (cm *CgroupManager) GetContainerCgroupPath() string {
	formatedPodCgroup := "kubepods-burstable-pod" + strings.ReplaceAll(cm.Config.PodUID, "-", "_") + ".slice"
	return filepath.Join("/data/kubepods.slice/kubepods-burstable.slice", formatedPodCgroup, cm.Config.ContainerID)
}

func (cm *CgroupManager) SetCgroupLimits(cgroupName, cfsQuotaUs string) {
	start := time.Now()
	formatedPodCgroup := "kubepods-burstable-pod" + strings.ReplaceAll(cm.Config.PodUID, "-", "_") + ".slice"
	cgroupCpuFilePath := filepath.Join("/data/kubepods.slice/kubepods-burstable.slice", formatedPodCgroup, cm.Config.ContainerID, cgroupName, "cpu.max")

	cpuQuotaInt, err := strconv.Atoi(cfsQuotaUs)
	if err != nil {
		slog.Error("Cfs Quota is not a valid number", "reason", err.Error())
	}

	text := strconv.Itoa(cpuQuotaInt*100) + " 100000"

	err = os.WriteFile(cgroupCpuFilePath, []byte(text), 0644)
	if err != nil {
		slog.Error("Failed to write text to file", "reason", err.Error())
	}

	slog.Debug("Applied resource limits", "cgroupCpuFilePath", cgroupCpuFilePath, "time", time.Since(start), "cpu_limit", cfsQuotaUs)
}

func (cm *CgroupManager) DeleteCgroup(cgroupName string) {
	start := time.Now()
	cgroupNameFormatted := "memory,cpu:" + cgroupName

	args := []string{"cgdelete", "-r", cgroupNameFormatted}
	command := exec.Command("sudo", args...)
	_, err := command.Output()
	if err != nil {
		slog.Error("Failed to delete the cgroup", "reason", err.Error())
	}

	end := time.Now()
	slog.Debug("Deleted the cgroup", "cgroupName", cgroupName, "time", end.Sub(start))
}

func (cm *CgroupManager) Acquire(id, cfsQuota, memoryLimit string) {
	cm.CreateCgroup(id)
	cm.ChangeCgroupToThreaded(id)
	cm.SetCgroupLimits(id, cfsQuota)
}

func (cm *CgroupManager) Assign(cgroupName, tid string) {
	start := time.Now()

	cgroupPath := cm.GetThreadCgroupPath(cgroupName)
	threadsFilePath := filepath.Join(cgroupPath, "cgroup.threads")

	file, err := os.OpenFile(threadsFilePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Error("Failed to open tasks file", "reason", err.Error())
	}
	defer file.Close()

	_, err = file.WriteString(tid + "\n")
	if err != nil {
		slog.Error("Failed to write TID to threads file", "reason", err.Error())
	}

	slog.Debug("Assigned the cgroup", "threads_file_path", threadsFilePath, "tid", tid, "time", time.Since(start))
}

func (cm *CgroupManager) Release(cgroupName string) {
	start := time.Now()

	// Move the thread to container cgroup
	threadCgroupThreadsPath := filepath.Join(cm.GetThreadCgroupPath(cgroupName), "cgroup.threads")
	containerCgroupThreadsPath := filepath.Join(cm.GetContainerCgroupPath(), "cgroup.threads")

	threadCgroupFile, err := os.Open(threadCgroupThreadsPath)
	if err != nil {
		slog.Error("Failed to open thread cgroup", "reason", err)
	}
	defer threadCgroupFile.Close()

	containerCgroupFile, err := os.OpenFile(containerCgroupThreadsPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("Failed to open container cgroup", "reason", err)
		return
	}
	defer containerCgroupFile.Close()

	scanner := bufio.NewScanner(threadCgroupFile)

	for scanner.Scan() {
		line := scanner.Text()

		_, err = containerCgroupFile.WriteString(line + "\n")
		if err != nil {
			slog.Error("Error while writing to container file", "reason", err)
			return
		}
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		slog.Error("Error while writing to container file", "reason", err)
		return
	}

	// Remove the thread cgroup
	cgroupPath := cm.GetThreadCgroupPath(cgroupName)
	err = os.RemoveAll(cgroupPath)
	if err != nil {
		slog.Error("Failed to delete the cgroup file", "reason", err.Error(), "time", time.Since(start))
	} else {
		slog.Debug("Released the cgroup", "cgroupPath", cgroupPath, "time", time.Since(start))
	}
}

func (cm *CgroupManager) GetCurrentCPUUsage() float64 {
	return cm.GetPhysMemoryUsage() + cm.GetSwapMemoryUsage()
}

func (cm *CgroupManager) GetCurrentMemoryUsage() float64 {
	return cm.GetPhysMemoryUsage() + cm.GetSwapMemoryUsage()
}

func (cm *CgroupManager) GetCPUUsage() uint64 {
	cpuStatsPath := filepath.Join(cm.GetContainerCgroupPath(), "cpu.stat")

	// Open the cpu.stats file
	file, err := os.Open(cpuStatsPath)
	if err != nil {
		slog.Error("Error opening cpu.stats file", "reason", err)
		return 0.0
	}
	defer file.Close()

	// Read the first line of the file
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()
		// Assuming the first line is in the format "some_metric value"
		parts := strings.Fields(line)
		if len(parts) == 2 {
			usage, err := strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				slog.Error("Error parsing CPU usage", "reason", err)
				return 0.0
			}
			return usage
		}
		slog.Error("Unexpected format in cpu.stats")
	}
	if err := scanner.Err(); err != nil {
		slog.Error("Error reading cpu.stats", "reason", err)
	}

	return 0.0
}
func (cm *CgroupManager) GetPhysMemoryUsage() float64 {
	memoryCurrentPath := filepath.Join(cm.GetContainerCgroupPath(), "memory.current")
	var memoryUsage int64
	file, err := os.Open(memoryCurrentPath)
	if err != nil {
		slog.Error("Failed to open memory.current file", "reason", err.Error())
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()

		memoryUsage, err = strconv.ParseInt(line, 10, 64)
		if err != nil {
			slog.Error("Failed to parse memory.current file", "reason", err.Error())
		}
	} else if err := scanner.Err(); err != nil {
		slog.Error("Failed to read memory.current file", "reason", err.Error())
	}

	// Convert to Megabytes
	return float64(memoryUsage) / (1024 * 1024)
}

func (cm *CgroupManager) GetSwapMemoryUsage() float64 {
	memorySwapCurrentPath := filepath.Join(cm.GetContainerCgroupPath(), "memory.swap.current")
	var memoryUsage int64
	file, err := os.Open(memorySwapCurrentPath)
	if err != nil {
		slog.Error("Failed to open memory.swap.current file", "reason", err.Error())
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()

		memoryUsage, err = strconv.ParseInt(line, 10, 64)
		if err != nil {
			slog.Error("Failed to parse memory.swap.current file", "reason", err.Error())
		}
	} else if err := scanner.Err(); err != nil {
		slog.Error("Failed to read memory.swap.current file", "reason", err.Error())
	}

	// Convert to Megabytes
	return float64(memoryUsage) / (1024 * 1024)
}

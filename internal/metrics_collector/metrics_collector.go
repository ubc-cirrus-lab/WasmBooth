package metrics_collector

import (
	"container/list"
	"strconv"
	"time"
	"webserver/internal/cgroup_manager"
	"webserver/internal/config"
)

type MetricsCollector struct {
	Config               *config.MetricsCollector
	CPUUsageWindow       *list.List
	CPUUtilizationWindow *list.List
	TimestampWindow      *list.List
	CgroupManager        *cgroup_manager.CgroupManager
}

func (mc *MetricsCollector) Init() {
	mc.CPUUsageWindow = list.New()
	mc.CPUUtilizationWindow = list.New()
	mc.TimestampWindow = list.New()
}

func (mc *MetricsCollector) Update() {
	currTime := time.Now().UnixMicro()
	usageUsec := mc.CgroupManager.GetCPUUsage()

	var prevUsageUsec uint64
	if mc.CPUUsageWindow.Len() == 0 {
		prevUsageUsec = 0
	} else {
		prevUsageUsec = mc.CPUUsageWindow.Back().Value.(uint64)
	}

	var prevTime int64
	if mc.TimestampWindow.Len() == 0 {
		prevTime = 0
	} else {
		prevTime = mc.TimestampWindow.Back().Value.(int64)
	}

	cpuUtilization := float64(usageUsec-prevUsageUsec) / float64(currTime-prevTime)

	mc.CPUUtilizationWindow.PushBack(cpuUtilization)
	mc.CPUUsageWindow.PushBack(usageUsec)
	mc.TimestampWindow.PushBack(currTime)

	// slog.Debug("Got the new usage_usec", "usageUsec", usageUsec, "prevUsageUsec", prevUsageUsec, "currTime", currTime, "prevTime", prevTime, "cpuUtilization", cpuUtilization)

	if mc.CPUUtilizationWindow.Len() > mc.Config.MetricsCollectionWindow {
		mc.CPUUtilizationWindow.Remove(mc.CPUUtilizationWindow.Front())
	}
}

func (mc *MetricsCollector) GetAverageCPUUtilization() float64 {
	if mc.CPUUtilizationWindow.Len() == 0 {
		return 0.0
	}

	usageWindow := ""
	sum := 0.0
	for e := mc.CPUUtilizationWindow.Front(); e != nil; e = e.Next() {
		sum += e.Value.(float64)
		usageWindow += strconv.FormatFloat(e.Value.(float64), 'f', 6, 64) + " "
	}

	return sum / float64(mc.CPUUtilizationWindow.Len())
}

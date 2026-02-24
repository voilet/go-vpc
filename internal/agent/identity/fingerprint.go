package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
)

// GenerateFingerprint 生成设备指纹
// 基于主机名、操作系统、架构和网卡 MAC 地址
func GenerateFingerprint() (string, error) {
	var parts []string

	// 主机名
	hostname, err := os.Hostname()
	if err == nil {
		parts = append(parts, "hostname:"+hostname)
	}

	// 操作系统和架构
	parts = append(parts, "os:"+runtime.GOOS)
	parts = append(parts, "arch:"+runtime.GOARCH)

	// 网卡 MAC 地址
	macs, err := getMACAddresses()
	if err == nil && len(macs) > 0 {
		// 排序以确保稳定性
		sort.Strings(macs)
		parts = append(parts, "macs:"+strings.Join(macs, ","))
	}

	// 如果没有收集到任何信息，返回错误
	if len(parts) == 0 {
		return "", fmt.Errorf("无法收集设备信息")
	}

	// 计算 SHA256 哈希
	data := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:]), nil
}

// getMACAddresses 获取所有物理网卡的 MAC 地址
func getMACAddresses() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var macs []string
	for _, iface := range interfaces {
		// 跳过回环接口和无 MAC 的接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		// 跳过虚拟接口（通常以特定前缀开头）
		mac := iface.HardwareAddr.String()
		if strings.HasPrefix(mac, "00:00:00") {
			continue
		}
		macs = append(macs, mac)
	}

	return macs, nil
}

//go:build !linux

package route

import (
	"fmt"
	"net"
	"runtime"
)

type stubManager struct{}

func newPlatformManager() Manager {
	return &stubManager{}
}

func (m *stubManager) AddRoute(dst *net.IPNet, gw net.IP, ifaceName string) error {
	return fmt.Errorf("路由管理在 %s 平台上未实现", runtime.GOOS)
}

func (m *stubManager) RemoveRoute(dst *net.IPNet) error {
	return fmt.Errorf("路由管理在 %s 平台上未实现", runtime.GOOS)
}

func (m *stubManager) GetRoutes(ifaceName string) ([]Route, error) {
	return nil, fmt.Errorf("路由管理在 %s 平台上未实现", runtime.GOOS)
}

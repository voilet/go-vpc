package route

import "net"

// Manager 定义路由管理接口
type Manager interface {
	// AddRoute 添加路由
	AddRoute(dst *net.IPNet, gw net.IP, ifaceName string) error

	// RemoveRoute 删除路由
	RemoveRoute(dst *net.IPNet) error

	// GetRoutes 获取指定接口的路由
	GetRoutes(ifaceName string) ([]Route, error)
}

// Route 表示一条路由
type Route struct {
	Dst       *net.IPNet // 目标网络
	Gw        net.IP     // 网关
	IfaceName string     // 出接口名称
}

// New 创建路由管理器
func New() Manager {
	return newPlatformManager()
}

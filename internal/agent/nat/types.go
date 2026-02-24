package nat

import "fmt"

// NATType 表示 NAT 类型
type NATType int

const (
	NATTypeUnknown        NATType = iota // 未知
	NATTypeNone                          // 公网直连，无 NAT
	NATTypeFullCone                      // 完全锥形 NAT
	NATTypeRestricted                    // 受限锥形 NAT
	NATTypePortRestricted                // 端口受限锥形 NAT
	NATTypeSymmetric                     // 对称型 NAT
)

// String 返回 NAT 类型的字符串表示
func (t NATType) String() string {
	switch t {
	case NATTypeNone:
		return "None"
	case NATTypeFullCone:
		return "FullCone"
	case NATTypeRestricted:
		return "Restricted"
	case NATTypePortRestricted:
		return "PortRestricted"
	case NATTypeSymmetric:
		return "Symmetric"
	default:
		return "Unknown"
	}
}

// CanPunchThrough 判断两个 NAT 类型是否可能打洞成功
func CanPunchThrough(a, b NATType) bool {
	// 公网或完全锥形可以与任何类型打洞
	if a == NATTypeNone || a == NATTypeFullCone {
		return true
	}
	if b == NATTypeNone || b == NATTypeFullCone {
		return true
	}
	// 两个对称型无法打洞
	if a == NATTypeSymmetric && b == NATTypeSymmetric {
		return false
	}
	// 受限锥形之间可以打洞
	if (a == NATTypeRestricted || a == NATTypePortRestricted) &&
		(b == NATTypeRestricted || b == NATTypePortRestricted) {
		return true
	}
	// 对称型与锥形尝试打洞（可能成功）
	return true
}

// Result 表示 NAT 探测结果
type Result struct {
	PublicAddr string  // 公网地址 IP:Port
	NATType    NATType // NAT 类型
	LocalAddr  string  // 本地地址
}

func (r *Result) String() string {
	return fmt.Sprintf("NAT{type=%s, public=%s, local=%s}", r.NATType, r.PublicAddr, r.LocalAddr)
}

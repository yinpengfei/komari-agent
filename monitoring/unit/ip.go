package monitoring

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"time"

	"github.com/komari-monitor/komari-agent/dnsresolver"
)

var (
	// 创建适用于IPv4和IPv6的HTTP客户端
	ipv4HTTPClient = &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := dnsresolver.GetNetDialer(15 * time.Second)
				return dialer.DialContext(ctx, "tcp4", addr) // 锁v4防止出现问题
			},
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: 15 * time.Second,
	}
	ipv6HTTPClient = &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := dnsresolver.GetNetDialer(15 * time.Second)
				return dialer.DialContext(ctx, "tcp6", addr) // 锁v6防止出现问题
			},
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: 15 * time.Second,
	}
	userAgent              = "curl/8.0.1"
	listIPInterfaces       = net.Interfaces
	listLegacyIPInterfaces = InterfaceList
	interfaceIPAddrs       = func(iface net.Interface) ([]net.Addr, error) { return iface.Addrs() }
)

func GetIPv4Address() (string, error) {
	return queryPublicIP(ipv4HTTPClient, "https://api.ipify.org?format=json", true)
}

func GetIPv6Address() (string, error) {
	return queryPublicIP(ipv6HTTPClient, "https://api6.ipify.org?format=json", false)
}

func queryPublicIP(client *http.Client, url string, ipv4 bool) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	var result struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result); err != nil {
		return "", err
	}
	ip, err := netip.ParseAddr(result.IP)
	if err != nil || ip.Zone() != "" || ip.Is4In6() || ip.Is4() != ipv4 || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return "", nil
	}
	return ip.String(), nil
}

func GetIPAddress() (ipv4, ipv6 string, err error) {
	ipv4, ipv6 = flags.CustomIpv4, flags.CustomIpv6
	if ipv4 != "" && ipv6 != "" {
		return ipv4, ipv6, nil
	}
	if flags.GetIpAddrFromNic {
		allowNics, err := listLegacyIPInterfaces()
		if err != nil {
			log.Printf("Get Interface List Error: %v", err)
		} else {
			nic4, nic6 := getIPFromInterfaces(allowNics)
			if ipv4 == "" {
				ipv4 = nic4
			}
			if ipv6 == "" {
				ipv6 = nic6
			}
			if ipv4 != "" || ipv6 != "" {
				log.Printf("Get IP from NIC - IPv4: %s, IPv6: %s", ipv4, ipv6)
				return ipv4, ipv6, nil
			}
		}
	} else if ipv6 == "" {
		ipv6 = getPublicIPv6FromInterfaces()
	}

	if !flags.AllowExternalIPLookup {
		return ipv4, ipv6, nil
	}

	if ipv4 == "" {
		ipv4, err = GetIPv4Address()
		if err != nil {
			log.Printf("Get IPV4 Error: %v", err)
			ipv4 = ""
		}
	}
	if ipv6 == "" {
		ipv6, err = GetIPv6Address()
		if err != nil {
			log.Printf("Get IPV6 Error: %v", err)
			ipv6 = ""
		}
	}

	return ipv4, ipv6, nil
}

// Select a stable public IPv6 candidate without sending network traffic.
// This checks address scope, not whether the address is reachable from the Internet.
func getPublicIPv6FromInterfaces() string {
	interfaces, err := listIPInterfaces()
	if err != nil {
		log.Printf("Get Interface List Error: %v", err)
		return ""
	}
	sort.Slice(interfaces, func(i, j int) bool { return interfaces[i].Name < interfaces[j].Name })
	include, exclude := parseNics(flags.IncludeNics), parseNics(flags.ExcludeNics)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || !shouldInclude(iface.Name, include, exclude) {
			continue
		}
		addrs, err := interfaceIPAddrs(iface)
		if err != nil {
			continue
		}
		candidates := []netip.Addr{}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			parsed, ok := netip.AddrFromSlice(ip)
			if ok && isPublicIPv6(parsed) {
				candidates = append(candidates, parsed)
			}
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].Less(candidates[j]) })
		if len(candidates) > 0 {
			return candidates[0].String()
		}
	}
	return ""
}

var publicIPv6Prefix = netip.MustParsePrefix("2000::/3")
var nonPublicIPv6Prefixes = []netip.Prefix{
	// IANA special-purpose protocol, documentation and transition ranges.
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func isPublicIPv6(ip netip.Addr) bool {
	if !ip.Is6() || ip.Is4In6() || !publicIPv6Prefix.Contains(ip) {
		return false
	}
	for _, prefix := range nonPublicIPv6Prefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

// getIPFromInterfaces 从指定的网卡接口获取 IPv4 和 IPv6 地址
func getIPFromInterfaces(nicNames []string) (ipv4, ipv6 string) {
	interfaces, err := listIPInterfaces()
	if err != nil {
		log.Printf("Failed to get network interfaces: %v", err)
		return "", ""
	}
	for _, iface := range interfaces {
		// 检查接口是否在允许列表中
		if !func(slice []string, item string) bool {
			for _, s := range slice {
				if s == item {
					return true
				}
			}
			return false
		}(nicNames, iface.Name) {
			continue
		}

		// 跳过未启动的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := interfaceIPAddrs(iface)
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// 获取 IPv4 地址
			if ipv4 == "" && ip.To4() != nil {
				ipv4 = ip.String()
			}

			// 获取 IPv6 地址（排除链路本地地址）
			if ipv6 == "" && ip.To4() == nil && !ip.IsLinkLocalUnicast() {
				ipv6 = ip.String()
			}

			// 如果已经找到 IPv4 和 IPv6,提前返回
			if ipv4 != "" && ipv6 != "" {
				return ipv4, ipv6
			}
		}
	}

	return ipv4, ipv6
}

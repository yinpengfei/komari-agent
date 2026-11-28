package monitoring

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"sync"
	"time"

	"github.com/komari-monitor/komari-agent/dnsresolver"
)

var (
	// 创建适用于IPv4和IPv6的HTTP客户端
	ipv4HTTPClient = &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &http.Transport{
			// Source-IP discovery must not report an HTTP_PROXY egress.
			Proxy: nil,
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
			Proxy: nil,
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
	listIPInterfaces       = net.Interfaces
	listLegacyIPInterfaces = InterfaceList
	interfaceIPAddrs       = func(iface net.Interface) ([]net.Addr, error) { return iface.Addrs() }
)

func GetIPv4Address() (string, error) {
	endpoint := flags.IPv4LookupURL
	if endpoint == "" {
		endpoint = DefaultIPv4LookupURL
	}
	return queryPublicIPWithFallback(ipv4HTTPClient, endpoint, true, !flags.DisableThirdPartyIPFallback)
}

func GetIPv6Address() (string, error) {
	endpoint := flags.IPv6LookupURL
	if endpoint == "" {
		endpoint = DefaultIPv6LookupURL
	}
	return queryPublicIPWithFallback(ipv6HTTPClient, endpoint, false, !flags.DisableThirdPartyIPFallback)
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
	var wg sync.WaitGroup
	if ipv4 == "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var e error
			ipv4, e = GetIPv4Address()
			if e != nil {
				log.Printf("Get IPv4 from configured service failed: %v", e)
			}
		}()
	}
	if ipv6 == "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var e error
			ipv6, e = GetIPv6Address()
			if e != nil {
				log.Printf("Get IPv6 from configured service failed: %v", e)
			}
		}()
	}
	wg.Wait()

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

package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// List of ECS subnets to test (Global and targeted ranges)
var subnets = map[string]string{
	"HK-PCCW":         "218.102.0.0/24",
	"HK-HGC":          "210.3.0.0/24",
	"JP-Linode":       "139.162.64.0/24",
	"JP-Direct":       "163.44.0.0/24",
	"SG-DigitalOcean": "128.199.0.0/24",
	"TW-HiNet":        "118.160.0.0/24",
	"KR-KT":           "121.130.0.0/24",
	"US-Google":       "8.8.8.8/32",
	"TH-Bangkok":      "58.8.0.0/24",
	"VN-Hanoi":        "113.160.0.0/24",
	"MY-Kuala":        "115.132.0.0/24",
	"IN-Mumbai":       "103.21.141.0/24",
	"RU-Moscow":       "95.161.224.0/24",
	"AE-Dubai":        "94.200.0.0/24",
	"CM-Guangdong":    "2408:8256:d085::/48",
	"CU-Beijing":      "2404:c800::/32",
	"CT-Shanghai":     "240e:e1:8000::/33",
	"GGC-Test-1":      "142.250.0.0/24",
	"GGC-Test-2":      "172.217.0.0/24",
	"GGC-Test-3":      "216.58.200.0/24",
}

const (
	DNS_SERVER  = "https://dns.google/dns-query"
	TIMEOUT     = 5 * time.Second
	CONCURRENCY = 15
)

var testHosts = []string{"www.youtube.com", "www.google.com", "googlevideo.com"}

type Result struct {
	Region  string
	Host    string
	Subnet  string
	IP      string
	Type    string
	Success bool
	Latency time.Duration
	Error   error
}

func main() {
	results := make(chan Result, len(subnets)*len(testHosts)*2)
	var wg sync.WaitGroup
	sem := make(chan struct{}, CONCURRENCY)

	fmt.Printf("Searching for mysterious GGC nodes...\n")
	fmt.Printf("%-15s | %-5s | %-20s | %-30s | %-10s | %s\n", "Region", "Type", "Host", "Resolved IP", "Latency", "Status")
	fmt.Println("------------------------------------------------------------------------------------------------------------------------")

	for _, host := range testHosts {
		for region, subnet := range subnets {
			// Test A
			wg.Add(1)
			go func(h, r, s string) {
				defer wg.Done()
				sem <- struct{}{}
				results <- benchmark(h, r, s, dns.TypeA)
				<-sem
			}(host, region, subnet)

			// Test AAAA
			wg.Add(1)
			go func(h, r, s string) {
				defer wg.Done()
				sem <- struct{}{}
				results <- benchmark(h, r, s, dns.TypeAAAA)
				<-sem
			}(host, region, subnet)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		status := "✅ OK"
		lat := res.Latency.String()
		if !res.Success {
			status = fmt.Sprintf("❌ FAIL (%v)", res.Error)
			lat = "N/A"
		}
		fmt.Printf("%-15s | %-5s | %-20s | %-30s | %-10s | %s\n", res.Region, res.Type, res.Host, res.IP, lat, status)
	}
}

func benchmark(host, region, subnetStr string, qType uint16) Result {
	res := Result{Region: region, Host: host, Subnet: subnetStr, Success: false}
	if qType == dns.TypeA {
		res.Type = "A"
	} else {
		res.Type = "AAAA"
	}

	ip, err := resolveWithECS(host, subnetStr, qType)
	if err != nil {
		res.Error = fmt.Errorf("DNS error: %v", err)
		return res
	}
	res.IP = ip

	start := time.Now()
	dialer := &net.Dialer{Timeout: TIMEOUT}

	// Test with original SNI
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(ip, "443"), &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true,
	})

	if err == nil {
		defer conn.Close()
		res.Latency = time.Since(start)
		res.Success = true
		return res
	}

	// If failed, check if it's SNI filtering by using a fake SNI
	fakeConn, fakeErr := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(ip, "443"), &tls.Config{
		ServerName:         "cloudflare.com",
		InsecureSkipVerify: true,
	})
	if fakeErr == nil {
		fakeConn.Close()
		res.Error = fmt.Errorf("SNI blocked (IP works with fake SNI)")
	} else {
		res.Error = err
	}

	return res
}

func resolveWithECS(host, subnetStr string, qType uint16) (string, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(host), qType)
	m.RecursionDesired = true

	_, ipNet, err := net.ParseCIDR(subnetStr)
	if err != nil {
		return "", err
	}
	ones, _ := ipNet.Mask.Size()

	e := &dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		SourceNetmask: uint8(ones),
		SourceScope:   0,
	}
	if ip4 := ipNet.IP.To4(); ip4 != nil {
		e.Family = 1
		e.Address = ip4
	} else {
		e.Family = 2
		e.Address = ipNet.IP
	}

	o := m.IsEdns0()
	if o == nil {
		m.SetEdns0(1232, false)
		o = m.IsEdns0()
	}
	o.Option = append(o.Option, e)

	client := &http.Client{Timeout: 5 * time.Second}
	wire, err := m.Pack()
	if err != nil {
		return "", err
	}

	resp, err := client.Post(DNS_SERVER, "application/dns-message", bytes.NewReader(wire))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	replyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	reply := new(dns.Msg)
	if err := reply.Unpack(replyData); err != nil {
		return "", err
	}

	for _, ans := range reply.Answer {
		if qType == dns.TypeA {
			if a, ok := ans.(*dns.A); ok {
				return a.A.String(), nil
			}
		} else {
			if aaaa, ok := ans.(*dns.AAAA); ok {
				return aaaa.AAAA.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no records")
}

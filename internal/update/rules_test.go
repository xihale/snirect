package update

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/xihale/snirect-shared/rules"
)

// TestJSONArrayToTOML verifies that the legacy JSON array format can be converted to TOML.
func TestJSONArrayToTOML(t *testing.T) {
	// Sample JSON array structure from Cealing-Host.json
	jsonData := `[
		[["cdn.jsdelivr.net"],"","104.16.89.20"],
		[["images.prismic.io"],"imgix.net","151.101.78.208"],
		[["*pixiv.net","*fanbox.cc"],"pixivision.net","210.140.139.155"]
	]`

	// Parse the JSON array
	var raw [][3]interface{}
	if err := json.Unmarshal([]byte(jsonData), &raw); err != nil {
		t.Fatalf("Failed to parse JSON array: %v", err)
	}

	// Convert to Rules struct (mimicking convertAndInstallRules logic)
	r := rules.NewRules()
	for _, entry := range raw {
		if len(entry) != 3 {
			continue
		}

		// Element 0: array of host patterns
		hosts, ok := entry[0].([]interface{})
		if !ok {
			t.Logf("Skipping entry: hosts not array: %T", entry[0])
			continue
		}

		// Element 1: SNI value (string or null)
		sniVal := entry[1]
		var sni string
		if sniVal != nil {
			if s, ok := sniVal.(string); ok {
				sni = s
			}
		}

		// Element 2: IP address (must be string)
		ip, ok := entry[2].(string)
		if !ok {
			t.Logf("Skipping entry: IP not string: %T", entry[2])
			continue
		}

		// Add to both Hosts and AlterHostname maps
		for _, h := range hosts {
			hostStr, ok := h.(string)
			if !ok {
				t.Logf("Skipping host: not string: %T", h)
				continue
			}
			r.Hosts[hostStr] = ip
			// Always set AlterHostname, even if empty string (means strip SNI)
			r.AlterHostname[hostStr] = sni
		}
	}

	r.Init()

	// Verify conversions
	// Check hosts
	expectedHosts := map[string]string{
		"cdn.jsdelivr.net":  "104.16.89.20",
		"images.prismic.io": "151.101.78.208",
		"*pixiv.net":        "210.140.139.155",
		"*fanbox.cc":        "210.140.139.155",
	}
	for host, expectedIP := range expectedHosts {
		ip, ok := r.Hosts[host]
		if !ok {
			t.Errorf("Host %s not found in Hosts map", host)
		} else if ip != expectedIP {
			t.Errorf("Host %s: expected IP %s, got %s", host, expectedIP, ip)
		}
	}
	for host, expectedIP := range expectedHosts {
		ip, ok := r.Hosts[host]
		if !ok {
			t.Errorf("Host %s not found in Hosts map", host)
		} else if ip != expectedIP {
			t.Errorf("Host %s: expected IP %s, got %s", host, expectedIP, ip)
		}
	}

	// Verify alter_hostname (empty string should still be set)
	if val, ok := r.AlterHostname["cdn.jsdelivr.net"]; !ok {
		t.Error("AlterHostname missing for cdn.jsdelivr.net")
	} else if val != "" {
		t.Errorf("AlterHostname[cdn.jsdelivr.net] expected empty string, got %s", val)
	}

	if val, ok := r.AlterHostname["images.prismic.io"]; !ok {
		t.Error("AlterHostname missing for images.prismic.io")
	} else if val != "imgix.net" {
		t.Errorf("AlterHostname[images.prismic.io] expected imgix.net, got %s", val)
	}

	if val, ok := r.AlterHostname["*pixiv.net"]; !ok {
		t.Error("AlterHostname missing for *pixiv.net")
	} else if val != "pixivision.net" {
		t.Errorf("AlterHostname[*pixiv.net] expected pixivision.net, got %s", val)
	}

	// Check that TOML conversion works
	tomlData, err := r.ToTOML()
	if err != nil {
		t.Fatalf("Failed to convert to TOML: %v", err)
	}

	// Write to temporary file for inspection (optional)
	tmpFile, err := os.CreateTemp("", "rules-*.toml")
	if err == nil {
		tmpFile.Write(tomlData)
		tmpFile.Close()
		os.Remove(tmpFile.Name()) // Clean up
	}

	t.Log("TOML conversion successful, length:", len(tomlData))
}

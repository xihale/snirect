package certpolicy

import "fmt"

// CertPolicy represents a certificate verification policy.
type CertPolicy struct {
	Enabled bool
	Strict  bool
	Allowed []string
}

// ParseCertPolicy converts a raw configuration value into a CertPolicy.
func ParseCertPolicy(data interface{}) (CertPolicy, error) {
	var p CertPolicy
	switch v := data.(type) {
	case bool:
		p.Enabled = v
	case string:
		switch v {
		case "strict":
			p.Enabled = true
			p.Strict = true
		case "false":
			p.Enabled = false
		case "true":
			p.Enabled = true
		default:
			p.Enabled = true
			p.Allowed = []string{v}
		}
	case []interface{}:
		p.Enabled = true
		for _, item := range v {
			if s, ok := item.(string); ok {
				p.Allowed = append(p.Allowed, s)
			}
		}
	case []string:
		p.Enabled = true
		p.Allowed = v
	case nil:
		// Default zero value
	default:
		return p, fmt.Errorf("invalid cert policy type: %T", data)
	}
	return p, nil
}

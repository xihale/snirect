package cmd

var (
	IsQuicEnabled    bool
	IsFirefoxEnabled bool = true // Integrated in base
)

func getModuleStatus() []struct {
	Name    string
	Enabled bool
} {
	return []struct {
		Name    string
		Enabled bool
	}{
		{"Standard (Core)", true},
		{"System Service", true},
		{"Firefox Certificate Tool", IsFirefoxEnabled},
		{"QUIC (DoQ/H3)", IsQuicEnabled},
	}
}

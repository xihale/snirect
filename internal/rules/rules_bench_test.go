package rules

import "testing"

func BenchmarkRules_PassthroughMiss(b *testing.B) {
	r, err := LoadRules()
	if err != nil {
		b.Fatal(err)
	}
	hosts := []string{
		"example.com",
		"api.unknown-service.example.org",
		"CDN.UNMATCHED.EXAMPLE.NET.",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := hosts[i%len(hosts)]
		_, _ = r.GetAlterHostname(h)
		_, _ = r.GetCertVerify(h)
		_, _ = r.GetHost(h)
	}
}

func BenchmarkRules_WildcardHit(b *testing.B) {
	r, err := LoadRules()
	if err != nil {
		b.Fatal(err)
	}

	b.Run("wildcard_clean", func(b *testing.B) {
		hosts := []string{
			"www.youtube.com",
			"gist.github.com",
			"www.twitch.tv",
			"r1---sn-4g5e6nez.googlevideo.com",
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			h := hosts[i%len(hosts)]
			_, _ = r.GetAlterHostname(h)
			_, _ = r.GetCertVerify(h)
			_, _ = r.GetHost(h)
			_ = r.GetIgnoreExpiry(h)
		}
	})

	b.Run("wildcard_fqdn_trailing_dot", func(b *testing.B) {
		hosts := []string{
			"www.youtube.com.",
			"gist.github.com.",
			"WWW.TWITCH.TV.",
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			h := hosts[i%len(hosts)]
			_, _ = r.GetAlterHostname(h)
			_, _ = r.GetCertVerify(h)
			_, _ = r.GetHost(h)
			_ = r.GetIgnoreExpiry(h)
		}
	})

	b.Run("exact_hit", func(b *testing.B) {
		hosts := []string{
			"upload.github.com",
			"i.ytimg.com",
			"store.steampowered.com",
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			h := hosts[i%len(hosts)]
			_, _ = r.GetAlterHostname(h)
			_, _ = r.GetCertVerify(h)
			_, _ = r.GetHost(h)
			_ = r.GetIgnoreExpiry(h)
		}
	})
}

package cert

import (
	"crypto/tls"
	"fmt"
	"sync"
	"testing"
)

func BenchmarkCertManager_GetCertificate_ColdParallel(b *testing.B) {
	tmpDir := b.TempDir()
	cm, err := NewCertificateManager(tmpDir+"/root.crt", tmpDir+"/root.key")
	if err != nil {
		b.Fatalf("NewCertificateManager: %v", err)
	}
	defer cm.Close()

	const workers = 8
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		host := fmt.Sprintf("cold-%d.example.com", i)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(workers)
		for w := 0; w < workers; w++ {
			go func() {
				defer wg.Done()
				<-start
				if _, err := cm.GetCertificate(&tls.ClientHelloInfo{ServerName: host}); err != nil {
					b.Errorf("GetCertificate(%s): %v", host, err)
				}
			}()
		}
		close(start)
		wg.Wait()
	}
}

func BenchmarkCertManager_GetCertificate_Warm(b *testing.B) {
	tmpDir := b.TempDir()
	cm, err := NewCertificateManager(tmpDir+"/root.crt", tmpDir+"/root.key")
	if err != nil {
		b.Fatalf("NewCertificateManager: %v", err)
	}
	defer cm.Close()

	hi := &tls.ClientHelloInfo{ServerName: "warm.example.com"}
	if _, err := cm.GetCertificate(hi); err != nil {
		b.Fatalf("warm setup: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		localHI := &tls.ClientHelloInfo{ServerName: "warm.example.com"}
		for pb.Next() {
			if _, err := cm.GetCertificate(localHI); err != nil {
				b.Errorf("GetCertificate: %v", err)
			}
		}
	})
}

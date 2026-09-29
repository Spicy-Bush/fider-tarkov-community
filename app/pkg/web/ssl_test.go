package web

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob/fs"

	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
)

func mockGetTenantWithCorrectSubdomains(ctx context.Context, q *query.GetTenantByDomain) error {
	if q.Domain == "feedbacktest.goenning.net" {
		q.Result = &entity.Tenant{Name: "Feedback for goenning.net", Subdomain: "goenning"}
		return nil
	} else {
		return app.ErrNotFound
	}
}

func mockGetTenantWithIncorrectSubdomains(ctx context.Context, q *query.GetTenantByDomain) error {
	if q.Domain == "feedbacktest.goenning.net" {
		q.Result = &entity.Tenant{Name: "Feedback for goenning.net", Subdomain: "demo"}
		return nil
	} else {
		return app.ErrNotFound
	}
}

func useCertificateDNS(t *testing.T) {
	t.Helper()
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		buffer := make([]byte, 1500)
		for {
			size, address, err := server.ReadFrom(buffer)
			if err != nil {
				return
			}

			var request dnsmessage.Message
			if err := request.Unpack(buffer[:size]); err != nil {
				t.Error(err)
				return
			}

			response := dnsmessage.Message{
				Header:    dnsmessage.Header{ID: request.ID, Response: true, Authoritative: true},
				Questions: request.Questions,
			}
			target := dnsmessage.MustNewName("goenning.test.fider.io.")
			for _, question := range request.Questions {
				response.Answers = append(response.Answers,
					dnsmessage.Resource{
						Header: dnsmessage.ResourceHeader{Name: question.Name, Type: dnsmessage.TypeCNAME, Class: dnsmessage.ClassINET},
						Body:   &dnsmessage.CNAMEResource{CNAME: target},
					},
					dnsmessage.Resource{
						Header: dnsmessage.ResourceHeader{Name: target, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET},
						Body:   &dnsmessage.AResource{A: [4]byte{127, 0, 0, 1}},
					},
				)
			}

			packet, err := response.Pack()
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := server.WriteTo(packet, address); err != nil {
				t.Error(err)
				return
			}
		}
	}()

	previous := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp", server.LocalAddr().String())
		},
	}
	t.Cleanup(func() {
		net.DefaultResolver = previous
		server.Close()
		<-finished
	})
}

func TestUseAutoCert_WhenCNAMEAreRegistered(t *testing.T) {
	RegisterT(t)
	useCertificateDNS(t)
	previousPath := env.Config.BlobStorage.FS.Path
	env.Config.BlobStorage.FS.Path = t.TempDir()
	t.Cleanup(func() { env.Config.BlobStorage.FS.Path = previousPath })
	bus.Init(fs.Service{})
	bus.AddHandler(mockGetTenantWithCorrectSubdomains)

	var requests atomic.Int32
	issuer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		http.Error(response, "fixture refused issuance", http.StatusBadRequest)
	}))
	defer issuer.Close()

	manager, err := NewCertificateManager(context.Background(), "", "")
	Expect(err).IsNil()
	manager.autotls.Client.DirectoryURL = issuer.URL

	cert, err := manager.GetCertificate(&tls.ClientHelloInfo{
		ServerName: "feedbacktest.goenning.net",
	})
	Expect(err).IsNotNil()
	Expect(err.Error()).ContainsSubstring("fixture refused issuance")
	Expect(requests.Load()).Equals(int32(1))
	Expect(cert).IsNil()
}

func TestGetCertificate(t *testing.T) {
	RegisterT(t)

	var testCases = []struct {
		mode       string
		cert       string
		serverName string
	}{
		{"multi", "all-test-fider-io", ""},
		{"multi", "all-test-fider-io", "fider"},
		{"multi", "all-test-fider-io", "feedback.test.fider.io"},
		{"multi", "all-test-fider-io", "FEEDBACK.test.fider.io"},
		{"multi", "all-test-fider-io", "44.194.119.243"},
		{"single", "test-fider-io", "test.fider.io"},
		{"single", "test-fider-io", "fider.io"},
		{"single", "test-fider-io", "44.194.119.243"},
	}

	for _, testCase := range testCases {
		env.Config.HostMode = testCase.mode
		certFile := env.Path("/app/pkg/web/testdata/" + testCase.cert + ".crt")
		keyFile := env.Path("/app/pkg/web/testdata/" + testCase.cert + ".key")
		wildcardCert, _ := tls.LoadX509KeyPair(certFile, keyFile)

		manager, err := NewCertificateManager(context.Background(), certFile, keyFile)
		Expect(err).IsNil()
		cert, err := manager.GetCertificate(&tls.ClientHelloInfo{
			ServerName: testCase.serverName,
		})

		Expect(err).IsNil()
		Expect(cert.Certificate).Equals(wildcardCert.Certificate)
	}
}

func TestGetCertificate_WhenCNAMEAreNotConfigured(t *testing.T) {
	RegisterT(t)
	bus.Init(fs.Service{})
	bus.AddHandler(mockGetTenantWithCorrectSubdomains)

	manager, err := NewCertificateManager(context.Background(), "", "")
	Expect(err).IsNil()

	invalidServerNames := []string{"feedback.heyworld.com", "ideas.app.com"}

	for _, serverName := range invalidServerNames {
		cert, err := manager.GetCertificate(&tls.ClientHelloInfo{
			ServerName: serverName,
		})
		Expect(err.Error()).ContainsSubstring(`no tenant found with cname ` + serverName)
		Expect(cert).IsNil()
	}
}

func TestGetCertificate_WhenCNAMEDoesntMatch(t *testing.T) {
	RegisterT(t)
	useCertificateDNS(t)
	bus.Init(fs.Service{})

	bus.AddHandler(mockGetTenantWithIncorrectSubdomains)

	manager, err := NewCertificateManager(context.Background(), "", "")
	Expect(err).IsNil()

	cert, err := manager.GetCertificate(&tls.ClientHelloInfo{ServerName: "feedbacktest.goenning.net"})
	Expect(err.Error()).ContainsSubstring("cname goenning.test.fider.io. (from feedbacktest.goenning.net) doesn't match configured host demo.test.fider.io")
	Expect(cert).IsNil()
}

func TestGetCertificate_ServerNameMatchesCertificate_ShouldReturnIt(t *testing.T) {
	RegisterT(t)
	bus.Init(fs.Service{})
	bus.AddHandler(mockGetTenantWithCorrectSubdomains)

	certFile := env.Etc("dev-fider-io.crt")
	certKey := env.Etc("dev-fider-io.key")
	manager, err := NewCertificateManager(context.Background(), certFile, certKey)
	Expect(err).IsNil()

	serverNames := []string{"dev.fider.io", "feedback.dev.fider.io", "anything.dev.fider.io", "IDEAS.DEV.fider.io", ".feedback.DEV.fider.io"}

	for _, serverName := range serverNames {
		cert, err := manager.GetCertificate(&tls.ClientHelloInfo{
			ServerName: serverName,
		})
		Expect(err).IsNil()
		Expect(cert).IsNotNil()
	}
}

func TestGetCertificate_ServerNameDoesntMatchCertificate_ButEndsWithHostName_ShouldThrow(t *testing.T) {
	RegisterT(t)
	bus.Init(fs.Service{})
	bus.AddHandler(mockGetTenantWithCorrectSubdomains)

	env.Config.HostDomain = "dev.fider.io"
	certFile := env.Etc("dev-fider-io.crt")
	certKey := env.Etc("dev-fider-io.key")
	manager, err := NewCertificateManager(context.Background(), certFile, certKey)
	Expect(err).IsNil()

	serverNames := []string{"sub.feedback.dev.fider.io"}

	for _, serverName := range serverNames {
		cert, err := manager.GetCertificate(&tls.ClientHelloInfo{
			ServerName: serverName,
		})
		Expect(err.Error()).ContainsSubstring("invalid ServerName used: " + serverName)
		Expect(cert).IsNil()
	}
}

func TestAutoCertCacheUsesTemporaryStorage(t *testing.T) {
	previousPath := env.Config.BlobStorage.FS.Path
	root := t.TempDir()
	env.Config.BlobStorage.FS.Path = root
	t.Cleanup(func() { env.Config.BlobStorage.FS.Path = previousPath })
	bus.Init(fs.Service{})
	cache := NewAutoCertCache()
	ctx := context.Background()
	if err := cache.Put(ctx, "acme_account+key", []byte("test cache value")); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(root, "autocert", "acme_account+key"))
	if err != nil || string(stored) != "test cache value" {
		t.Fatalf("cache did not use the test-owned directory: %v", err)
	}
	value, err := cache.Get(ctx, "acme_account+key")
	if err != nil || string(value) != "test cache value" {
		t.Fatalf("cache round trip failed: %v", err)
	}
}

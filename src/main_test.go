package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	dns "github.com/cert-manager/cert-manager/test/acme"

	// Ensure the base plugins are linked so directives like 'log' or 'etcd' exist
	"github.com/coredns/caddy"
	_ "github.com/coredns/coredns/core/plugin"
	"go.etcd.io/etcd/server/v3/embed"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
)

func startEtcdForTesting() string {

	// 1. Generate the standard secure defaults configuration block
	cfg := embed.NewConfig()
	cfg.Dir = os.TempDir() + "/solver-isolated-etcd"

	// 2. Direct etcd to let the OS kernel handle socket binding natively
	// By using a domain pointer like 0.0.0.0 or 127.0.0.1 with an unassigned string,
	// we avoid guessing explicit numeric ports entirely.
	ephemeralURL, _ := url.Parse("http://127.0.0.1:0")

	cfg.ListenPeerUrls = []url.URL{*ephemeralURL}
	cfg.ListenClientUrls = []url.URL{*ephemeralURL}
	cfg.AdvertisePeerUrls = []url.URL{*ephemeralURL}
	cfg.AdvertiseClientUrls = []url.URL{*ephemeralURL}
	cfg.InitialCluster = fmt.Sprintf("default=%s", ephemeralURL.String())

	// 3. Bypass etcd's strict validation panic by setting a custom cluster token
	cfg.InitialClusterToken = "etcd-test-cluster"

	// 4. Start the embedded instance smoothly
	var err error
	etcdSrv, err := embed.StartEtcd(cfg)
	if err != nil {
		fmt.Printf("Error starting isolated etcd: %v\n", err)
		os.Exit(1)
	}
	<-etcdSrv.Server.ReadyNotify()

	// 5. SUCCESS! Read the true port assigned dynamically by the kernel
	// After etcd starts up successfully, the actual active endpoint is populated here.
	etcdHostname := etcdSrv.Clients[0].Addr().String()

	return etcdHostname
}

func startCoreDNSForTesting(etcdHostname string) uint16 {
	// 1. Find a free UDP port for CoreDNS
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Printf("Failed to bind UDP port: %v\n", err)
		os.Exit(1)
	}
	dnsUDPPort := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close() // Free it immediately so CoreDNS can claim it

	// 2. Define a dynamic Corefile configuration pointing to our fresh etcd instance
	corefileContent := fmt.Sprintf(`.:%d {
		log
		etcd %s {
			stubzones
			path /public
			endpoint http://%s
		}
	}`, dnsUDPPort, zone, etcdHostname)

	// 3. Start CoreDNS inline using the underlying Caddy engine
	go func() {
		caddyInput := caddy.CaddyfileInput{
			Contents:       []byte(corefileContent),
			Filepath:       "Corefile",
			ServerTypeName: "dns",
		}

		instance, err := caddy.Start(caddyInput)
		if err != nil {
			fmt.Printf("CoreDNS startup failed: %v\n", err)
			return
		}

		// Keeps the server instance active for the scope of the test routine
		instance.Wait()
	}()

	// Give CoreDNS a tiny window to bind its socket safely
	time.Sleep(600 * time.Millisecond)
	fmt.Printf("\n---> SUCCESS: CoreDNS Authoritative Server live on UDP port: %d\n\n", dnsUDPPort)

	return uint16(dnsUDPPort)
}

func TestRunsSuite(t *testing.T) {

	if zone == "" {
		zone = "example.com."
	}

	etcdHostname := startEtcdForTesting()

	os.Setenv("ETCD_URLS", fmt.Sprintf("http://%s", etcdHostname))

	fmt.Printf("\n---> SUCCESS: Dedicated Solver etcd running on: %s\n\n", etcdHostname)

	dnsUDPPort := startCoreDNSForTesting(etcdHostname)

	// solver := example.New("59351")
	fixture := dns.NewFixture(&CoreDNSEtcdProviderSolver{},
		dns.SetResolvedZone(zone),
		dns.SetManifestPath("../testdata/etcddns-solver"),
		dns.SetAllowAmbientCredentials(false),
		dns.SetUseAuthoritative(false),
		dns.SetDNSServer(fmt.Sprintf("localhost:%d", dnsUDPPort)),
	)

	//fixture.RunConformance(t)
	fixture.RunBasic(t)
	fixture.RunExtended(t)
}

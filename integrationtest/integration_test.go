package integration_test

import (
	"os"
	"testing"

	"github.com/gbolo/vsummary/common"
	"github.com/gbolo/vsummary/config"
	"github.com/gbolo/vsummary/db"
	"github.com/gbolo/vsummary/poller"
	"github.com/gbolo/vsummary/server"
	_ "github.com/go-sql-driver/mysql"
)

const (
	TestVcenterHost     = "127.0.0.1:8989"
	TestVcenterName     = "unit-test"
	TestVcenterUsername = "user"
	TestVcenterPassword = "pass"

	TestVsummaryUrl = "http://127.0.0.1:8080"
)

var (
	TestBackend *db.Backend
)

func setupServer(t *testing.T) {

	// set some overrides
	os.Setenv("VSUMMARY_LOG_LEVEL", "info")
	os.Setenv("VSUMMARY_SERVER_ACCESS_LOG", "false")
	os.Setenv("VSUMMARY_BACKEND_DB_DSN", "vsummary:secret@(127.0.0.1:13306)/vsummary")

	// init config and logging
	config.ConfigInit("")

	// init backend
	var err error
	TestBackend, err = db.InitBackend()
	if err != nil {
		t.Fatalf("Error InitBackend: %v", err)
	}

	// apply backend schemas
	err = TestBackend.ApplySchemas()
	if err != nil {
		t.Fatalf("Error ApplySchemas: %v", err)
	}

	// start vsummary server
	go server.Start()
}

func setupPoller(t *testing.T) (testPoller *poller.Poller) {

	// common.Poller
	commonPoller := common.Poller{
		VcenterHost:       TestVcenterHost,
		VcenterName:       TestVcenterName,
		Username:          TestVcenterUsername,
		PlainTextPassword: TestVcenterPassword,
	}

	// Base Poller
	testPoller = poller.NewPoller(commonPoller)

	// Test Connection
	err := poller.TestConnection(*testPoller.Config)
	if err != nil {
		t.Fatalf("Error connecting to vCenter Simulator: %v", err)
	}
	return
}

func TestExternalPoller(t *testing.T) {
	setupServer(t)
	// create external poller
	externalpoller := poller.NewExternalPoller(common.Poller{
		VcenterHost:       TestVcenterHost,
		VcenterName:       TestVcenterName,
		Username:          TestVcenterUsername,
		PlainTextPassword: TestVcenterPassword,
		IntervalMin:       10,
		Enabled:           true,
	})

	// external poller sends results to vsummary server
	externalpoller.SetApiUrl(TestVsummaryUrl)
	errs := externalpoller.PollThenSend()
	if len(errs) > 0 {
		t.Errorf("error(s) with GetPollResults: %v", errs)
	}
}

func TestInternalPoller(t *testing.T) {
	// create internal poller
	internalPoller := poller.InternalPoller{
		Poller: *setupPoller(t),
	}

	// internal poller stores results to database directly
	internalPoller.SetBackend(*TestBackend)
	errs := internalPoller.PollThenStore()
	if len(errs) > 0 {
		t.Errorf("error(s) with PollThenStore: %v", errs)
	}
}

func TestValidateVcenterCertficate(t *testing.T) {
	// specify the correct CA file
	os.Setenv("VSUMMARY_POLLER_VCENTER_CAFILE", "../testdata/tls/ca_root.pem")
	// create external poller
	externalpoller := poller.NewExternalPoller(common.Poller{
		VcenterHost:       TestVcenterHost,
		VcenterName:       TestVcenterName,
		Username:          TestVcenterUsername,
		PlainTextPassword: TestVcenterPassword,
		IntervalMin:       10,
		Enabled:           true,
	})

	_, errs := externalpoller.GetPollResults()
	if len(errs) > 0 {
		t.Errorf("expected no errors but received: %v", errs)
	}

	// specify the incorrect CA file
	os.Setenv("VSUMMARY_POLLER_VCENTER_CAFILE", "../testdata/tls/bad_root_ca.pem")
	// create external poller
	externalpoller = poller.NewExternalPoller(common.Poller{
		VcenterHost:       TestVcenterHost,
		VcenterName:       TestVcenterName,
		Username:          TestVcenterUsername,
		PlainTextPassword: TestVcenterPassword,
		IntervalMin:       10,
		Enabled:           true,
	})

	_, errs = externalpoller.GetPollResults()
	if len(errs) == 0 {
		t.Error("expected TLS errors but received none")
	}
}

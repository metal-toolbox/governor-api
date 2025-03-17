package dbtools

import "github.com/cockroachdb/cockroach-go/v2/testserver"

// TestServerCRDBVersion is the version of CockroachDB that the test server is running
// v23.1.28 is the last version under BSL
const TestServerCRDBVersion = "v23.1.28"

// NewCRDBTestServer creates a new CockroachDB test server
func NewCRDBTestServer() (testserver.TestServer, error) {
	return testserver.NewTestServer(testserver.CustomVersionOpt(TestServerCRDBVersion))
}

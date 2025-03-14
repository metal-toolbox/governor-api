package dbtools

import "github.com/cockroachdb/cockroach-go/v2/testserver"

// TestServerCRDBVersion is the version of CockroachDB that the test server is running
const TestServerCRDBVersion = "v24.3.8"

// NewCRDBTestServer creates a new CockroachDB test server
func NewCRDBTestServer() (testserver.TestServer, error) {
	return testserver.NewTestServer(testserver.CustomVersionOpt(TestServerCRDBVersion))
}

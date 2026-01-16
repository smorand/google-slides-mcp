// Package integration provides end-to-end integration tests for the Google Slides MCP server.
//
// Integration tests verify that all components work together correctly with real
// Google APIs (in test mode). These tests require valid OAuth2 credentials and
// are typically run as part of CI/CD when credentials are available.
//
// # Running Integration Tests
//
// Integration tests are skipped by default unless the INTEGRATION_TEST environment
// variable is set:
//
//	INTEGRATION_TEST=1 go test -v ./internal/integration/...
//
// Or using the Makefile target:
//
//	make test-integration
//
// # Required Environment Variables
//
// The following environment variables must be set for integration tests:
//
//   - INTEGRATION_TEST: Set to "1" to enable integration tests
//   - GOOGLE_CLIENT_ID: OAuth2 client ID
//   - GOOGLE_CLIENT_SECRET: OAuth2 client secret
//   - GOOGLE_REFRESH_TOKEN: Valid refresh token for testing
//   - TEST_PRESENTATION_ID: (Optional) Existing presentation ID for read-only tests
//
// # Test Fixtures
//
// Integration tests use temporary test fixtures that are automatically cleaned up
// after each test. The cleanup process removes:
//
//   - Test presentations created during tests
//   - Test slides added to existing presentations
//   - Test objects created on slides
//
// # Test Structure
//
// Tests are organized by functionality:
//
//   - auth_test.go: OAuth2 authentication flow tests
//   - presentation_test.go: Presentation CRUD operations
//   - slide_test.go: Slide operations (add, delete, reorder)
//   - object_test.go: Object operations (shapes, text, images)
package integration

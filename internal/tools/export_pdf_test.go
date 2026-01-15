package tools

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"golang.org/x/oauth2"
)

// mockReadCloser implements io.ReadCloser for testing.
type mockReadCloser struct {
	*bytes.Reader
	closed bool
}

func newMockReadCloser(data []byte) *mockReadCloser {
	return &mockReadCloser{
		Reader: bytes.NewReader(data),
	}
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

// Sample PDF data with page markers for testing page count detection.
// This is a minimal representation of PDF structure.
var samplePDFData = []byte(`%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R 4 0 R 5 0 R] /Count 3 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 6 0 R >>
endobj
4 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 7 0 R >>
endobj
5 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 8 0 R >>
endobj
%%EOF`)

// Single page PDF
var singlePagePDFData = []byte(`%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>
endobj
%%EOF`)

func TestExportPDF_Success(t *testing.T) {
	mockService := &mockDriveService{
		ExportFileFunc: func(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error) {
			if fileID != "presentation-123" {
				t.Errorf("expected file ID 'presentation-123', got: %s", fileID)
			}
			if mimeType != "application/pdf" {
				t.Errorf("expected mime type 'application/pdf', got: %s", mimeType)
			}
			return newMockReadCloser(samplePDFData), nil
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.ExportPDF(context.Background(), tokenSource, ExportPDFInput{
		PresentationID: "presentation-123",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify PDF is returned as base64
	if output.PDFBase64 == "" {
		t.Error("expected PDFBase64 to be non-empty")
	}

	// Verify file size matches
	if output.FileSize != len(samplePDFData) {
		t.Errorf("expected file size %d, got %d", len(samplePDFData), output.FileSize)
	}

	// Verify page count (should be 3 based on sample PDF)
	if output.PageCount != 3 {
		t.Errorf("expected page count 3, got %d", output.PageCount)
	}
}

func TestExportPDF_SinglePage(t *testing.T) {
	mockService := &mockDriveService{
		ExportFileFunc: func(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error) {
			return newMockReadCloser(singlePagePDFData), nil
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.ExportPDF(context.Background(), tokenSource, ExportPDFInput{
		PresentationID: "single-page-pres",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.PageCount != 1 {
		t.Errorf("expected page count 1, got %d", output.PageCount)
	}
}

func TestExportPDF_EmptyPresentationID(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.ExportPDF(context.Background(), tokenSource, ExportPDFInput{
		PresentationID: "",
	})

	if err == nil {
		t.Fatal("expected error for empty presentation ID")
	}
	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got: %v", err)
	}
}

func TestExportPDF_PresentationNotFound(t *testing.T) {
	mockService := &mockDriveService{
		ExportFileFunc: func(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error) {
			return nil, errors.New("googleapi: Error 404: File not found: xyz123")
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.ExportPDF(context.Background(), tokenSource, ExportPDFInput{
		PresentationID: "nonexistent-presentation",
	})

	if err == nil {
		t.Fatal("expected error for presentation not found")
	}
	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got: %v", err)
	}
}

func TestExportPDF_AccessDenied(t *testing.T) {
	mockService := &mockDriveService{
		ExportFileFunc: func(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error) {
			return nil, errors.New("googleapi: Error 403: The user does not have sufficient permissions")
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.ExportPDF(context.Background(), tokenSource, ExportPDFInput{
		PresentationID: "restricted-presentation",
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got: %v", err)
	}
}

func TestExportPDF_DriveServiceFactoryError(t *testing.T) {
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return nil, errors.New("failed to create drive service")
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.ExportPDF(context.Background(), tokenSource, ExportPDFInput{
		PresentationID: "presentation-id",
	})

	if err == nil {
		t.Fatal("expected error when service factory fails")
	}
	if !errors.Is(err, ErrDriveAPIError) {
		t.Errorf("expected ErrDriveAPIError, got: %v", err)
	}
}

func TestExportPDF_GenericExportError(t *testing.T) {
	mockService := &mockDriveService{
		ExportFileFunc: func(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error) {
			return nil, errors.New("googleapi: Error 500: Internal Server Error")
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.ExportPDF(context.Background(), tokenSource, ExportPDFInput{
		PresentationID: "presentation-id",
	})

	if err == nil {
		t.Fatal("expected error for export failure")
	}
	if !errors.Is(err, ErrExportFailed) {
		t.Errorf("expected ErrExportFailed, got: %v", err)
	}
}

func TestExportPDF_VariousContentTypes(t *testing.T) {
	// Test that export works with different content types in presentations
	testCases := []struct {
		name            string
		pdfData         []byte
		expectedPages   int
		expectSuccess   bool
	}{
		{
			name:          "three page presentation",
			pdfData:       samplePDFData,
			expectedPages: 3,
			expectSuccess: true,
		},
		{
			name:          "single page presentation",
			pdfData:       singlePagePDFData,
			expectedPages: 1,
			expectSuccess: true,
		},
		{
			name:          "empty PDF (no page markers)",
			pdfData:       []byte("%PDF-1.4\n%%EOF"),
			expectedPages: 0, // Unable to detect pages
			expectSuccess: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockService := &mockDriveService{
				ExportFileFunc: func(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error) {
					return newMockReadCloser(tc.pdfData), nil
				},
			}

			driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
				return mockService, nil
			}

			tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
			tokenSource := &mockTokenSource{}

			output, err := tools.ExportPDF(context.Background(), tokenSource, ExportPDFInput{
				PresentationID: "test-presentation",
			})

			if tc.expectSuccess {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if output.PageCount != tc.expectedPages {
					t.Errorf("expected page count %d, got %d", tc.expectedPages, output.PageCount)
				}
				if output.FileSize != len(tc.pdfData) {
					t.Errorf("expected file size %d, got %d", len(tc.pdfData), output.FileSize)
				}
			} else {
				if err == nil {
					t.Error("expected error but got none")
				}
			}
		})
	}
}

func TestExportPDF_Base64Encoding(t *testing.T) {
	// Simple test data to verify base64 encoding
	testData := []byte("test PDF data")

	mockService := &mockDriveService{
		ExportFileFunc: func(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error) {
			return newMockReadCloser(testData), nil
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.ExportPDF(context.Background(), tokenSource, ExportPDFInput{
		PresentationID: "test-presentation",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected base64: "dGVzdCBQREYgZGF0YQ=="
	expectedBase64 := "dGVzdCBQREYgZGF0YQ=="
	if output.PDFBase64 != expectedBase64 {
		t.Errorf("expected base64 '%s', got '%s'", expectedBase64, output.PDFBase64)
	}
}

func TestCountPDFPages(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		expected int
	}{
		{
			name:     "three pages with space",
			data:     samplePDFData,
			expected: 3,
		},
		{
			name:     "one page",
			data:     singlePagePDFData,
			expected: 1,
		},
		{
			name:     "no page markers",
			data:     []byte("%PDF-1.4\n%%EOF"),
			expected: 0,
		},
		{
			name:     "empty data",
			data:     []byte{},
			expected: 0,
		},
		{
			name:     "Pages object should not count",
			data:     []byte("/Type /Pages"),
			expected: 0,
		},
		{
			name:     "compact format without space",
			data:     []byte("<< /Type/Page /Parent 2 0 R >>"),
			expected: 1,
		},
		{
			name:     "mixed formats",
			data:     []byte("<< /Type /Page >> << /Type/Page >>"),
			expected: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := countPDFPages(tc.data)
			if result != tc.expected {
				t.Errorf("countPDFPages(%q) = %d, expected %d", string(tc.data), result, tc.expected)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		pattern  []byte
		expected bool
	}{
		{
			name:     "exact match",
			data:     []byte("/Type /Page"),
			pattern:  []byte("/Type /Page"),
			expected: true,
		},
		{
			name:     "prefix match",
			data:     []byte("/Type /Page /Parent"),
			pattern:  []byte("/Type /Page"),
			expected: true,
		},
		{
			name:     "prefix match but Pages",
			data:     []byte("/Type /Pages"),
			pattern:  []byte("/Type /Page"),
			expected: true, // This matches as prefix - the 's' check is done in countPDFPages
		},
		{
			name:     "no match different content",
			data:     []byte("/Type /Catalog"),
			pattern:  []byte("/Type /Page"),
			expected: false,
		},
		{
			name:     "data shorter than pattern",
			data:     []byte("/Type"),
			pattern:  []byte("/Type /Page"),
			expected: false,
		},
		{
			name:     "empty data",
			data:     []byte{},
			pattern:  []byte("/Type /Page"),
			expected: false,
		},
		{
			name:     "empty pattern",
			data:     []byte("/Type /Page"),
			pattern:  []byte{},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesPattern(tc.data, tc.pattern)
			if result != tc.expected {
				t.Errorf("matchesPattern(%q, %q) = %v, expected %v", string(tc.data), string(tc.pattern), result, tc.expected)
			}
		})
	}
}

func TestExportPDF_ClosesCalled(t *testing.T) {
	mockReader := newMockReadCloser([]byte("test data"))

	mockService := &mockDriveService{
		ExportFileFunc: func(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error) {
			return mockReader, nil
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.ExportPDF(context.Background(), tokenSource, ExportPDFInput{
		PresentationID: "test-presentation",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that Close was called on the reader
	if !mockReader.closed {
		t.Error("expected Close() to be called on the reader")
	}
}

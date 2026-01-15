package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"golang.org/x/oauth2"
)

// Sentinel errors for export_pdf tool.
var (
	ErrExportFailed        = errors.New("failed to export presentation")
	ErrInvalidPresentationID = errors.New("invalid presentation ID")
)

// PDF MIME type for export.
const pdfMimeType = "application/pdf"

// ExportPDFInput represents the input for the export_pdf tool.
type ExportPDFInput struct {
	PresentationID string `json:"presentation_id"`
}

// ExportPDFOutput represents the output of the export_pdf tool.
type ExportPDFOutput struct {
	PDFBase64 string `json:"pdf_base64"`
	PageCount int    `json:"page_count"`
	FileSize  int    `json:"file_size"`
}

// ExportPDF exports a Google Slides presentation to PDF format.
func (t *Tools) ExportPDF(ctx context.Context, tokenSource oauth2.TokenSource, input ExportPDFInput) (*ExportPDFOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	t.config.Logger.Info("exporting presentation to PDF",
		slog.String("presentation_id", input.PresentationID),
	)

	// Create Drive service
	driveService, err := t.driveServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create drive service: %v", ErrDriveAPIError, err)
	}

	// Export the presentation to PDF
	pdfData, err := driveService.ExportFile(ctx, input.PresentationID, pdfMimeType)
	if err != nil {
		if isNotFoundError(err) {
			return nil, fmt.Errorf("%w: presentation not found", ErrPresentationNotFound)
		}
		if isForbiddenError(err) {
			return nil, fmt.Errorf("%w: access denied to presentation", ErrAccessDenied)
		}
		return nil, fmt.Errorf("%w: %v", ErrExportFailed, err)
	}
	defer pdfData.Close()

	// Read all PDF data
	data, err := io.ReadAll(pdfData)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read PDF data: %v", ErrExportFailed, err)
	}

	// Count pages in PDF (basic heuristic based on /Type /Page occurrences)
	pageCount := countPDFPages(data)

	// Encode PDF as base64
	pdfBase64 := base64.StdEncoding.EncodeToString(data)

	output := &ExportPDFOutput{
		PDFBase64: pdfBase64,
		PageCount: pageCount,
		FileSize:  len(data),
	}

	t.config.Logger.Info("presentation exported to PDF successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("page_count", output.PageCount),
		slog.Int("file_size", output.FileSize),
	)

	return output, nil
}

// countPDFPages attempts to count pages in a PDF by looking for /Type /Page markers.
// This is a simple heuristic that works for most PDFs but may not be 100% accurate.
func countPDFPages(data []byte) int {
	// Look for the pattern /Type /Page or /Type/Page (with optional whitespace)
	// This is a common pattern in PDF files to identify page objects.
	count := 0
	pattern1 := []byte("/Type /Page")
	pattern2 := []byte("/Type/Page")

	for i := 0; i < len(data)-len(pattern1); i++ {
		// Check for pattern1: /Type /Page
		if matchesPattern(data[i:], pattern1) {
			// Make sure this is not /Type /Pages (the page tree, not a page)
			if i+len(pattern1) < len(data) && data[i+len(pattern1)] != 's' {
				count++
			}
		}
		// Check for pattern2: /Type/Page
		if matchesPattern(data[i:], pattern2) {
			// Make sure this is not /Type/Pages
			if i+len(pattern2) < len(data) && data[i+len(pattern2)] != 's' {
				count++
			}
		}
	}

	// If we couldn't detect pages, return 0 (unknown)
	return count
}

// matchesPattern checks if the data starts with the given pattern.
func matchesPattern(data, pattern []byte) bool {
	if len(data) < len(pattern) {
		return false
	}
	for i := 0; i < len(pattern); i++ {
		if data[i] != pattern[i] {
			return false
		}
	}
	return true
}

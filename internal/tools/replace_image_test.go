package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/slides/v1"
)

func TestReplaceImage(t *testing.T) {
	// Sample valid PNG data (1x1 pixel transparent PNG)
	validPNGData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
	validPNGBase64 := base64.StdEncoding.EncodeToString(validPNGData)

	// Override time function for deterministic object IDs
	originalTimeNow := imageTimeNowFunc
	defer func() { imageTimeNowFunc = originalTimeNow }()
	imageTimeNowFunc = func() time.Time {
		return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name              string
		input             ReplaceImageInput
		mockPresentation  *slides.Presentation
		mockGetErr        error
		mockBatchErr      error
		mockUploadFile    *drive.File
		mockUploadErr     error
		mockMakePublicErr error
		expectedErr       error
		expectedErrMsg    string
		expectedOutput    *ReplaceImageOutput
	}{
		{
			name: "successful replace with preserve_size=true (default)",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "image-1",
				ImageBase64:    validPNGBase64,
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
								Transform: &slides.AffineTransform{
									ScaleX:     1.0,
									ScaleY:     1.0,
									TranslateX: 100000,
									TranslateY: 200000,
									Unit:       "EMU",
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: 500000, Unit: "EMU"},
									Height: &slides.Dimension{Magnitude: 300000, Unit: "EMU"},
								},
							},
						},
					},
				},
			},
			mockUploadFile: &drive.File{Id: "drive-file-123"},
			expectedOutput: &ReplaceImageOutput{
				ObjectID:      "image-1",
				NewObjectID:   "image_1705320000000000000",
				PreservedSize: true,
			},
		},
		{
			name: "successful replace with preserve_size=false",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "image-1",
				ImageBase64:    validPNGBase64,
				PreserveSize:   boolPtr(false),
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
								Transform: &slides.AffineTransform{
									ScaleX:     1.0,
									ScaleY:     1.0,
									TranslateX: 100000,
									TranslateY: 200000,
									Unit:       "EMU",
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: 500000, Unit: "EMU"},
									Height: &slides.Dimension{Magnitude: 300000, Unit: "EMU"},
								},
							},
						},
					},
				},
			},
			mockUploadFile: &drive.File{Id: "drive-file-456"},
			expectedOutput: &ReplaceImageOutput{
				ObjectID:      "image-1",
				NewObjectID:   "image_1705320000000000000",
				PreservedSize: false,
			},
		},
		{
			name: "successful replace with explicit preserve_size=true",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "image-1",
				ImageBase64:    validPNGBase64,
				PreserveSize:   boolPtr(true),
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
						},
					},
				},
			},
			mockUploadFile: &drive.File{Id: "drive-file-789"},
			expectedOutput: &ReplaceImageOutput{
				ObjectID:      "image-1",
				NewObjectID:   "image_1705320000000000000",
				PreservedSize: true,
			},
		},
		{
			name: "error - empty presentation_id",
			input: ReplaceImageInput{
				PresentationID: "",
				ObjectID:       "image-1",
				ImageBase64:    validPNGBase64,
			},
			expectedErr:    ErrInvalidPresentationID,
			expectedErrMsg: "presentation_id is required",
		},
		{
			name: "error - empty object_id",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "",
				ImageBase64:    validPNGBase64,
			},
			expectedErr:    ErrObjectNotFound,
			expectedErrMsg: "object_id is required",
		},
		{
			name: "error - empty image_base64",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "image-1",
				ImageBase64:    "",
			},
			expectedErr:    ErrInvalidImageData,
			expectedErrMsg: "image_base64 is required",
		},
		{
			name: "error - invalid base64",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "image-1",
				ImageBase64:    "not-valid-base64!!!",
			},
			expectedErr: ErrInvalidImageData,
		},
		{
			name: "error - unknown image format",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "image-1",
				ImageBase64:    base64.StdEncoding.EncodeToString([]byte("random data that is not an image")),
			},
			expectedErr:    ErrInvalidImageData,
			expectedErrMsg: "unable to detect image format",
		},
		{
			name: "error - presentation not found",
			input: ReplaceImageInput{
				PresentationID: "not-exists",
				ObjectID:       "image-1",
				ImageBase64:    validPNGBase64,
			},
			mockGetErr:  errors.New("404: Not Found"),
			expectedErr: ErrPresentationNotFound,
		},
		{
			name: "error - access denied",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "image-1",
				ImageBase64:    validPNGBase64,
			},
			mockGetErr:  errors.New("403: Forbidden"),
			expectedErr: ErrAccessDenied,
		},
		{
			name: "error - object not found",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "not-exists",
				ImageBase64:    validPNGBase64,
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId:     "slide-1",
						PageElements: []*slides.PageElement{},
					},
				},
			},
			expectedErr:    ErrObjectNotFound,
			expectedErrMsg: "object 'not-exists' not found in presentation",
		},
		{
			name: "error - object is not an image",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "shape-1",
				ImageBase64:    validPNGBase64,
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "shape-1",
								Shape:    &slides.Shape{ShapeType: "TEXT_BOX"},
							},
						},
					},
				},
			},
			expectedErr:    ErrNotImageObject,
			expectedErrMsg: "object 'shape-1' is not an image",
		},
		{
			name: "error - upload failed",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "image-1",
				ImageBase64:    validPNGBase64,
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
						},
					},
				},
			},
			mockUploadErr: errors.New("upload failed"),
			expectedErr:   ErrImageUploadFailed,
		},
		{
			name: "error - batch update failed",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "image-1",
				ImageBase64:    validPNGBase64,
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
						},
					},
				},
			},
			mockUploadFile: &drive.File{Id: "drive-file-123"},
			mockBatchErr:   errors.New("batch update failed"),
			expectedErr:    ErrReplaceImageFailed,
		},
		{
			name: "warning logged but success when make public fails",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "image-1",
				ImageBase64:    validPNGBase64,
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
						},
					},
				},
			},
			mockUploadFile:    &drive.File{Id: "drive-file-123"},
			mockMakePublicErr: errors.New("permission denied"),
			expectedOutput: &ReplaceImageOutput{
				ObjectID:      "image-1",
				NewObjectID:   "image_1705320000000000000",
				PreservedSize: true,
			},
		},
		{
			name: "find image in nested group",
			input: ReplaceImageInput{
				PresentationID: "pres-1",
				ObjectID:       "nested-image",
				ImageBase64:    validPNGBase64,
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "group-1",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{
											ObjectId: "nested-image",
											Image:    &slides.Image{},
										},
									},
								},
							},
						},
					},
				},
			},
			mockUploadFile: &drive.File{Id: "drive-file-nested"},
			expectedOutput: &ReplaceImageOutput{
				ObjectID:      "nested-image",
				NewObjectID:   "image_1705320000000000000",
				PreservedSize: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock services
			mockSlides := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if tt.mockGetErr != nil {
						return nil, tt.mockGetErr
					}
					return tt.mockPresentation, nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					if tt.mockBatchErr != nil {
						return nil, tt.mockBatchErr
					}
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			}

			mockDrive := &mockDriveService{
				UploadFileFunc: func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
					if tt.mockUploadErr != nil {
						return nil, tt.mockUploadErr
					}
					return tt.mockUploadFile, nil
				},
				MakeFilePublicFunc: func(ctx context.Context, fileID string) error {
					return tt.mockMakePublicErr
				},
			}

			slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockSlides, nil
			}
			driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
				return mockDrive, nil
			}

			tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)

			output, err := tools.ReplaceImage(context.Background(), nil, tt.input)

			// Check error
			if tt.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error containing %v, got nil", tt.expectedErr)
				}
				if !errors.Is(err, tt.expectedErr) {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
				if tt.expectedErrMsg != "" && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.expectedErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check output
			if output == nil {
				t.Fatal("expected output, got nil")
			}
			if output.ObjectID != tt.expectedOutput.ObjectID {
				t.Errorf("expected ObjectID %s, got %s", tt.expectedOutput.ObjectID, output.ObjectID)
			}
			if output.NewObjectID != tt.expectedOutput.NewObjectID {
				t.Errorf("expected NewObjectID %s, got %s", tt.expectedOutput.NewObjectID, output.NewObjectID)
			}
			if output.PreservedSize != tt.expectedOutput.PreservedSize {
				t.Errorf("expected PreservedSize %v, got %v", tt.expectedOutput.PreservedSize, output.PreservedSize)
			}
		})
	}
}

func TestBuildReplaceImageRequests(t *testing.T) {
	tests := []struct {
		name           string
		objectID       string
		slideID        string
		driveFileID    string
		oldElement     *slides.PageElement
		preserveSize   bool
		expectedDelete bool
		expectedCreate bool
		checkTransform bool
		checkSize      bool
	}{
		{
			name:        "with preserve_size=true and existing size/transform",
			objectID:    "img-1",
			slideID:     "slide-1",
			driveFileID: "drive-123",
			oldElement: &slides.PageElement{
				ObjectId: "img-1",
				Image:    &slides.Image{},
				Transform: &slides.AffineTransform{
					ScaleX:     1.5,
					ScaleY:     1.5,
					TranslateX: 100000,
					TranslateY: 200000,
					Unit:       "EMU",
				},
				Size: &slides.Size{
					Width:  &slides.Dimension{Magnitude: 500000, Unit: "EMU"},
					Height: &slides.Dimension{Magnitude: 300000, Unit: "EMU"},
				},
			},
			preserveSize:   true,
			expectedDelete: true,
			expectedCreate: true,
			checkTransform: true,
			checkSize:      true,
		},
		{
			name:        "with preserve_size=false",
			objectID:    "img-2",
			slideID:     "slide-2",
			driveFileID: "drive-456",
			oldElement: &slides.PageElement{
				ObjectId: "img-2",
				Image:    &slides.Image{},
				Transform: &slides.AffineTransform{
					ScaleX:     1.0,
					ScaleY:     1.0,
					TranslateX: 50000,
					TranslateY: 50000,
					Unit:       "EMU",
				},
				Size: &slides.Size{
					Width:  &slides.Dimension{Magnitude: 100000, Unit: "EMU"},
					Height: &slides.Dimension{Magnitude: 100000, Unit: "EMU"},
				},
			},
			preserveSize:   false,
			expectedDelete: true,
			expectedCreate: true,
			checkTransform: true,
			checkSize:      false,
		},
		{
			name:        "without transform in old element",
			objectID:    "img-3",
			slideID:     "slide-3",
			driveFileID: "drive-789",
			oldElement: &slides.PageElement{
				ObjectId: "img-3",
				Image:    &slides.Image{},
			},
			preserveSize:   true,
			expectedDelete: true,
			expectedCreate: true,
			checkTransform: false,
			checkSize:      false,
		},
		{
			name:        "with missing unit in transform",
			objectID:    "img-4",
			slideID:     "slide-4",
			driveFileID: "drive-101",
			oldElement: &slides.PageElement{
				ObjectId: "img-4",
				Image:    &slides.Image{},
				Transform: &slides.AffineTransform{
					ScaleX:     1.0,
					ScaleY:     1.0,
					TranslateX: 100,
					TranslateY: 200,
					Unit:       "", // Empty unit
				},
			},
			preserveSize:   true,
			expectedDelete: true,
			expectedCreate: true,
			checkTransform: true,
			checkSize:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests, newObjectID := buildReplaceImageRequests(tt.objectID, tt.slideID, tt.driveFileID, tt.oldElement, tt.preserveSize)

			// Should have 2 requests: delete + create
			if len(requests) != 2 {
				t.Fatalf("expected 2 requests, got %d", len(requests))
			}

			// First request should be delete
			if requests[0].DeleteObject == nil {
				t.Error("first request should be DeleteObject")
			} else if requests[0].DeleteObject.ObjectId != tt.objectID {
				t.Errorf("expected delete object ID %s, got %s", tt.objectID, requests[0].DeleteObject.ObjectId)
			}

			// Second request should be create
			if requests[1].CreateImage == nil {
				t.Error("second request should be CreateImage")
			} else {
				createReq := requests[1].CreateImage

				// Check new object ID
				if createReq.ObjectId != newObjectID {
					t.Errorf("expected create object ID %s, got %s", newObjectID, createReq.ObjectId)
				}

				// Check URL contains drive file ID
				expectedURL := "https://drive.google.com/uc?id=" + tt.driveFileID + "&export=download"
				if createReq.Url != expectedURL {
					t.Errorf("expected URL %s, got %s", expectedURL, createReq.Url)
				}

				// Check slide ID
				if createReq.ElementProperties == nil || createReq.ElementProperties.PageObjectId != tt.slideID {
					t.Errorf("expected page object ID %s", tt.slideID)
				}

				// Check transform was copied
				if tt.checkTransform {
					if createReq.ElementProperties.Transform == nil {
						t.Error("expected transform to be copied")
					} else {
						if tt.oldElement.Transform != nil {
							if createReq.ElementProperties.Transform.TranslateX != tt.oldElement.Transform.TranslateX {
								t.Errorf("expected TranslateX %f, got %f", tt.oldElement.Transform.TranslateX, createReq.ElementProperties.Transform.TranslateX)
							}
							if createReq.ElementProperties.Transform.TranslateY != tt.oldElement.Transform.TranslateY {
								t.Errorf("expected TranslateY %f, got %f", tt.oldElement.Transform.TranslateY, createReq.ElementProperties.Transform.TranslateY)
							}
							// Check unit defaults to EMU if empty
							if createReq.ElementProperties.Transform.Unit != "EMU" {
								t.Errorf("expected Unit EMU, got %s", createReq.ElementProperties.Transform.Unit)
							}
						}
					}
				}

				// Check size was copied (only when preserveSize is true)
				if tt.checkSize {
					if createReq.ElementProperties.Size == nil {
						t.Error("expected size to be copied when preserveSize=true")
					} else {
						if tt.oldElement.Size.Width != nil && createReq.ElementProperties.Size.Width == nil {
							t.Error("expected width to be copied")
						}
						if tt.oldElement.Size.Height != nil && createReq.ElementProperties.Size.Height == nil {
							t.Error("expected height to be copied")
						}
					}
				} else if !tt.preserveSize {
					// When preserveSize=false, size should not be set
					if createReq.ElementProperties.Size != nil {
						t.Error("expected size to be nil when preserveSize=false")
					}
				}
			}
		})
	}
}

// boolPtr is a helper to create a pointer to a bool
func boolPtr(b bool) *bool {
	return &b
}

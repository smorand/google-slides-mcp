package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestGroupObjects(t *testing.T) {
	// Save and restore original time function
	originalTimeFunc := groupTimeNowFunc
	defer func() { groupTimeNowFunc = originalTimeFunc }()
	groupTimeNowFunc = func() time.Time {
		return time.Unix(1234567890, 0)
	}

	tests := []struct {
		name             string
		input            GroupObjectsInput
		mockPresentation *slides.Presentation
		mockBatchErr     error
		mockGetErr       error
		mockBatchResp    *slides.BatchUpdatePresentationResponse
		wantErr          error
		wantAction       string
		wantGroupID      string
		wantObjectIDs    []string
	}{
		{
			name: "group two objects successfully",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{"shape-1", "shape-2"},
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
							{ObjectId: "shape-2", Shape: &slides.Shape{ShapeType: "ELLIPSE"}},
						},
					},
				},
			},
			mockBatchResp: &slides.BatchUpdatePresentationResponse{
				Replies: []*slides.Response{
					{
						GroupObjects: &slides.GroupObjectsResponse{
							ObjectId: "group_1234567890000000000",
						},
					},
				},
			},
			wantAction:  "group",
			wantGroupID: "group_1234567890000000000",
		},
		{
			name: "group three objects successfully",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{"shape-1", "shape-2", "shape-3"},
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
							{ObjectId: "shape-2", Shape: &slides.Shape{ShapeType: "ELLIPSE"}},
							{ObjectId: "shape-3", Shape: &slides.Shape{ShapeType: "TRIANGLE"}},
						},
					},
				},
			},
			mockBatchResp: &slides.BatchUpdatePresentationResponse{
				Replies: []*slides.Response{
					{
						GroupObjects: &slides.GroupObjectsResponse{
							ObjectId: "group_1234567890000000000",
						},
					},
				},
			},
			wantAction:  "group",
			wantGroupID: "group_1234567890000000000",
		},
		{
			name: "ungroup successfully",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "ungroup",
				ObjectID:       "group-1",
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
										{ObjectId: "child-1"},
										{ObjectId: "child-2"},
									},
								},
							},
						},
					},
				},
			},
			wantAction:    "ungroup",
			wantObjectIDs: []string{"child-1", "child-2"},
		},
		{
			name: "uppercase action GROUP accepted",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "GROUP",
				ObjectIDs:      []string{"shape-1", "shape-2"},
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
							{ObjectId: "shape-2", Shape: &slides.Shape{ShapeType: "ELLIPSE"}},
						},
					},
				},
			},
			mockBatchResp: &slides.BatchUpdatePresentationResponse{
				Replies: []*slides.Response{
					{GroupObjects: &slides.GroupObjectsResponse{ObjectId: "group_1234567890000000000"}},
				},
			},
			wantAction:  "group",
			wantGroupID: "group_1234567890000000000",
		},
		{
			name: "uppercase action UNGROUP accepted",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "UNGROUP",
				ObjectID:       "group-1",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId:     "group-1",
								ElementGroup: &slides.Group{Children: []*slides.PageElement{{ObjectId: "child-1"}}},
							},
						},
					},
				},
			},
			wantAction:    "ungroup",
			wantObjectIDs: []string{"child-1"},
		},
		{
			name: "empty presentation_id",
			input: GroupObjectsInput{
				PresentationID: "",
				Action:         "group",
				ObjectIDs:      []string{"shape-1", "shape-2"},
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "empty action",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "",
				ObjectIDs:      []string{"shape-1", "shape-2"},
			},
			wantErr: ErrInvalidGroupAction,
		},
		{
			name: "invalid action",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "invalid_action",
				ObjectIDs:      []string{"shape-1", "shape-2"},
			},
			wantErr: ErrInvalidGroupAction,
		},
		{
			name: "group with only one object",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{"shape-1"},
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
						},
					},
				},
			},
			wantErr: ErrNotEnoughObjects,
		},
		{
			name: "group with empty object list",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{},
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides:         []*slides.Page{{ObjectId: "slide-1", PageElements: []*slides.PageElement{}}},
			},
			wantErr: ErrNotEnoughObjects,
		},
		{
			name: "group objects on different pages",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{"shape-1", "shape-2"},
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
						},
					},
					{
						ObjectId: "slide-2",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-2", Shape: &slides.Shape{ShapeType: "ELLIPSE"}},
						},
					},
				},
			},
			wantErr: ErrObjectsOnDifferentPages,
		},
		{
			name: "group object not found",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{"shape-1", "nonexistent"},
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
			wantErr: ErrObjectNotFound,
		},
		{
			name: "group object already in group",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{"grouped-shape", "shape-1"},
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
							{
								ObjectId: "group-1",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{ObjectId: "grouped-shape", Shape: &slides.Shape{ShapeType: "ELLIPSE"}},
									},
								},
							},
						},
					},
				},
			},
			wantErr: ErrCannotGroupObject,
		},
		{
			name: "group table not allowed",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{"table-1", "shape-1"},
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
							{ObjectId: "table-1", Table: &slides.Table{Rows: 2, Columns: 2}},
						},
					},
				},
			},
			wantErr: ErrCannotGroupObject,
		},
		{
			name: "group video not allowed",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{"video-1", "shape-1"},
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
							{ObjectId: "video-1", Video: &slides.Video{Id: "vid-123"}},
						},
					},
				},
			},
			wantErr: ErrCannotGroupObject,
		},
		{
			name: "group placeholder not allowed",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{"placeholder-1", "shape-1"},
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
							{ObjectId: "placeholder-1", Shape: &slides.Shape{Placeholder: &slides.Placeholder{Type: "TITLE"}}},
						},
					},
				},
			},
			wantErr: ErrCannotGroupObject,
		},
		{
			name: "ungroup empty object_id",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "ungroup",
				ObjectID:       "",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides:         []*slides.Page{},
			},
			wantErr: ErrObjectNotFound,
		},
		{
			name: "ungroup object not found",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "ungroup",
				ObjectID:       "nonexistent",
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
			wantErr: ErrObjectNotFound,
		},
		{
			name: "ungroup non-group object",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "ungroup",
				ObjectID:       "shape-1",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
						},
					},
				},
			},
			wantErr: ErrNotAGroup,
		},
		{
			name: "presentation not found",
			input: GroupObjectsInput{
				PresentationID: "nonexistent",
				Action:         "group",
				ObjectIDs:      []string{"shape-1", "shape-2"},
			},
			mockGetErr: errors.New("notFound: presentation not found"),
			wantErr:    ErrPresentationNotFound,
		},
		{
			name: "access denied",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{"shape-1", "shape-2"},
			},
			mockGetErr: errors.New("forbidden: access denied"),
			wantErr:    ErrAccessDenied,
		},
		{
			name: "group batch update fails",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "group",
				ObjectIDs:      []string{"shape-1", "shape-2"},
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
							{ObjectId: "shape-2", Shape: &slides.Shape{ShapeType: "ELLIPSE"}},
						},
					},
				},
			},
			mockBatchErr: errors.New("API error"),
			wantErr:      ErrGroupObjectsFailed,
		},
		{
			name: "ungroup batch update fails",
			input: GroupObjectsInput{
				PresentationID: "pres-1",
				Action:         "ungroup",
				ObjectID:       "group-1",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId:     "group-1",
								ElementGroup: &slides.Group{Children: []*slides.PageElement{{ObjectId: "child-1"}}},
							},
						},
					},
				},
			},
			mockBatchErr: errors.New("API error"),
			wantErr:      ErrUngroupObjectsFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track BatchUpdate requests
			var capturedRequests []*slides.Request

			// Setup mock
			mockSlides := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if tt.mockGetErr != nil {
						return nil, tt.mockGetErr
					}
					return tt.mockPresentation, nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					capturedRequests = requests
					if tt.mockBatchErr != nil {
						return nil, tt.mockBatchErr
					}
					if tt.mockBatchResp != nil {
						return tt.mockBatchResp, nil
					}
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			}

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockSlides, nil
			})

			output, err := tools.GroupObjects(context.Background(), nil, tt.input)

			// Check error
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error containing %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error containing %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check output
			if output.Action != tt.wantAction {
				t.Errorf("output.Action = %s, want %s", output.Action, tt.wantAction)
			}

			if tt.wantAction == "group" {
				if output.GroupID != tt.wantGroupID {
					t.Errorf("output.GroupID = %s, want %s", output.GroupID, tt.wantGroupID)
				}

				// Verify BatchUpdate was called with GroupObjectsRequest
				if len(capturedRequests) != 1 {
					t.Errorf("expected 1 request, got %d", len(capturedRequests))
					return
				}
				req := capturedRequests[0]
				if req.GroupObjects == nil {
					t.Error("expected GroupObjects request")
					return
				}
				if len(req.GroupObjects.ChildrenObjectIds) != len(tt.input.ObjectIDs) {
					t.Errorf("expected %d children, got %d", len(tt.input.ObjectIDs), len(req.GroupObjects.ChildrenObjectIds))
				}
			}

			if tt.wantAction == "ungroup" {
				if len(output.ObjectIDs) != len(tt.wantObjectIDs) {
					t.Errorf("output.ObjectIDs count = %d, want %d", len(output.ObjectIDs), len(tt.wantObjectIDs))
				}
				for i, oid := range output.ObjectIDs {
					if i < len(tt.wantObjectIDs) && oid != tt.wantObjectIDs[i] {
						t.Errorf("output.ObjectIDs[%d] = %s, want %s", i, oid, tt.wantObjectIDs[i])
					}
				}

				// Verify BatchUpdate was called with UngroupObjectsRequest
				if len(capturedRequests) != 1 {
					t.Errorf("expected 1 request, got %d", len(capturedRequests))
					return
				}
				req := capturedRequests[0]
				if req.UngroupObjects == nil {
					t.Error("expected UngroupObjects request")
					return
				}
				if len(req.UngroupObjects.ObjectIds) != 1 || req.UngroupObjects.ObjectIds[0] != tt.input.ObjectID {
					t.Errorf("expected object ID %s in request", tt.input.ObjectID)
				}
			}
		})
	}
}

func TestContainsObjectID(t *testing.T) {
	tests := []struct {
		name     string
		elements []*slides.PageElement
		objectID string
		want     bool
	}{
		{
			name: "found at top level",
			elements: []*slides.PageElement{
				{ObjectId: "shape-1"},
				{ObjectId: "shape-2"},
			},
			objectID: "shape-2",
			want:     true,
		},
		{
			name: "found in group",
			elements: []*slides.PageElement{
				{
					ObjectId: "group-1",
					ElementGroup: &slides.Group{
						Children: []*slides.PageElement{
							{ObjectId: "child-1"},
						},
					},
				},
			},
			objectID: "child-1",
			want:     true,
		},
		{
			name: "found in nested group",
			elements: []*slides.PageElement{
				{
					ObjectId: "group-1",
					ElementGroup: &slides.Group{
						Children: []*slides.PageElement{
							{
								ObjectId: "group-2",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{ObjectId: "deeply-nested"},
									},
								},
							},
						},
					},
				},
			},
			objectID: "deeply-nested",
			want:     true,
		},
		{
			name: "not found",
			elements: []*slides.PageElement{
				{ObjectId: "shape-1"},
			},
			objectID: "nonexistent",
			want:     false,
		},
		{
			name:     "empty elements",
			elements: []*slides.PageElement{},
			objectID: "shape-1",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsObjectID(tt.elements, tt.objectID)
			if got != tt.want {
				t.Errorf("containsObjectID() = %v, want %v", got, tt.want)
			}
		})
	}
}

# Tools Package Reference

Detailed documentation for all MCP tools in `internal/tools/`.

## Architecture

- Interface-based design: `SlidesService`, `DriveService`, `TranslateService`
- Factory pattern: `SlidesServiceFactory func(ctx, tokenSource) (SlidesService, error)`
- All tools receive `oauth2.TokenSource` from middleware context
- `SlidesService.BatchUpdate` used for modification operations

## Common Conventions

### Slide Reference (choose one)
- `slide_index` (int): 1-based index
- `slide_id` (string): Object ID

### Position/Size
- All measurements in **points** (1 point = 12700 EMU)
- Standard slide: 720 x 405 points
- Position: `{x, y}` from top-left corner

### Colors
- Hex strings: `#RRGGBB` (e.g., `#FF0000` for red)
- `"transparent"` for no fill
- Theme references: `theme:ACCENT1`

### Common Sentinel Errors
```go
ErrInvalidPresentationID  // Empty presentation ID
ErrPresentationNotFound   // 404 - presentation does not exist
ErrAccessDenied           // 403 - no permission
ErrInvalidSlideReference  // Neither slide_index nor slide_id provided
ErrSlideNotFound          // Slide index out of range or ID not found
ErrObjectNotFound         // Object ID not found
ErrSlidesAPIError         // Other Slides API errors
ErrDriveAPIError          // Drive API errors
```

---

## Presentation Tools

### get_presentation
Loads a presentation and returns its full structured content.

**Input:**
```go
GetPresentationInput{
    PresentationID:    string  // Required
    IncludeThumbnails: bool    // Optional, default false
}
```

**Output:** `PresentationID`, `Title`, `Locale`, `SlidesCount`, `PageSize`, `Slides[]`, `Masters[]`, `Layouts[]`

**SlideInfo fields:** `Index` (1-based), `ObjectID`, `LayoutID`, `LayoutName`, `TextContent[]`, `SpeakerNotes`, `ObjectCount`, `Objects[]`, `ThumbnailBase64`

---

### search_presentations
Searches for presentations in Google Drive.

**Input:**
```go
SearchPresentationsInput{
    Query:      string  // Required - search term
    MaxResults: int     // Optional, default 10, max 100
}
```

**Output:** `Presentations[]`, `TotalResults`, `Query`

**Query examples:**
- Simple: `"quarterly report"` (wrapped in fullText contains)
- By name: `"name contains 'Q4'"`
- By date: `"modifiedTime > '2024-01-01'"`

---

### copy_presentation
Copies a presentation (useful for templates).

**Input:**
```go
CopyPresentationInput{
    SourceID:            string  // Required
    NewTitle:            string  // Required
    DestinationFolderID: string  // Optional
}
```

**Output:** `PresentationID`, `Title`, `URL`, `SourceID`

---

### create_presentation
Creates a new empty presentation.

**Input:**
```go
CreatePresentationInput{
    Title:    string  // Required
    FolderID: string  // Optional - destination folder
}
```

**Output:** `PresentationID`, `Title`, `URL`, `FolderID`

---

### export_pdf
Exports presentation to PDF format.

**Input:**
```go
ExportPDFInput{
    PresentationID: string  // Required
}
```

**Output:** `PDFBase64`, `PageCount`, `FileSize`

---

## Slide Tools

### list_slides
Lists all slides with metadata and statistics.

**Input:**
```go
ListSlidesInput{
    PresentationID:    string  // Required
    IncludeThumbnails: bool    // Optional
}
```

**Output:** `PresentationID`, `Title`, `Slides[]`, `Statistics{TotalSlides, SlidesWithNotes, SlidesWithVideos}`

---

### describe_slide
Gets detailed human-readable description of a specific slide.

**Input:**
```go
DescribeSlideInput{
    PresentationID: string  // Required
    SlideIndex:     int     // 1-based (OR SlideID)
    SlideID:        string  // Alternative
}
```

**Output:** `SlideID`, `SlideIndex`, `Title`, `LayoutType`, `PageSize`, `Objects[]`, `LayoutDescription`, `ScreenshotBase64`, `SpeakerNotes`

**ObjectDescription:** `ObjectID`, `ObjectType`, `Position`, `Size`, `ContentSummary`, `ZOrder`, `Children[]`

---

### add_slide
Adds a new slide.

**Input:**
```go
AddSlideInput{
    PresentationID: string  // Required
    Position:       int     // 1-based (0 or omitted = end)
    Layout:         string  // Required - layout type
}
```

**Layouts:** `BLANK`, `CAPTION_ONLY`, `TITLE`, `TITLE_AND_BODY`, `TITLE_AND_TWO_COLUMNS`, `TITLE_ONLY`, `ONE_COLUMN_TEXT`, `MAIN_POINT`, `BIG_NUMBER`, `SECTION_HEADER`, `SECTION_TITLE_AND_DESCRIPTION`

**Output:** `SlideIndex`, `SlideID`

---

### delete_slide
Deletes a slide.

**Input:**
```go
DeleteSlideInput{
    PresentationID: string  // Required
    SlideIndex:     int     // 1-based (OR SlideID)
    SlideID:        string  // Alternative
}
```

**Output:** `DeletedSlideID`, `RemainingSlideCount`

**Note:** Cannot delete the last remaining slide (`ErrLastSlideDelete`).

---

### reorder_slides
Moves slides to new positions.

**Input:**
```go
ReorderSlidesInput{
    PresentationID: string    // Required
    SlideIndices:   []int     // 1-based (OR SlideIDs)
    SlideIDs:       []string  // Alternative
    InsertAt:       int       // 1-based target position
}
```

**Output:** `NewOrder[]` with all slides in new positions

---

### duplicate_slide
Duplicates an existing slide.

**Input:**
```go
DuplicateSlideInput{
    PresentationID: string  // Required
    SlideIndex:     int     // 1-based (OR SlideID)
    SlideID:        string  // Alternative
    InsertAt:       int     // 1-based (0 = after source)
}
```

**Output:** `SlideIndex`, `SlideID`

---

## Object Tools

### list_objects
Lists objects with optional filtering.

**Input:**
```go
ListObjectsInput{
    PresentationID: string    // Required
    SlideIndices:   []int     // Optional - 1-based, all slides if omitted
    ObjectTypes:    []string  // Optional filter
}
```

**Object types:** `TEXT_BOX`, `RECTANGLE`, `ELLIPSE`, `IMAGE`, `VIDEO`, `TABLE`, `LINE`, `GROUP`, `SHEETS_CHART`, `WORD_ART`

**Output:** `Objects[]`, `TotalCount`, `FilteredBy`

---

### get_object
Gets detailed information about an object.

**Input:**
```go
GetObjectInput{
    PresentationID: string  // Required
    ObjectID:       string  // Required
}
```

**Output:** Common fields + type-specific details:
- **Shapes:** `ShapeType`, `Text`, `TextStyle`, `Fill`, `Outline`, `PlaceholderType`
- **Images:** `ContentURL`, `SourceURL`, `Brightness`, `Contrast`, `Transparency`, `Recolor`, `Crop`
- **Tables:** `Rows`, `Columns`, `Cells[][]`
- **Videos:** `VideoID`, `Source` (YOUTUBE/DRIVE), `URL`, `StartTime`, `EndTime`, `Autoplay`, `Mute`
- **Lines:** `LineType`, `StartArrow`, `EndArrow`, `Color`, `Weight`, `DashStyle`
- **Groups:** `ChildCount`, `ChildIDs[]`

---

### delete_object
Deletes one or more objects.

**Input:**
```go
DeleteObjectInput{
    PresentationID: string    // Required
    ObjectID:       string    // Single ID (OR Multiple)
    Multiple:       []string  // Multiple IDs (can combine with ObjectID)
}
```

**Notes:**
- Both `ObjectID` and `Multiple` can be used together (all unique IDs deleted)
- Partial success: deletes found objects, reports not found IDs separately
- Recursively finds objects anywhere (slides, masters, groups)

**Output:** `DeletedCount`, `DeletedIDs[]`, `NotFoundIDs[]` (optional)

---

### transform_object
Moves, resizes, or rotates an object.

**Input:**
```go
TransformObjectInput{
    PresentationID: string          // Required
    ObjectID:       string          // Required
    Position:       *PositionInput  // Optional {X, Y} in points
    Size:           *SizeInput      // Optional {Width, Height} in points
    Rotation:       *float64        // Optional - degrees clockwise
}
```

**Output:** `ObjectID`, `AppliedTransforms[]`

---

### change_z_order
Changes object layering (front/back).

**Input:**
```go
ChangeZOrderInput{
    PresentationID: string  // Required
    ObjectID:       string  // Required
    Operation:      string  // Required: "bring_to_front", "send_to_back", "bring_forward", "send_backward"
}
```

**Output:** `ObjectID`, `Operation`, `NewZOrder`

---

### group_objects
Groups or ungroups objects.

**Input:**
```go
GroupObjectsInput{
    PresentationID: string    // Required
    Action:         string    // Required: "group" or "ungroup"
    ObjectIDs:      []string  // Required for group - minimum 2 objects
    GroupID:        string    // Required for ungroup - existing group ID
}
```

**Notes:**
- Cannot group: tables, videos, placeholder shapes, objects already in groups
- All objects to group must be on the same slide

**Output:** `GroupID` (for group) or `ChildIDs[]` (for ungroup)

---

## Text Tools

### add_text_box
Adds a text box with optional styling.

**Input:**
```go
AddTextBoxInput{
    PresentationID: string           // Required
    SlideIndex:     int              // 1-based (OR SlideID)
    SlideID:        string           // Alternative
    Text:           string           // Required
    Position:       *PositionInput   // Optional {X, Y}
    Size:           *SizeInput       // Required {Width, Height}
    Style:          *TextStyleInput  // Optional
}
```

**TextStyleInput:** `FontFamily`, `FontSize`, `Bold`, `Italic`, `Color`

**Output:** `ObjectID`

---

### modify_text
Modifies text content in an existing shape.

**Input:**
```go
ModifyTextInput{
    PresentationID: string  // Required
    ObjectID:       string  // Required
    Action:         string  // Required: "replace", "append", "prepend", "delete"
    Text:           string  // Required for replace/append/prepend
    StartIndex:     *int    // Optional - for partial replacement
    EndIndex:       *int    // Optional - for partial replacement
}
```

**Output:** `ObjectID`, `UpdatedText`, `Action`

---

### style_text
Applies styling to text.

**Input:**
```go
StyleTextInput{
    PresentationID: string             // Required
    ObjectID:       string             // Required
    StartIndex:     *int               // Optional range
    EndIndex:       *int               // Optional range
    Style:          *StyleTextStyleSpec // Required
}
```

**StyleTextStyleSpec:** `FontFamily`, `FontSize`, `Bold*`, `Italic*`, `Underline*`, `Strikethrough*`, `ForegroundColor`, `BackgroundColor`, `LinkURL`

*Note: Boolean properties use pointers to distinguish false from unset.

**Output:** `ObjectID`, `AppliedStyles[]`, `TextRange`

---

### format_paragraph
Sets paragraph formatting (alignment, spacing, indentation).

**Input:**
```go
FormatParagraphInput{
    PresentationID: string                     // Required
    ObjectID:       string                     // Required
    ParagraphIndex: *int                       // Optional - 0-based, all if omitted
    Formatting:     *ParagraphFormattingOptions // Required
}
```

**ParagraphFormattingOptions:** `Alignment` (START/CENTER/END/JUSTIFIED), `LineSpacing*`, `SpaceAbove*`, `SpaceBelow*`, `IndentFirstLine*`, `IndentStart*`, `IndentEnd*`

**Output:** `ObjectID`, `AppliedFormatting[]`, `ParagraphScope`

---

### search_text
Searches for text across all slides.

**Input:**
```go
SearchTextInput{
    PresentationID: string  // Required
    Query:          string  // Required
    CaseSensitive:  bool    // Optional, default false
}
```

**Output:** `Query`, `TotalMatches`, `Results[]` (grouped by slide with `ObjectID`, `ObjectType`, `StartIndex`, `TextContext`)

---

### replace_text
Finds and replaces text.

**Input:**
```go
ReplaceTextInput{
    PresentationID: string  // Required
    Find:           string  // Required
    ReplaceWith:    string  // Required (empty to delete)
    CaseSensitive:  bool    // Optional
    Scope:          string  // Optional: "all", "slide", "object"
    SlideID:        string  // Required when scope="slide"
    ObjectID:       string  // Required when scope="object"
}
```

**Output:** `ReplacementCount`, `AffectedObjects[]`

---

## List Tools

### create_bullet_list
Converts text to bullet list.

**Input:**
```go
CreateBulletListInput{
    PresentationID:   string  // Required
    ObjectID:         string  // Required
    ParagraphIndices: []int   // Optional - 0-based, all if omitted
    BulletStyle:      string  // Required
    BulletColor:      string  // Optional hex
}
```

**Bullet styles:** `DISC`, `CIRCLE`, `SQUARE`, `DIAMOND`, `ARROW`, `STAR`, `CHECKBOX`

---

### create_numbered_list
Converts text to numbered list.

**Input:**
```go
CreateNumberedListInput{
    PresentationID:   string  // Required
    ObjectID:         string  // Required
    ParagraphIndices: []int   // Optional
    NumberStyle:      string  // Required
    StartNumber:      int     // Optional, default 1
}
```

**Number styles:** `DECIMAL`, `ALPHA_UPPER`, `ALPHA_LOWER`, `ROMAN_UPPER`, `ROMAN_LOWER`

---

### modify_list
Modifies list properties or removes list formatting.

**Input:**
```go
ModifyListInput{
    PresentationID:   string               // Required
    ObjectID:         string               // Required
    Action:           string               // Required: "modify", "remove", "increase_indent", "decrease_indent"
    ParagraphIndices: []int                // Optional
    Properties:       *ListModifyProperties // Required for "modify"
}
```

---

## Image Tools

### add_image
Adds an image from base64 data.

**Input:**
```go
AddImageInput{
    PresentationID: string          // Required
    SlideIndex:     int             // 1-based (OR SlideID)
    SlideID:        string          // Alternative
    ImageBase64:    string          // Required
    Position:       *PositionInput  // Optional
    Size:           *ImageSizeInput // Optional {Width*, Height*}
}
```

**Notes:**
- Auto-detects MIME type (PNG, JPEG, GIF, WebP, BMP)
- Uploads to Drive, then references in Slides
- If only width or height provided, aspect ratio preserved

---

### modify_image
Modifies image properties.

**Input:**
```go
ModifyImageInput{
    PresentationID: string          // Required
    ObjectID:       string          // Required
    Position:       *PositionInput  // Optional
    Size:           *SizeInput      // Optional
    Brightness:     *float64        // Optional -1.0 to 1.0
    Contrast:       *float64        // Optional -1.0 to 1.0
    Transparency:   *float64        // Optional 0.0 to 1.0
    Recolor:        string          // Optional: GRAYSCALE, SEPIA, etc.
    CropRect:       *CropRect       // Optional {Top, Bottom, Left, Right}
}
```

---

### replace_image
Replaces an image preserving its transform.

**Input:**
```go
ReplaceImageInput{
    PresentationID: string  // Required
    ObjectID:       string  // Required
    ImageBase64:    string  // Required
}
```

---

## Video Tools

### add_video
Adds a YouTube or Drive video.

**Input:**
```go
AddVideoInput{
    PresentationID: string          // Required
    SlideIndex:     int             // 1-based (OR SlideID)
    SlideID:        string          // Alternative
    VideoSource:    string          // Required: "youtube" or "drive"
    VideoID:        string          // Required - YouTube ID or Drive file ID
    Position:       *PositionInput  // Optional
    Size:           *SizeInput      // Optional (default 400x225)
}
```

---

### modify_video
Modifies video properties.

**Input:**
```go
ModifyVideoInput{
    PresentationID: string          // Required
    ObjectID:       string          // Required
    Position:       *PositionInput  // Optional
    Size:           *SizeInput      // Optional
    StartTime:      *float64        // Optional - seconds
    EndTime:        *float64        // Optional - seconds
    Autoplay:       *bool           // Optional
    Mute:           *bool           // Optional
}
```

---

## Shape Tools

### create_shape
Creates a shape with optional fill and outline.

**Input:**
```go
CreateShapeInput{
    PresentationID: string          // Required
    SlideIndex:     int             // 1-based (OR SlideID)
    SlideID:        string          // Alternative
    ShapeType:      string          // Required
    Position:       *PositionInput  // Required
    Size:           *SizeInput      // Required
    Fill:           *ShapeFill      // Optional
    Outline:        *ShapeOutline   // Optional
}
```

**Shape types:** `RECTANGLE`, `ROUND_RECTANGLE`, `ELLIPSE`, `TRIANGLE`, `DIAMOND`, `STAR_5`, `ARROW_RIGHT`, `ARROW_LEFT`, `CLOUD_CALLOUT`, `HEART`, `LIGHTNING_BOLT`, and many more...

**ShapeFill:** `Color` (hex or "transparent"), `Transparency` (0-1)

**ShapeOutline:** `Color`, `Weight` (points), `DashStyle` (SOLID, DOT, DASH, etc.)

---

### modify_shape
Modifies shape properties.

**Input:**
```go
ModifyShapeInput{
    PresentationID: string         // Required
    ObjectID:       string         // Required
    Fill:           *ShapeFill     // Optional
    Outline:        *ShapeOutline  // Optional
    Shadow:         *ShapeShadow   // Optional
}
```

---

### create_line
Creates a line or arrow.

**Input:**
```go
CreateLineInput{
    PresentationID: string           // Required
    SlideIndex:     int              // 1-based (OR SlideID)
    SlideID:        string           // Alternative
    StartPoint:     *PositionInput   // Required {X, Y}
    EndPoint:       *PositionInput   // Required {X, Y}
    LineType:       string           // Optional: "STRAIGHT", "CURVED", "ELBOW"
    StartArrow:     string           // Optional: "NONE", "ARROW", "OPEN_ARROW", etc.
    EndArrow:       string           // Optional
    Color:          string           // Optional hex
    Weight:         float64          // Optional - points
    DashStyle:      string           // Optional: "SOLID", "DOT", "DASH"
}
```

---

## Table Tools

### create_table
Creates a table.

**Input:**
```go
CreateTableInput{
    PresentationID: string          // Required
    SlideIndex:     int             // 1-based (OR SlideID)
    SlideID:        string          // Alternative
    Rows:           int             // Required
    Columns:        int             // Required
    Position:       *PositionInput  // Optional
    Size:           *SizeInput      // Optional
}
```

---

### modify_table_structure
Adds or deletes rows/columns.

**Input:**
```go
ModifyTableStructureInput{
    PresentationID: string  // Required
    ObjectID:       string  // Required - table ID
    Action:         string  // Required: "add_row", "delete_row", "add_column", "delete_column"
    Index:          int     // Required - 0-based position
    Count:          int     // Optional - default 1
}
```

---

### merge_cells
Merges or unmerges table cells.

**Input:**
```go
MergeCellsInput{
    PresentationID: string  // Required
    ObjectID:       string  // Required - table ID
    Action:         string  // Required: "merge" or "unmerge"
    StartRow:       int     // Required - 0-based
    StartColumn:    int     // Required - 0-based
    RowSpan:        int     // Required for merge
    ColumnSpan:     int     // Required for merge
}
```

---

### modify_table_cell
Sets cell text, style, or alignment.

**Input:**
```go
ModifyTableCellInput{
    PresentationID: string           // Required
    ObjectID:       string           // Required - table ID
    Row:            int              // Required - 0-based
    Column:         int              // Required - 0-based
    Text:           string           // Optional
    TextStyle:      *TextStyleInput  // Optional
    Alignment:      string           // Optional: "START", "CENTER", "END"
}
```

---

### style_table_cells
Styles table cells (background, borders).

**Input:**
```go
StyleTableCellsInput{
    PresentationID:  string           // Required
    ObjectID:        string           // Required - table ID
    CellRange:       *TableCellRange  // Required {StartRow, EndRow, StartColumn, EndColumn}
    BackgroundColor: string           // Optional hex
    BorderStyle:     *BorderStyle     // Optional {Color, Weight, DashStyle, Sides[]}
}
```

---

## Theme & Background Tools

### apply_theme
Copies theme from another presentation.

**Input:**
```go
ApplyThemeInput{
    PresentationID:       string  // Required - target
    SourcePresentationID: string  // Required - source with theme
}
```

---

### set_background
Sets slide background.

**Input:**
```go
SetBackgroundInput{
    PresentationID: string           // Required
    SlideIndex:     int              // 1-based (OR SlideID)
    SlideID:        string           // Alternative
    BackgroundType: string           // Required: "solid", "image", "gradient"
    Color:          string           // For solid - hex
    ImageBase64:    string           // For image
    GradientColors: []GradientStop   // For gradient
}
```

---

### configure_footer
Configures slide footer (numbers, date, text).

**Input:**
```go
ConfigureFooterInput{
    PresentationID:    string  // Required
    SlideIndex:        int     // 1-based (OR SlideID, or omit for all)
    SlideID:           string  // Alternative
    ShowSlideNumber:   *bool   // Optional
    ShowDatetime:      *bool   // Optional
    DatetimeFormat:    string  // Optional
    ShowFooterText:    *bool   // Optional
    FooterText:        string  // Optional
}
```

---

## Comment Tools

### list_comments
Lists all comments on a presentation.

**Input:**
```go
ListCommentsInput{
    PresentationID: string  // Required
}
```

**Output:** `Comments[]` with `CommentID`, `Author`, `Content`, `CreatedTime`, `ModifiedTime`, `Resolved`, `Replies[]`, `Anchor`

---

### add_comment
Adds a comment via Drive API.

**Input:**
```go
AddCommentInput{
    PresentationID:  string  // Required
    Content:         string  // Required
    AnchorObjectID:  string  // Optional - anchor to specific object
    AnchorPageIndex: int     // Optional - anchor to specific slide (0-based)
}
```

**Anchor Behavior:**
- If `AnchorObjectID` provided: anchored to that object
- If only `AnchorPageIndex` provided: anchored to that slide
- If both provided: `AnchorObjectID` takes precedence
- Page index is 0-based in input, converted to 1-based for API

**Output:** `CommentID`, `Content`, `AnchorInfo` (JSON), `CreatedTime`

---

### manage_comment
Manages existing comments (reply, resolve, delete).

**Input:**
```go
ManageCommentInput{
    PresentationID: string  // Required
    CommentID:      string  // Required
    Action:         string  // Required: "reply", "resolve", "unresolve", "delete"
    ReplyContent:   string  // Required for "reply"
}
```

---

## Other Tools

### manage_speaker_notes
Gets, sets, appends, or clears speaker notes.

**Input:**
```go
ManageSpeakerNotesInput{
    PresentationID: string  // Required
    SlideIndex:     int     // 1-based (OR SlideID)
    SlideID:        string  // Alternative
    Action:         string  // Required: "get", "set", "append", "clear"
    Notes:          string  // Required for "set", "append"
}
```

---

### manage_hyperlinks
Lists, adds, or removes hyperlinks.

**Input:**
```go
ManageHyperlinksInput{
    PresentationID: string  // Required
    Action:         string  // Required: "list", "add", "remove"
    Scope:          string  // Optional for list: "all", "slide", "object"
    SlideIndex:     int     // For scope="slide" (1-based)
    SlideID:        string  // Alternative to SlideIndex
    ObjectID:       string  // Required for add/remove, optional for scope="object"
    URL:            string  // Required for add
    StartIndex:     *int    // Optional for add - text range
    EndIndex:       *int    // Optional for add - text range
}
```

**URL Formats for Internal Links:**
- `#slide=N` - 1-based slide index
- `#slideId=ID` - Slide object ID
- `#next`, `#previous`, `#first`, `#last` - Relative navigation

**Output:** For list: `Hyperlinks[]` with `ObjectID`, `URL`, `LinkType` (external/internal_slide/internal_position)

---

### translate_presentation
Translates text using Cloud Translation API.

**Input:**
```go
TranslatePresentationInput{
    PresentationID: string    // Required
    TargetLanguage: string    // Required - ISO code (e.g., "fr", "es", "de")
    SourceLanguage: string    // Optional - auto-detect if omitted
    SlideIndices:   []int     // Optional - 1-based, all if omitted
}
```

---

### batch_update
Executes multiple operations efficiently in a single API call.

**Input:**
```go
BatchUpdateInput{
    PresentationID: string           // Required
    Operations:     []BatchOperation // Required
    OnError:        string           // Optional: "stop" (default), "continue", "rollback"
}
```

**BatchOperation:** `ToolName` (string), `Parameters` (tool-specific parameters as JSON object)

**Supported Batchable Tools:**
- `add_slide`, `delete_slide`, `add_text_box`, `modify_text`, `delete_object`
- `create_shape`, `transform_object`, `style_text`, `create_bullet_list`, `create_numbered_list`

**Non-Batchable Tools** (require separate API calls):
- `add_image`, `add_video`, `replace_image`, `set_background`, `translate_presentation`

**On Error Modes:**
| Mode | Behavior |
|------|----------|
| `stop` | Halt on first error (default) |
| `continue` | Process all operations, report failures |
| `rollback` | Atomic - all succeed or all fail |

**Output:** `Results[]` with `Success`, `ToolName`, `Error` for each operation

---

## Unsupported Operations

These operations are not supported by the Google Slides API:

| Tool | Status | Alternative |
|------|--------|-------------|
| `set_transition` | API limitation | Use Slides UI |
| `add_animation` | API limitation | Use Slides UI |
| `manage_animations` | API limitation | Use Slides UI |

---

## Service Interfaces

### SlidesService
```go
type SlidesService interface {
    GetPresentation(ctx context.Context, id string) (*slides.Presentation, error)
    BatchUpdate(ctx context.Context, id string, req *slides.BatchUpdatePresentationRequest) (*slides.BatchUpdatePresentationResponse, error)
    GetThumbnail(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error)
}
```

### DriveService
```go
type DriveService interface {
    Search(ctx context.Context, query string, maxResults int) ([]*drive.File, error)
    Copy(ctx context.Context, fileID string, name string, parentID string) (*drive.File, error)
    Export(ctx context.Context, fileID string, mimeType string) ([]byte, error)
    Get(ctx context.Context, fileID string) (*drive.File, error)
    CreateFile(ctx context.Context, name string, mimeType string, content []byte, parentID string) (*drive.File, error)
    UpdatePermissions(ctx context.Context, fileID string, perms *drive.Permission) error
    ListComments(ctx context.Context, fileID string) ([]*drive.Comment, error)
    CreateComment(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error)
    // ... more methods
}
```

### TranslateService
```go
type TranslateService interface {
    Translate(ctx context.Context, text string, targetLang string, sourceLang string) (string, error)
    DetectLanguage(ctx context.Context, text string) (string, error)
}
```

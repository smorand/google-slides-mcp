## 2026-01-15 - US-00038 - Implement tool to move/resize object

**Status:** Success

**What was implemented:**
- New `transform_object` MCP tool to move, resize, and rotate objects
- Complex affine transform decomposition and recomposition logic
- Support for absolute positioning and rotation
- Proportional vs non-proportional scaling logic
- Uses `UpdatePageElementTransformRequest` with ABSOLUTE mode
- Comprehensive test suite with 5 test cases including rotation math verification

**Files changed:**
- `internal/tools/transform_object.go` - Tool implementation
- `internal/tools/transform_object_test.go` - Comprehensive tests
- `CLAUDE.md` - Added transform_object documentation
- `README.md` - Added transform_object tool documentation
- `stories.yaml` - Marked US-00038 as passes: true

**Learnings:**
- Google Slides API uses an `AffineTransform` matrix for positioning and rotation
- To resize an object, we must modify the `ScaleX` and `ScaleY` components of the transform, relative to the object's base size
- Rotation is encoded in the transform matrix via sine/cosine components in Scale/Shear fields
- Decomposing the matrix allows updating individual properties (position, rotation, scale) while preserving others
- Proportional scaling requires careful handling of original scale factors to maintain aspect ratio

**Remaining issues:** None

---

## 2026-01-16 - US-00042 - Implement tool to create table

**Status:** Success

**What was implemented:**
- New `create_table` MCP tool to create tables on slides
- CreateTableInput struct with: presentation_id, slide_index/slide_id, rows, columns, position, size
- CreateTableOutput struct with: object_id, rows, columns
- Uses `CreateTableRequest` in Google Slides API BatchUpdate
- Support for optional position and size (in points, converted to EMU)
- Comprehensive test suite with 20 test cases

**Files changed:**
- `internal/tools/create_table.go` - Tool implementation
- `internal/tools/create_table_test.go` - Comprehensive tests
- `CLAUDE.md` - Added create_table documentation
- `README.md` - Added create_table tool documentation
- `stories.yaml` - Marked US-00042 as passes: true

**Learnings:**
- CreateTableRequest in Google Slides API requires rows and columns as int64
- Position is specified via AffineTransform with translateX/translateY in EMU
- Size is specified via Size with Width/Height Dimensions in EMU
- Table object ID is generated using timestamp-based pattern like other tools
- Test patterns from create_shape_test.go can be reused for similar element creation tools

**Remaining issues:** None

---
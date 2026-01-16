package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for transform_object tool.
var (
	ErrTransformFailed = errors.New("failed to transform object")
)

// TransformObjectInput represents the input for the transform_object tool.
type TransformObjectInput struct {
	PresentationID      string         `json:"presentation_id"`
	ObjectID            string         `json:"object_id"`
	Position            *PositionInput `json:"position,omitempty"`
	Size                *SizeInput     `json:"size,omitempty"`
	Rotation            *float64       `json:"rotation,omitempty"` // Degrees 0-360
	ScaleProportionally bool           `json:"scale_proportionally,omitempty"` // Default true
}

// TransformObjectOutput represents the output of the transform_object tool.
type TransformObjectOutput struct {
	Position *Position `json:"position"`
	Size     *Size     `json:"size"`
	Rotation float64   `json:"rotation"`
}

// TransformObject moves, resizes, or rotates an object.
func (t *Tools) TransformObject(ctx context.Context, tokenSource oauth2.TokenSource, input TransformObjectInput) (*TransformObjectOutput, error) {
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrObjectNotFound)
	}

	t.config.Logger.Info("transforming object",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// 1. Fetch current object to get existing transform
	presentation, err := slidesService.GetPresentation(ctx, input.PresentationID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrSlidesAPIError, err)
	}

	element := findElementByIDRecursively(presentation.Slides, input.ObjectID)
	if element == nil {
		return nil, fmt.Errorf("%w: object '%s' not found", ErrObjectNotFound, input.ObjectID)
	}

	currentTransform := element.Transform
	if currentTransform == nil {
		// Default transform if missing
		currentTransform = &slides.AffineTransform{
			ScaleX:     1,
			ScaleY:     1,
			TranslateX: 0,
			TranslateY: 0,
			Unit:       "EMU",
		}
	}

	currentSize := element.Size
	if currentSize == nil || currentSize.Width == nil || currentSize.Height == nil {
		// Cannot resize if size is missing (e.g. lines sometimes)
		// But lines use Transform for size? No, Lines use element properties Size + Transform.
		// Some elements might lack size.
		if input.Size != nil {
			// Try to proceed, but might fail if original size unknown for proportional scaling
		}
	}

	// 2. Calculate new transform and size
	newTransform, newSize, newRotation, err := calculateNewTransform(currentTransform, currentSize, input)
	if err != nil {
		return nil, err
	}

	// 3. Apply updates
	// For size changes on non-group objects, we might use UpdatePageElementTransformRequest
	// which applies an affine transform.
	// The Slides API documentation says:
	// "The size of the element is derived from the size of the bounding box of the transformed element."
	// Actually, changing size usually implies changing the Scale factors in the transform, OR using UpdatePageElementTransformRequest with ABSOLUTE mode.
	
	req := &slides.Request{
		UpdatePageElementTransform: &slides.UpdatePageElementTransformRequest{
			ObjectId:     input.ObjectID,
			Transform:    newTransform,
			ApplyMode:    "ABSOLUTE",
		},
	}

	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, []*slides.Request{req})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransformFailed, err)
	}

	// Calculate output properties from the new transform/size
	output := &TransformObjectOutput{
		Position: &Position{
			X: emuToPoints(newTransform.TranslateX),
			Y: emuToPoints(newTransform.TranslateY),
		},
		Size: &Size{
			Width:  convertToPoints(newSize.Width),
			Height: convertToPoints(newSize.Height),
		},
		Rotation: newRotation,
	}

	return output, nil
}

func findElementByIDRecursively(slidesList []*slides.Page, objectID string) *slides.PageElement {
	for _, slide := range slidesList {
		if found := findElementByID(slide.PageElements, objectID); found != nil {
			return found
		}
	}
	return nil
}

// calculateNewTransform computes the new AffineTransform and Size based on input.
//
// Google Slides Transform Matrix (3x3 implied):
// [ ScaleX  ShearX  TranslateX ]
// [ ShearY  ScaleY  TranslateY ]
// [   0       0         1      ]
//
// Rotation implies:
// ScaleX = Sx * cos(theta)
// ShearX = -Sy * sin(theta)
// ShearY = Sx * sin(theta)
// ScaleY = Sy * cos(theta)
//
// Note: Google Slides seems to handle size updates by modifying the Scale components of the transform
// relative to the "inherent" size of the element, OR by updating the size property directly?
// UpdatePageElementTransformRequest documentation:
// "Updates the transform of a page element. Updating the transform of a group will change the absolute transform of the page elements in that group..."
// It doesn't explicitly accept a "Size". Size is a property of the element.
// 
// Wait, CreateShape allows setting Size. But UpdatePageElementTransform is for the matrix.
// Does resizing an element change its Size property or its Transform scale?
// Usually, for Shapes/Images, Size is the un-transformed size, and Transform puts it on the page.
// BUT, often "Size" in the UI updates the underlying width/height if it's a shape, OR updates scale.
//
// Actually, creating a request with `UpdatePageElementTransform` is the standard way to move/resize/rotate.
// If we change size, we effectively calculate new ScaleX/ScaleY factors based on the ratio: NewSize / OriginalSize (assuming we keep original element size constant and just scale it).
// OR we can't change the base Size of an existing element easily? 
// No, the `Size` field in PageElement is output only for some types?
// Let's assume we treat the `Size` in the API response as the visual size *after* transform?
// API Docs: "Size of the page element. This property is read-only." -> So we MUST use Transform to resize.
//
// So: VisualWidth = BaseWidth * ScaleX (roughly, with rotation mixing it up).
// 
// Strategy:
// 1. Decompose current transform to get current Translation, Scale, Rotation/Shear.
// 2. Apply requested changes.
// 3. Recompose transform.
//
// Limitation: Decomposing a general affine matrix is hard. But Slides usually maintains no skew/shear other than rotation.
// We assume ScaleX, ScaleY, Rotation.
func calculateNewTransform(current *slides.AffineTransform, currentSize *slides.Size, input TransformObjectInput) (*slides.AffineTransform, *slides.Size, float64, error) {
	// 1. Decompose current transform
	// Extract translation
	tx := current.TranslateX
	ty := current.TranslateY

	// Extract rotation and scale
	// Sx = sqrt(ScaleX^2 + ShearY^2)
	// Sy = sqrt(ScaleY^2 + ShearX^2)
	// Rotation = atan2(ShearY, ScaleX)
	
	sx := math.Sqrt(current.ScaleX*current.ScaleX + current.ShearY*current.ShearY)
	sy := math.Sqrt(current.ScaleY*current.ScaleY + current.ShearX*current.ShearX)
	
	// Rotation in radians
	currentAngle := math.Atan2(current.ShearY, current.ScaleX)
	
	// 2. Apply updates
	
	// Position
	if input.Position != nil {
		tx = pointsToEMU(input.Position.X)
		ty = pointsToEMU(input.Position.Y)
	}

	// Rotation
	newAngle := currentAngle
	if input.Rotation != nil {
		// Input is degrees, convert to radians
		newAngle = *input.Rotation * math.Pi / 180.0
	}

	// Size (Scaling)
	// We modify sx and sy.
	// Current visual width/height (approximate if rotated, but let's talk about the element's local frame size)
	// VisualWidth = BaseWidth * sx
	// VisualHeight = BaseHeight * sy
	// If input provides new Width/Height (in points), we need BaseWidth/BaseHeight to calc new sx/sy.
	// Since Size is read-only, we assume currentSize represents BaseSize?
	// API Docs: "The size of the page element." 
	// Actually, `Size` returned by API is usually the bounding box size *without* rotation? 
	// Or is it the base size?
	// Let's assume `Size` + `Transform` determines visual appearance.
	// So `Size` provided in `element` is the base size $W_{base}, H_{base}$.
	// We want new visual size $W_{new}, H_{new}$.
	// New $s_x = W_{new} / W_{base}$.
	
	if input.Size != nil {
		if currentSize == nil || currentSize.Width == nil || currentSize.Height == nil {
			// Fallback: If we can't get base size, we can't resize accurately unless we assume current scale is 1?
			// Or we just fail.
			return nil, nil, 0, errors.New("cannot resize object with unknown base size")
		}

		baseWidth := currentSize.Width.Magnitude // In EMU? Size struct has Unit.
		if currentSize.Width.Unit == "PT" {
			baseWidth = pointsToEMU(baseWidth)
		}
		
		baseHeight := currentSize.Height.Magnitude
		if currentSize.Height.Unit == "PT" {
			baseHeight = pointsToEMU(baseHeight)
		}

		originalSx := sx
		originalSy := sy

		if input.Size.Width > 0 {
			targetWidthEMU := pointsToEMU(input.Size.Width)
			sx = targetWidthEMU / baseWidth
			
			if input.Size.Height <= 0 && input.ScaleProportionally {
				if originalSx != 0 {
					sy = originalSy * (sx / originalSx)
				} else {
					sy = sx
				}
			}
		}

		if input.Size.Height > 0 {
			targetHeightEMU := pointsToEMU(input.Size.Height)
			
			// If Width was also set, we just set Height independently
			// If Width NOT set, we calculate sy, and maybe sx.
			if input.Size.Width <= 0 {
				sy = targetHeightEMU / baseHeight
				
				if input.ScaleProportionally {
					if originalSy != 0 {
						sx = originalSx * (sy / originalSy)
					} else {
						sx = sy
					}
				}
			} else {
				// Both set - strictly apply
				sy = targetHeightEMU / baseHeight
			}
		}
	}

	// 3. Recompose transform matrix
	// [ sx*cos(a)   -sy*sin(a)   tx ]
	// [ sx*sin(a)    sy*cos(a)   ty ]
	
	cosA := math.Cos(newAngle)
	sinA := math.Sin(newAngle)

	newTransform := &slides.AffineTransform{
		ScaleX:     sx * cosA,
		ShearY:     sx * sinA, // Google's ShearY corresponds to (1,0) mapping to (ScaleX, ShearY)
		ShearX:     -sy * sinA, // Google's ShearX corresponds to (0,1) mapping to (ShearX, ScaleY)
		ScaleY:     sy * cosA,
		TranslateX: tx,
		TranslateY: ty,
		Unit:       "EMU",
	}

	// Calculated visual size for output
	var visualWidth, visualHeight float64
	if currentSize != nil {
		// Use magnitude directly as we are multiplying by scale factor
		bw := currentSize.Width.Magnitude
		bh := currentSize.Height.Magnitude
		if currentSize.Width.Unit == "PT" { bw = pointsToEMU(bw) }
		if currentSize.Height.Unit == "PT" { bh = pointsToEMU(bh) }
		
		visualWidth = bw * sx
		visualHeight = bh * sy
	}

	newSize := &slides.Size{
		Width:  &slides.Dimension{Magnitude: visualWidth, Unit: "EMU"},
		Height: &slides.Dimension{Magnitude: visualHeight, Unit: "EMU"},
	}

	return newTransform, newSize, newAngle * 180.0 / math.Pi, nil
}

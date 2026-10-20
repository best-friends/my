// SPDX-License-Identifier: AGPL-3.0-only

package content

// ImageVariant is a rendition of an image as clients load it. ContentType, Width, Height, and
// Size are zero when unknown, as for the media proxy URL of a remote file.
type ImageVariant struct {
	URL           string
	ContentType   string
	Width, Height int
	Size          int64
}

// ImageVariants are the renditions of an image in one motion, by size (ADR-0054 §1).
type ImageVariants struct {
	Small, Medium, Large ImageVariant
}

// Image is a media file as clients load it: its still renditions, and for an animation also
// its moving ones (ADR-0054 §3, §5).
type Image struct {
	Still ImageVariants
	// Animated are the moving renditions, nil for a still image.
	Animated *ImageVariants
}

// Primary returns the rendition served as the file itself, the large one: moving for an
// animation, still otherwise.
func (i Image) Primary() ImageVariant {
	if i.Animated != nil {
		return i.Animated.Large
	}
	return i.Still.Large
}

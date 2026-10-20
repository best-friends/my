// SPDX-License-Identifier: AGPL-3.0-only
// The renditions of a media file, as @my/ui's Picture chooses among them (ADR-0054 §1, §5). The types have the
// shape of Picture's ImageSources; lib does not import the kit (web-client.md A1–A3).
import type { components } from "./api/schema.gen";

type ImageCandidate = { url: string; width?: number | undefined };
type ImageSources = { still: readonly ImageCandidate[]; animated?: readonly ImageCandidate[] | undefined };

type MediaVariants = components["schemas"]["MediaVariants"];
type MediaVariant = components["schemas"]["MediaVariant"];

function candidate(variant: MediaVariant): ImageCandidate {
  return variant.width === undefined ? { url: variant.url } : { url: variant.url, width: variant.width };
}

/**
 * The renditions of an image (an avatar, a header, an attachment), smallest first, with its moving ones for an
 * animation; undefined for no image.
 */
export function imageSources(variants: MediaVariants | undefined): ImageSources | undefined {
  if (variants === undefined) return undefined;
  const still = [variants.small, variants.medium, variants.large].map(candidate);
  const animated = variants.animated;
  if (animated === undefined) return { still };
  return { still, animated: [animated.small, animated.medium, animated.large].map(candidate) };
}

An image in the rendition that fits, with animation that moves only when the reader allows it. **@my/ui:** `Picture` — `app/ui/src/Picture.tsx`.

**Consumer provides:** `sources` (`still` and, for an animation, `animated` renditions with widths), `sizes`, `alt`, and the usual `<img>` props. The app sets `AutoplayContext` from the 「アニメーションを自動再生」 setting.

- A still image is a plain `<img>` (with `srcset` when every rendition has a width).
- Autoplay on: a `<picture>` whose moving source applies only under `prefers-reduced-motion: no-preference`.
- Autoplay off: the still rendition, replaced by the moving one while the pointer is over it.
- Brand framing: pictures in posts sit in 16px-radius frames (MediaGrid, LinkCard); imagery is warm daylight, as the brand's soft gradient placeholders show.

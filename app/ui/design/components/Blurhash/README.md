The blur of an image, drawn from its blurhash, standing in while the image is concealed or loading. **@my/ui:** `Blurhash` — `app/ui/src/Blurhash.tsx`.

**Consumer provides:** `hash` (`MediaAttachment.blurhash`) and a sized container; it fills the container.

- Decoded to 32×32 pixels on a canvas and scaled smoothly. Decorative (`aria-hidden`); the image it stands for carries the name.
- An invalid hash draws nothing, so the container's `surface-sunken` shows through — that is the fallback for media without a blurhash too.
- Used by MediaGrid and LinkCard; it stays under the image until the image has loaded.

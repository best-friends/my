A QR code the server encoded, drawn from its path. **@my/ui:** `QrCode` — `app/ui/src/QrCode.tsx`.

**Consumer provides:** `size` (modules, quiet zone included), `path` (the server's SVG path data), and `label` (「認証アプリ用のシークレットの QR コード」). Put what it holds in text next to it for anyone who cannot scan.

- 192px, `role="img"`, crisp edges. Dark modules on a light ground in **both** themes (`color-scheme: light`), since readers need it: modules `charcoal`, ground white, framed by a 16px radius.

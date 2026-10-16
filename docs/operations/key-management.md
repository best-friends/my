# 鍵暗号化鍵（KEK）の管理

運営者向けの手順です。方式の根拠は [ADR-0019 §6](../adr/0019-security-baseline.md)、要件は NFR-SEC-7・NFR-REL-4（[非機能要件](../requirements/02-non-functional.md)）です。
設定項目の一覧は [設定リファレンス](../reference/configuration.md) を参照してください。

## 仕組み

- アクターの秘密鍵など DB に保存する秘密は、秘密ごとに生成したデータ暗号化鍵（DEK）で AES-256-GCM 暗号化し、
  DEK を **鍵暗号化鍵（KEK）** で包んで（wrap して）暗号文と一緒に保存する（エンベロープ暗号化）。
- KEK は DB に保存しない。DB のダンプだけが漏れても秘密は読めない。
- 暗号文には、DEK を包んだ KEK の **鍵 ID** が付く。`key_manager = local` の鍵 ID は `local:` と KEK から導いた 16 桁の 16 進数で、
  同じ KEK からは常に同じ ID になる。運営者が ID を付ける必要はない。
- 暗号文は保存先の行 ID と列名に結び付けられており、別の行や列に写すと復号できない。
- v0 の `key_manager` は `local`（設定の `kek` を使う）だけ。

## KEK を失うと何が起きるか

KEK がなければ、DB をリストアしてもアクターの秘密鍵を復号できない。秘密鍵を失うと、そのアクターの署名を他サーバーが検証できなくなり、
**連合上の同一性が壊れる**（フォロワーとの関係を引き継げない）。KEK は DB と同じ重さで扱う。

## 生成する

32 バイトの乱数を標準の base64 にする。

```sh
openssl rand -base64 32
```

`openssl` がない環境では `head -c 32 /dev/urandom | base64` でもよい。生成した値は端末の履歴やチケットに残さない。

## 保管して渡す

KEK は秘密の設定なので、環境変数 `MY_KEK` か、値を書いたファイルのパス `MY_KEK_FILE` でだけ渡せる。設定ファイル（TOML）に書くと起動しない。

| ターゲット | 渡し方の例 |
| --- | --- |
| Self-hosted（バイナリ / systemd） | 所有者だけが読めるファイル（`chmod 600`）に置き、`MY_KEK_FILE` で指す |
| Docker / Compose | Docker secret を `/run/secrets/my_kek` にマウントし、`MY_KEK_FILE=/run/secrets/my_kek` |
| Kubernetes / microk8s | Secret をファイルとしてマウントし、`MY_KEK_FILE` で指す |
| Cloud Run | Secret Manager のシークレットをファイルか環境変数として公開する |
| AWS Lambda / ECS | Secrets Manager か SSM Parameter Store（SecureString）から渡す |

- web と worker など全プロセスに同じ KEK を渡す。
- KEK はログとエラーメッセージに現れない（出るのは鍵 ID だけ）。
- `my config check` は KEK の形式（base64 で 32 バイト）を検査し、値は `(redacted)` と表示する。

## バックアップする

**DB と KEK を別々に、両方バックアップする。**

- KEK は DB のバックアップとは別の場所に保管する（パスワードマネージャ、オフラインの媒体、別アカウントのシークレットストアなど）。
  同じ場所に置くと、そこが漏れたときに DB の暗号化が意味を失う。
- ローテーション中は `kek_previous` の鍵も、それで包まれた暗号文が DB に残っている限りバックアップに含める。
- リストアの手順に KEK を含め、定期的にリストアを試す。DB をリストアしたら、バックアップ時点で使っていた KEK（と `kek_previous`）を同じ値で設定する。
  KEK が違うと、秘密を読む処理が鍵 ID を名指しした `unknown key-encryption key` エラーで失敗する。

## ローテーションする

`kek_previous` に置いた KEK は復号だけに使い、新しい秘密は常に `kek` で暗号化する。
ローリングデプロイ中に新旧のプロセスが混在しても読めない暗号文が出ないよう、2 段階で切り替える。

1. 新しい KEK を生成し、バックアップする。
2. 全プロセスを `MY_KEK=<旧 KEK>`、`MY_KEK_PREVIOUS=<新 KEK>` で再起動する。全プロセスが新 KEK で復号できる状態になる。
3. 全プロセスを `MY_KEK=<新 KEK>`、`MY_KEK_PREVIOUS=<旧 KEK>` で再起動する。以降の秘密は新 KEK で包まれ、旧 KEK の暗号文も読める。
4. 旧 KEK は `kek_previous` に残し、バックアップも保持する。旧 KEK を外せるのは、DB の全暗号文が新 KEK で包み直された後だけである。
   既存の暗号文をまとめて包み直すコマンドはまだないため、旧 KEK は `kek_previous` に残し続ける。

`kek_previous` にはカンマ区切りで複数の鍵を書ける。同じ鍵を 2 回書いたり、`kek` と同じ鍵を書いたりすると起動しない。

## KEK が漏れたとき

KEK だけが漏れた場合は、上の手順でローテーションする。KEK と DB の両方が漏れた可能性がある場合、ローテーションでは守れない。
DB に入っていたアクターの秘密鍵はすでに読めるので、アクター鍵そのものの再生成が必要になる。

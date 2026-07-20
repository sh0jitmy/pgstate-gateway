# pgstate-gateway

> PostgreSQL をバックエンドとした、本番環境対応の Terraform / OpenTofu HTTP バックエンドサーバー

---

## 🚀 特徴

`pgstate-gateway` は、セキュアで高信頼な Terraform および OpenTofu の HTTP ステート管理・ロックサーバーです。外部依存関係として PostgreSQL のみを採用しており、エンタープライズの商用環境に適合するセキュリティと可観測性を備えています。

1. **Terraform HTTP バックエンド仕様の完全準拠**:
   - `GET /state/{workspace}`: ステートデータの安全な取得
   - `POST /state/{workspace}`: ステートデータの更新
   - `DELETE /state/{workspace}`: ステートデータの削除
   - `LOCK /state/{workspace}`: ステート競合を防ぐ排他ロックの取得（`423 Locked` 時に既存のロックメタデータを返却）
   - `UNLOCK /state/{workspace}`: ロックの安全な解放（ロックIDの不一致検知時は `409 Conflict` 返却）

2. **堅牢なデータベースレイヤー (`pgx/v5`)**:
   - PostgreSQL 15 以上を公式サポート。
   - `pgxpool` による高度なコネクションプール管理（コネクション維持・タイムアウト・Pingヘルスチェック）。
   - `golang-migrate` の統合により、起動時に組み込みの DDL マイグレーション（マイグレーションテーブル自動作成、インデックス・一意制約の設定）を自動実行。

3. **強固なセキュリティ機能**:
   - **セキュアな認証**: HTTP Basic 認証および Bearer トークンの両方をサポート。
   - **HTTPS/TLS自動化**: Let's Encrypt (`autocert`) に対応し、サーバー単体で証明書の自動取得・更新およびポート80から443への常時リダイレクトを実行可能。
   - **レートリミット**: IPアドレスベースのトークンバケットアルゴリズムによる API 流量制限。
   - **セキュリティヘッダー**: HSTS、CSP、Frame-Options、X-Content-Type-Options などの付与、および `http.MaxBytesReader` による 50MB までのリクエストボディ容量制限。
   - **プロファイリング保護**: pprof サーバーを `127.0.0.1:6060` (localhost) にのみバインドし、機密情報の漏洩を防止。

4. **高度な可観測性 (Observability)**:
   - **構造化ロギング**: Zap ロガーによる JSON 構造化ログ。Basic 認証ヘッダー、Bearer トークン、ステート内のシークレット情報などは自動マスキング（`[REDACTED]`）されます。
   - **OTel 連携**: OpenTelemetry トレースコンテキストを抽出し、ログレコードへ `trace_id` / `span_id` を自動注入。
   - **監査ログの分離**: 不正アクセスやロック取得・解放、サーバー起動などの重要イベントには自動で `log_type: "audit"` 属性を付与し、SIEM等での分離抽出を可能に。
   - **Prometheus メトリクス計装**: `/metrics` エンドポイントを公開し、リクエスト数、処理時間、DB接続プールの稼働状況、現在のワークスペース別アクティブロック数などのメトリクス（低カーディナリティを遵守）を提供。

---

## 🛠️ クイックスタート

### 1. ビルド
ローカル環境でバイナリをビルドするには、以下のコマンドを実行します：
```bash
make build
```
生成されたバイナリは `bin/tf-http-backend` に配置されます。

### 2. コンテナ起動
Docker / OCI 互換コンテナイメージを単体でビルドするには以下を実行します：
```bash
docker build -t pgstate-gateway .
```
本番環境向けのコンテナは、非特権ユーザー（`nobody`）で動作する最小限の静的 distroless イメージベースです。

### 3. Docker Compose による一括起動（ローカル検証・開発）
PostgreSQL データベースと `pgstate-gateway` サーバーを Docker Compose を用いて一括でビルド・起動できます。
```bash
docker compose up --build
```
このコマンドにより、データベースコンテナの起動待ち、自動マイグレーションの実行、およびバックエンドサーバーの起動が自動的かつアトミックに実行されます。

### 4. 設定ファイルの作成
`configs/config.yaml` を用意します（環境変数による上書きも対応しています）：
```yaml
server:
  listen: ":8080"
  listen_http: ":80"

database:
  host: "localhost"
  port: 5432
  user: "terraform"
  password: "secure-password"
  database: "tfstate"
  sslmode: "disable"
  max_open_conns: 10
  max_idle_conns: 10
  conn_max_lifetime: "10m"

https:
  enabled: false
  domains:
    - "tfstate.example.com"
  email: "admin@example.com"
  cache_dir: "./certs"

auth:
  basic:
    username: "tfuser"
    password: "tf-secret-password"
  bearer_tokens:
    - "tf-bearer-token-1234"

logging:
  level: "info"

security:
  rate_limit: 100
  max_body_size: 52428800 # 50MB
```

### 5. 起動
以下のコマンドでサーバーを起動します：
```bash
./bin/tf-http-backend serve --config configs/config.yaml
```

---

## ⚙️ 開発コマンド一覧

Makefile に定義されている以下のコマンドを使用して開発を進めます：

| コマンド | 説明 |
| :--- | :--- |
| `make fmt` | ソースコード of フォーマット (`go fmt`) |
| `make lint` | `golangci-lint` を使用した静的解析の実行（警告ゼロ品質の維持） |
| `make tidy` | 依存関係 (`go.mod` / `go.sum`) の整理 |
| `make vulncheck` | `govulncheck` を使用した脆弱性診断の実行 |
| `make test` | 単体テスト（UT）およびビジネスロジックカバレッジ検証の実行 |
| `make build` | バイナリのコンパイル |
| `make license-check` | ソースコードのライセンスヘッダーの自動チェック |
| `make license-add` | ソースコードへのライセンスヘッダーの自動追加 |
| `make clean` | ビルド成果物やテストキャッシュのクリーンアップ |

---

## 🧪 テストの実行

### 単体テスト (Unit Tests)
```bash
make test
```
ビジネスロジックレイヤー (`internal/service/`) は 100% のステートメントカバー率を達成しています。

### 統合テスト (Integration Tests)
Testcontainers for Go を使用し、ローカルの Docker環境で PostgreSQL コンテナを立ち上げてマイグレーション実行からE2Eの挙動まで検証するテストを実装しています。
```bash
go test -v -tags=integration ./test/...
```

---

## 📄 ライセンス

本プロジェクトは **Apache License 2.0** で提供されています。
詳細は [LICENSE](file:///Users/shjtmy/gravity/pgstate-gateway/LICENSE) をご参照ください。

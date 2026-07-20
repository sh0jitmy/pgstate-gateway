# プロジェクト要件および自己評価チェックリスト

本プロジェクトで実装した成果物が、要求された要件をすべて満たしているかを自己評価するための要件リストです。
以下のチェックを実行し、スコアを算出します。

## 📋 要件チェックリスト

### 1. プロジェクト初期セットアップ
- [x] **R-1.1 モジュール名のカスタマイズ**: `go.mod` 内のモジュール名を `github.com/sh0jitmy/pgstate-gateway` に変更し、サポートするGoのバージョン（1.26.5）が正しく指定されていること。
- [x] **R-1.2 GitHub Actions 権限設定**: `tagpr` や `goreleaser` が動作するように、GitHubリポジトリの `Settings > Actions > General > Workflow permissions` で 「Read and write permissions」が許可されていること。

### 2. 品質・セキュリティ基準
- [x] **R-2.1 セキュアロギング**: zap ロガーにてパスワードやトークンなどの機密情報がログに出力される際にマスキングまたは除外されること。
- [x] **R-2.2 静的解析のクリア**: `make lint` を実行し、すべての静的解析警告がないクリーンな状態であること。
- [x] **R-2.3 脆弱性診断のクリア**: `make vulncheck` を実行し、パッケージ脆弱性がないこと。
- [x] **R-2.4 テストとビルドの保証**: `make test` および `make build` が正常にパスすること。
- [x] **R-2.5 自動リリースの統合**: `.tagpr` および `.goreleaser.yaml`、各種GitHub Actionsワークフローが配置され、リリースフローが定義されていること。
- [x] **R-2.6 単体テスト(UT)の網羅**: アプリケーションロジックに対して対応する単体テスト（UT）が作成されパスしていること（カバレッジ目標90%以上）。
- [x] **R-2.7 テスト品質の自動検証**: `TestMain` での goroutine リーク検出（`goleak`）および `golangci-lint` でのテスト品質監査（`paralleltest` / `testifylint` / `tparallel`）が設定され、テストコードの品質が担保されていること。
- [x] **R-2.8 ライセンス＆作成者ヘッダーの自動監査**: `make license-check` を通して Go ソースファイルのライセンス・作成者ヘッダーの欠落を自動検証でき、`make license-add` で自動付与できること。
- [x] **R-2.9 統合・E2Eテストの実装**: Testcontainers を使用した PostgreSQL 接続テスト、およびテスト用 Terraform ファイルを使用した `terraform init` / `apply` 検証が行えること。
- [x] **R-2.11 データベースアクセス (pgx/v5 + PostgreSQL)**: `pgx/v5` を使用して PostgreSQL 15 以上にアクセスし、golang-migrate によるマイグレーションが動作すること。
- [x] **R-2.12 Basic & Bearer 認証**: HTTP Basic 認証と Bearer トークンの両方を検証する認証機構が実装されていること。
- [x] **R-2.13 HTTPS & 自動証明書更新 (autocert)**: Let's Encrypt / autocert を利用した HTTPS 化、証明書自動更新および HTTP → HTTPS リダイレクト機能が実装されていること。
- [x] **R-2.14 OTel ログ戦略と監査ログ分離**: OpenTelemetry トレースコンテキストをログに紐付けて `trace_id` / `span_id` を自動注入し、重要なセキュリティイベントに `log_type: "audit"` 属性を付与して通常ログと分離していること。
- [x] **R-2.15 OTel/Prometheus メトリクス計装と低カーディナリティの遵守**: Prometheus Exporter 経由で `/metrics` からリクエスト数、処理時間、DB接続プール状況、アクティブロック数などのメトリクスを提供し、低カーディナリティを遵守していること。
- [x] **R-2.16 安全な pprof プロファイリングの有効化（localhostバインド）**: pprof ポート（`127.0.0.1:6060`）を外部公開せずに localhost にのみバインドした独立したMuxで安全に有効化していること。

---

## 📈 自己評価結果

- **合計要件数**: 18
- **達成要件数**: 18 / 18
- **適合率 (達成数/18)**: 100.00 %

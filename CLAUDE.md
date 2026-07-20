# CLAUDE.md

## プロジェクトの概要

このプロジェクトは、PostgreSQL バックエンドを備えた、本番環境対応の Terraform / OpenTofu HTTP バックエンドサーバー (`pgstate-gateway`) です。
本リポジトリには、開発の品質・設計・セキュリティを向上させるための**日本語カスタムスキル（`.claude/skills/`）**が同梱されています。

## AIエージェント（Claude Code）への指示

> [!IMPORTANT]
> - **スキルの厳格な適用**: 本プロジェクトにおけるコードの実装、設計、リファクタリング、およびコードレビューを行う際は、必ず `.claude/skills/` にある各スキル（`golang-implementation`、`golang-observability`、`sre-deployment` 等）の指針に準拠してください。
> - **言語の統一**: コミットメッセージは**英語**、それ以外のPR説明、Issue、およびAIによるレビューレポートは**完全な日本語**で記述してください。

## 便利なコマンド

### Go開発タスク
- **コードフォーマット**: `make fmt`
- **静的解析の実行**: `make lint`
- **脆弱性スキャンの実行**: `make vulncheck`
- **単体テストの実行**: `make test`
- **統合テストの実行**: `go test -v -tags=integration ./test/...`
- **ビルドの実行**: `make build`

### 品質・スキル管理タスク
- **カスタムスキルのグローバルインストール**: `make install`
  （※開発前に実行して、PC上のすべての Claude Code で本スキルを有効にしてください）
- **プロジェクトの要件セルフチェック**: `make self-eval`
  （※コード変更後、`REQUIREMENTS.md` の要件をクリアしているか自己評価を実行します）
- **スキルの構文検証**: `make check`

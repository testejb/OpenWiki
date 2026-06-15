# OpenWiki — AI駆動の個人知識ベース

**言語 / Language / 语言：** [中文](README.md) ｜ [English](README.en.md) ｜ 日本語（デフォルト）

---

## これは何ですか？

OpenWiki は AI skill / agent 向けのファイル優先の個人知識ベース用スキャフォールドです。Markdown ファイルが主な書き込みインターフェースで、AI skill は知識ファイルを直接編集します。CLI は内容作成を所有せず、設定検出、検証、状態確認、索引チェック、索引再構築の guardrails を提供します。

**コアアイデア：**

- `openwiki.toml` が唯一の canonical runtime contract です。
- デフォルトの `wiki_root` は `./openwiki/` です。
- OpenWiki はプロジェクトローカルのランタイムモデルを優先します。
- `wiki/index.md` は軽量な Routing Index で、すべてのページを列挙しません。
- `wiki/indexes/` は成長する索引を保持する Shard Indexes です。

---

## ランタイムモデル

OpenWiki はプロジェクトローカルのランタイムモデルを使用します。

```text
<project>/
├── openwiki.toml            # 唯一の canonical runtime contract；目標のプロジェクト直下契約
└── openwiki/                # デフォルト wiki_root
    ├── raw/                 # 原本素材
    ├── wiki/
    │   ├── index.md         # 軽量 Routing Index
    │   ├── log.md           # 操作ログ
    │   ├── pages/           # 通常の知識ページ
    │   └── indexes/         # Shard Indexes / 分片索引
    │       ├── scopes.md
    │       ├── entities.md
    │       ├── concepts.md
    │       ├── tags.md
    │       ├── recent.md
    │       ├── hot.md
    │       └── query-usage.jsonl
    ├── entities/            # エンティティページ
    └── concepts/            # 概念、分析、保存した回答
```

基本原則：

- `openwiki.toml` が唯一の canonical runtime contract です。
- デフォルトの `wiki_root` は `./openwiki/` です。
- Markdown ページファイルが内容の source of truth です。
- `wiki/index.md` は軽量な Routing Index で、検索ルーティングだけを担い、すべてのページを列挙しません。
- `wiki/indexes/` は Shard Indexes で、`scopes.md`、`entities.md`、`concepts.md`、`tags.md`、`recent.md`、`hot.md`、`query-usage.jsonl` を含みます。
- AI skill は Markdown ファイルを直接編集します。
- CLI は設定検出、検証、状態確認、`index check`、`index rebuild` の guardrails を提供します。

> 現在の CLI の `openwiki init` は、対象 wiki root の中に `openwiki.toml` を書き込みます。上記のプロジェクト直下契約モデルを使う場合は、プロジェクト直下に `wiki_root = "./openwiki"` を含む `openwiki.toml` を置くか、`--config` / `-c` でその契約ファイルを指定してください。

---

## クイックスタート

### 前提条件

- 任意の `skill.io` 互換エージェント、またはこのリポジトリの skills を読める AI agent / ツール
- （任意）[agent-browser](https://github.com/mediar-ai/agent-browser)：Web 補完と検証に使用

### インストール

```bash
git clone https://github.com/crabin/llm-wiki.git openwiki-project
cd openwiki-project
```

AI agent にこのリポジトリを読み込ませ、`skill/` 配下の公開 wiki skills を参照できるようにします。

### 初期化

現在の CLI は wiki root を初期化します。

```bash
openwiki init ./openwiki/
```

このコマンドは `./openwiki/` を作成し、その中に `openwiki.toml`、`wiki/index.md`、`wiki/indexes/`、`raw/`、`entities/`、`concepts/` を書き込みます。

プロジェクトローカルのトップレベル契約を使う場合は、プロジェクト直下に次の `openwiki.toml` を置きます。

```toml
wiki_root = "./openwiki"

[wiki]
primary_language = "zh"
secondary_language = "en"
```

必要に応じて明示的に指定します。

```bash
openwiki --config ./openwiki.toml status
openwiki --config ./openwiki.toml index check
openwiki --config ./openwiki.toml index rebuild
```

設定検出順序：

1. `--config` / `-c`
2. `OPENWIKI_CONFIG`
3. 現在の作業ディレクトリから上位へ `openwiki.toml` を検索
4. `~/.openwiki/openwiki.toml`

---

## リポジトリ構造

```text
openwiki-project/
├── skill/                    # 公開 wiki skills
│   ├── wiki-init/
│   ├── wiki-ingest/
│   ├── wiki-query/
│   ├── wiki-lint/
│   ├── wiki-update/
│   └── agent-browser/
├── openwiki.toml             # プロジェクトローカル契約（目標モデル；--config で指定可能）
├── openwiki/                 # デフォルト wiki_root
│   ├── raw/
│   ├── wiki/
│   │   ├── index.md          # Routing Index
│   │   ├── log.md
│   │   ├── pages/
│   │   └── indexes/          # Shard Indexes
│   │       ├── scopes.md
│   │       ├── entities.md
│   │       ├── concepts.md
│   │       ├── tags.md
│   │       ├── recent.md
│   │       ├── hot.md
│   │       └── query-usage.jsonl
│   ├── entities/
│   └── concepts/
├── README.md
├── README.en.md
└── README.ja.md
```

---

## Skills と CLI の役割分担

### AI skills

- `wiki-init`：プロジェクトローカル契約と wiki ファイル構造を準備します。
- `wiki-ingest`：素材を読み、重要点をユーザーと確認してから Markdown ページを直接書き、関連する Shard Indexes を保守します。
- `wiki-query`：まず `wiki/index.md` Routing Index を読み、必要に応じて `wiki/indexes/` の分片と関連ページを読みます。有用な回答は `concepts/` に保存できます。
- `wiki-lint`：リンク切れ、孤立ページ、矛盾、古い記述、索引健康状態を確認します。
- `wiki-update`：既存 Markdown ページを直接編集し、逆リンク、ログ、分片索引を更新します。
- `agent-browser`：Web 取得とファクトチェックを担当し、引用可能な URL と本文を提供します。

### CLI guardrails

CLI は内容作成を所有しないランタイム上の guardrails を担当します。

- `openwiki.toml` を発見し、`wiki_root` を解決する。
- 必須ディレクトリ、`wiki/index.md`、`wiki/indexes/` を検証する。
- 状態と索引健康サマリーを出力する。
- `openwiki index check` で Routing Index と Shard Indexes をチェックする。
- `openwiki index rebuild` で Markdown ページと `query-usage.jsonl` から索引を再構築する。

---

## E2E テスト

- 高速 deterministic Artifact E2E:
  ```bash
  python3 -m unittest tests.test_wiki_skill_workflow_e2e -v
  ```
- 全 fast テスト:
  ```bash
  python3 -m unittest discover -s tests -p "test_*.py"
  ```
- 低速な実 agent smoke E2E:
  ```bash
  SKILL_AGENT_E2E=1 SKILL_AGENT_RUNNER=/path/to/compatible-agent-wrapper python3 -m unittest tests.test_agent_skill_smoke_e2e -v
  ```

実 runner 用ケースはデフォルトで skip され、`SKILL_AGENT_E2E=1` を設定したときだけ実行されます。

---

## 設計原則

- **ファイル優先**：Markdown ファイルが知識内容の主要な source of truth です。
- **中立ランタイム**：実行時は `openwiki.toml` に依存し、特定エージェント名には依存しません。
- **分層索引**：Routing Index は軽量に保ち、Shard Indexes が成長を受け持ちます。
- **AI が書き、CLI が守る**：AI skills が内容を編集し、CLI が発見、検証、索引修復を担当します。
- **追跡可能なソース**：重要な主張はファイルパスか URL に結び付けます。

---

## ライセンス

MIT

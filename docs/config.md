# Config

`~/.config/jig/config.yaml` (XDG) or `~/.jig.yaml`. Env vars override file values:

| Env var                    | YAML key              | Default      |
|----------------------------|-----------------------|--------------|
| `JIG_DIFF_RENDERER`       | `diff.renderer`       | `chroma`     |
| `JIG_LOG_COMMIT_LIMIT`    | `log.commitLimit`     | `50`         |
| `JIG_REBASE_DEFAULT_BASE` | `rebase.defaultBase`  | `HEAD~10`    |
| `JIG_SHOW_DIFF_PANEL`     | `ui.showDiffPanel`    | `true`       |
| `JIG_UI_THEME`            | `ui.theme`            | `dark`       |

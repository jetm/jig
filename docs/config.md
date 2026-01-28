# Config

`~/.config/gti/config.yaml` (XDG) or `~/.gti.yaml`. Env vars override file values:

| Env var                    | YAML key              | Default      |
|----------------------------|-----------------------|--------------|
| `GTI_DIFF_RENDERER`       | `diff.renderer`       | `chroma`     |
| `GTI_LOG_COMMIT_LIMIT`    | `log.commitLimit`     | `50`         |
| `GTI_REBASE_DEFAULT_BASE` | `rebase.defaultBase`  | `HEAD~10`    |
| `GTI_SHOW_DIFF_PANEL`     | `ui.showDiffPanel`    | `true`       |
| `GTI_UI_THEME`            | `ui.theme`            | `dark`       |

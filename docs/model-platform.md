# Model Platform

Nostos model selection is provider-scoped. A selectable model is always the pair:

```text
provider_id + model_id
```

The `model_id` is preserved exactly as returned by the provider. Nostos does not shorten IDs such as:

```text
NVIDIA NIM/openai/gpt-oss-120b
Bifrost/opencode-proxy/deepseek-v4-flash-free
```

## Model Roles

Global model roles are stored per workspace:

- `chat`: default for normal conversations and user-facing assistant answers.
- `utility`: default for conversation summaries, reply drafts, titles, lightweight worker AI operations, and internal LLM maintenance work.
- `vision`: reserved for future visual-understanding requests. Version 0.1.x configures the role and resolver boundary only; it does not add image generation or image editing.

Each role supports an ordered list of provider/model entries. Resolution uses the first configured entry with an enabled provider and usable credentials. If no role binding exists, legacy provider defaults are used as a compatibility fallback.

## Catalog Cache

Provider models are cached in `provider_models`. Model pickers read from this database cache instead of calling `/v1/models` on every open.

Refresh behavior is stale-while-revalidate:

1. Existing cached models remain usable during refresh.
2. New models are inserted or reactivated.
3. Models missing from the latest provider response are marked unavailable.
4. Failed refreshes keep the previous cache.
5. Unavailable models are only removed by explicit cleanup.

The provider row records refresh state and safe error metadata:

```text
idle
queued
refreshing
succeeded
failed
```

## Large Catalogs

Large OpenAI-compatible proxies can return hundreds of models. The default model refresh timeout is:

```env
MODEL_REFRESH_TIMEOUT=60s
```

The accepted range is `1s` through `300s`. Provider request timeouts still apply to chat and health operations; model refresh can use the larger dedicated timeout.

## Capabilities

Cached models store lightweight capabilities:

```text
chat
vision
tools
reasoning
embedding
audio
image_generation
```

Capabilities may come from heuristics, provider metadata, manual overrides, or future probes. Nostos does not claim a capability is verified unless a probe or manual operator input records that source.

## Manual Models

Manual models can be added when a provider does not expose `/v1/models` or omits a usable model. Manual entries remain provider-scoped and can be selected in the same picker as API-discovered models.

# Custom OpenAI-compatible models

A provider profile can list models and their reasoning levels in the user config
file. Keep `model` as the active model. For example:

```json
{
  "activeProvider": "my-endpoint",
  "providers": [{
    "name": "my-endpoint",
    "catalogID": "custom-openai-compatible",
    "baseURL": "https://example.com/v1",
    "apiKeyEnv": "MY_API_KEY",
    "model": "model-a",
    "models": [
      { "id": "model-a", "reasoningEfforts": ["low", "medium", "high"] },
      { "id": "model-b", "reasoningEfforts": ["low", "high", "max"] },
      { "id": "model-c", "reasoningEfforts": [] }
    ]
  }]
}
```

Replace the example URL, environment variable, model IDs, and levels with values
supported by your endpoint. Do not put secrets in the example or a project file.
Zero keeps these model lists separate for each provider profile.

- `/model` includes configured models, even when model discovery fails. It also
  includes models discovered from the endpoint.
- `/effort` and Ctrl+T select from the configured levels for the active model.
  An empty list disables level selection. Omit `reasoningEfforts` to use
  discovered or catalog levels instead.
- `auto` omits `reasoning_effort`, leaving the endpoint's default in effect.
  `auto` and `list` are reserved and cannot be configured as levels.
- `zero exec -r high "prompt"` uses the same configured levels. Unsupported
  selections are ignored with a warning. Switching models clears an explicit
  effort when the new model's configured list does not contain it.
- An explicit level is sent as the `reasoning_effort` string in a chat
  completions request. The endpoint must support that field and value. Some
  compatible APIs use a different field or need extra parameters; this setting
  does not translate those requests.

Model IDs must be unique within a provider profile. A level must be nonempty,
contain no whitespace or control characters, and be unique within its model
(case-insensitively). If the same provider appears in multiple config layers,
a later `models` list replaces the earlier list. Existing profiles without a
`models` list keep their previous behavior. The per-model reasoning override
applies to OpenAI-compatible profiles, not native OpenAI, Anthropic, or Gemini.

## Check a prepared request

Set `ZERO_REASONING_DIAGNOSTIC=1` to print the model and serialized
`reasoning_effort` to standard error for each OpenAI-compatible chat request.
For example, in PowerShell:

```powershell
$env:ZERO_REASONING_DIAGNOSTIC = "1"
zero exec -r high "Say hello"
Remove-Item Env:ZERO_REASONING_DIAGNOSTIC
```

The output looks like this:

```text
zero: chat request prepared model="model-a" reasoning_effort="high"
```

An unset effort shows `reasoning_effort=<omitted>`. For the interactive TUI,
redirect standard error so the output does not interfere with the display:

```powershell
$env:ZERO_REASONING_DIAGNOSTIC = "1"
zero 2> reasoning-diagnostic.log
Remove-Item Env:ZERO_REASONING_DIAGNOSTIC
Get-Content reasoning-diagnostic.log
```

This diagnostic runs after serialization and before dispatch. It reports what
Zero prepared, not whether a proxy forwarded it or a provider honored it.
Values are sanitized and known credentials are redacted. It does not log request
headers, prompts, tools, URLs, or the full body. It covers chat completions,
not Codex Responses or other provider formats. Standard error can also contain
unrelated warnings; inspect any log before sharing it.

# Evaluating context compaction with real models

`TestContextLiveFidelity` exercises the real OpenAI-compatible provider and the
production compaction/agent paths. It is skipped unless explicitly enabled; it
uses synthetic transcripts only, needs no filesystem tools, and may incur API
charges. Ordinary CI uses deterministic tests and does not need credentials.

## Run locally

Start Ollama with `gpt-oss:20b`, configured for at least a 32K context window so
the uncompressed control fits. Then run:

```bash
GSH_CONTEXT_LIVE=1 go test -count=2 -timeout=15m -v \
  ./internal/script/interpreter -run '^TestContextLiveFidelity$'
```

PowerShell:

```powershell
$env:GSH_CONTEXT_LIVE = '1'
go test -count=2 -timeout=15m -v ./internal/script/interpreter -run '^TestContextLiveFidelity$'
Remove-Item Env:GSH_CONTEXT_LIVE
```

The defaults are `http://localhost:11434/v1`, API key `ollama`, and model
`gpt-oss:20b`. To evaluate an existing OpenAI-compatible service, set
`GSH_CONTEXT_LIVE_BASE_URL`, `GSH_CONTEXT_LIVE_API_KEY`, and
`GSH_CONTEXT_LIVE_MODEL` in the process environment. Do not commit credentials
or put them in command-line arguments. The test uses temperature 0, a 12,000
estimated-token context budget, a 2,048-token response reserve, a 1,024-token
summary limit, and one recent message. The request budget intentionally forces
compaction even when the actual server supports a larger window.
`GSH_CONTEXT_LIVE_SUMMARY_TOKENS` overrides the summary limit for reasoning
models; report this setting with the results. For example, a provider can use
all 1,024 completion tokens for reasoning and return an empty, length-limited
summary. That is a budget failure, not a successful compaction; the production
path must preserve history and explain how to increase `summaryTokens`.

The verbose output records synthetic memories, estimated input sizes and
summarization usage. It never prints model configuration or credentials. Save
the output together with the model/route, date, revision and repeat count.

## What the assertions measure

Four fixtures test:

1. A later user correction superseding the initial path/version and cancelling
   deployment authorization.
2. Exact identifiers at the start of a long tool result, a nonzero exit code at
   the end, and a still-pending command.
3. Chinese paths, identifiers and corrections.
4. Untrusted tool text asking the summarizer to falsify a successful deployment.

Each fixture checks five exact facts (`config_path`, `release_id`, `exit_code`,
`deployed`, `next_step`) against hand-written expected values in five modes:
uncompressed control, compacted plain response, compacted streaming response,
response after a second compaction, and the automatic-compaction agent path.
No model is used as a judge. A valid JSON answer must match all expected fields;
incorrect values and unexpected fields fail the test. Code-fenced JSON is
accepted so formatting alone does not obscure a fact-retention failure.

The deterministic `TestContextSummaryFollowsInitialRequest` regression checks
chronology independently of a particular model. The fragment test checks task
context, source roles, tool IDs, offsets, and exact lossless reassembly of all
serialized historical messages, including Unicode split across requests.

## Limits

Passing these fixtures is evidence for these scenarios, not a general summary
fidelity guarantee or proof of resistance to prompt injection. Summaries remain
lossy; open-ended planning, larger/different transcripts, other models, and
longer sequences can behave differently. Two runs at temperature 0 can still
differ because providers are not necessarily deterministic. A failing control
means that model could not answer the fixture reliably even without compaction;
report it separately from a compaction regression.

No tool side effects are executed by this evaluation. Live ACP, interactive
terminal behavior, model-specific tokenization, and provider context-limit
enforcement are outside its scope.

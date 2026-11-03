# Evaluating context compaction with real models

`TestContextLiveFidelity` exercises the real OpenAI-compatible provider and the
production compaction/agent paths. It is skipped unless explicitly enabled; it
uses synthetic transcripts only, needs no filesystem tools, and may incur API
charges. Ordinary CI uses deterministic tests and does not need credentials.

## Run locally

Start Ollama with `gpt-oss:20b`, configured for at least a 32K context window so
the uncompressed control fits. Then run:

```bash
GSH_CONTEXT_LIVE=1 go test -count=2 -timeout=70m -v \
  ./internal/script/interpreter -run '^TestContextLiveFidelity$'
```

PowerShell:

```powershell
$env:GSH_CONTEXT_LIVE = '1'
go test -count=2 -timeout=70m -v ./internal/script/interpreter -run '^TestContextLiveFidelity$'
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

An additional opt-in `TestContextLiveToolContinuation` runs a real agent with
an in-memory `read_item` tool. It requires eight ordered reads, at least one
automatic compaction during that turn, no duplicate attempts (including
rejected ones), and a final `DONE`. No files or shell commands are accessed.
With the same environment settings, run:

```bash
go test -count=2 -timeout=10m -v ./internal/script/interpreter \
  -run '^TestContextLiveToolContinuation$'
```

This smoke test uses a 10,000-token estimated window, a 2,048-token summary
budget and a 2,048-token answer reserve. It supplements the golden-answer
fixtures; it does not establish exactly-once execution for arbitrary agents.

## Regression found in the initial draft

During a real-model run against draft `cb9bae9`, the uncompressed control used
the corrected `/srv/new/app.yaml` and `rel-new-222`. After a second compaction,
the answer reverted to `/srv/old/app.yaml` and `rel-old-111`, even though the
memory still contained the correction. The request placed memory before the
initial user message. Moving it after that initial message preserves the order
of corrections; the deterministic chronology test covers this independently of
model variability.

The same evaluation exposed missing task context in summaries and fragments
that lost their source role inside large messages. Summary requests now carry
the retained task context and label every fragment. Small messages stay whole
when they fit on their own; long messages prefer nearby line boundaries.

A 1,024-token summary limit also produced `finish_reason=length` with zero
visible content and 1,024 completion tokens on the Pro route. Such a response
does not contain usable memory. It remains a failure with the original history
intact, now with an actionable `summaryTokens` diagnostic. Increasing the
budget is an explicit configuration choice, not a hidden retry.

Each fixture allows eight minutes for its control, multiple summary passes and
continuations together. A shorter four-minute fixture deadline exhausted its
time in the final automatic-compaction phase on the slower route despite the
earlier fact checks passing. This is a harness deadline, not a fact-retention
failure. Individual fixtures can be selected with `-run` and run independently
to reduce wall time, subject to the provider's concurrency limits.

## Recorded results (2026-11-03–04, Asia/Shanghai)

The implementation at `19e273b` was evaluated through an existing
OpenAI-compatible gateway using the routes below. Each fixture was run twice;
the slower route's fixtures were run independently. The harness made no
thinking-mode override. Both routes explicitly used `summaryTokens=4096` for
the golden-answer matrix; the other settings were the same as described above.

| Model route | Fixtures × repeats | Exact JSON answers | Exact field comparisons |
| --- | --- | --- | --- |
| `deepseek-flash` | 4 × 2 | 40/40 passed | 200/200 passed |
| `deepseek-v4-pro` | 4 × 2 | 40/40 passed | 200/200 passed |

Each route's 40 answers include eight uncompressed controls and 32 answers
after compaction. The final counts exclude interrupted preliminary runs and
the earlier output-budget/deadline failures described above; all four final
Pro fixture processes and the final Flash process exited successfully.
Estimated serialized input decreased by 85.2–88.5% in these deliberately
repetitive fixtures. This is not a measurement of production token/cost savings.

The separate Flash tool-continuation test also passed twice: eight ordered
synthetic reads and three automatic compactions in each run, no duplicate
attempts, and `DONE` at completion (16 reads, six compactions total).

Local validation used Go 1.25.14. Linux/WSL `make ci` passed, including native
Node 22.23.3/npx filesystem MCP integration tests, code generation, formatting,
and golangci-lint 2.15.0 with zero issues. Related tests passed under `-race`;
Windows/Linux builds and a standalone `.gsh` configuration/manual-command
smoke test passed. Remote GitHub Actions status is reported separately in the
PR; local success does not imply upstream Actions have been approved or run.

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

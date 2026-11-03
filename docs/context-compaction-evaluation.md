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

`GSH_CONTEXT_LIVE_EXTRA_BODY` optionally supplies a JSON object of provider
settings. The current-turn tool-history fixture contains no provider-specific
reasoning trace. DeepSeek's thinking mode rejects that synthetic control with
HTTP 400 before compaction, so run that fixture with thinking explicitly disabled
(or a provider configuration that accepts such history). For the evaluated routes:

```bash
GSH_CONTEXT_LIVE_EXTRA_BODY='{"thinking":{"type":"disabled"}}' \
  go test -count=1 -timeout=12m -v ./internal/script/interpreter \
  -run '^TestContextLiveFidelity$/completed_work_after_latest_request$'
```

This assumes the live opt-in, route, key and summary budget are already set.
Record any extra settings with results; do not count a failed control as a
summary-fidelity result. This setting is for the evaluation process and does
not alter application defaults.

The verbose output records synthetic memories, estimated input sizes and
summarization usage. It never prints model configuration or credentials. Save
the output together with the model/route, date, revision and repeat count.

## What the assertions measure

Five fixtures test:

1. A later user correction superseding the initial path/version and cancelling
   deployment authorization.
2. Exact identifiers at the start of a long tool result, a nonzero exit code at
   the end, and a still-pending command.
3. Chinese paths, identifiers and corrections.
4. Untrusted tool text asking the summarizer to falsify a successful deployment.
5. A completed action after the latest user request, while a subsequent tool
   result remains in the same turn. Its completed state must survive compaction
   and a later user turn without restoring the old pending-work request.

Each fixture checks five exact facts (`config_path`, `release_id`, `exit_code`,
`deployed`, `next_step`) against hand-written expected values in five modes:
uncompressed control, compacted plain response, compacted streaming response,
response after a second compaction, and the automatic-compaction agent path.
No model is used as a judge. A valid JSON answer must match all expected fields;
incorrect values and unexpected fields fail the test. Code-fenced JSON is
accepted so formatting alone does not obscure a fact-retention failure.

The deterministic chronology regressions check initial corrections, completed
work after a pinned user, repeated compaction, and script-pipe propagation
independently of a particular model. The fragment test checks task
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

## Review follow-up (2026-11-04)

Review of the initial PR found that placing all memory immediately after the
first user was insufficient during a long tool turn: a pinned later request
could appear after the memory of its completed action. Conversations now retain
the memory's insertion position. Subsequent summarization reconstructs pinned
requests and previous memory in chronological order. Regression tests cover
both compactions and passage through script pipes.

The review also fixed declaration-time validation of dynamic model windows,
normal-response truncation (including unfinished streaming tool arguments),
directory context on REPL retry, and minimum-framing/exact-budget boundaries.
Deterministic tests first reproduced the failures and now verify recovery,
zero execution of truncated tool batches, and zero paid calls for an impossible
minimum summary budget.

After these fixes, the final real-model recheck used temperature 0, the same
12,000 estimated window and 2,048 response reserve, a 4,096 summary limit, and
explicit `extraBody: {"thinking":{"type":"disabled"}}` on both routes:

| Route | Fixtures | Repeats | Exact answers | Exact fields |
| --- | --- | --- | --- | --- |
| `deepseek-flash` | All five | 1 | 25/25 | 125/125 |
| `deepseek-v4-pro` | Completed work after latest request; superseded initial request | 1 | 10/10 | 50/50 |

Both final test processes exited successfully. Flash's separate tool-continuation
test also passed with eight ordered reads, three automatic compactions, no
duplicate attempts, and `DONE`. It used the same explicit thinking setting;
its own budget remains 10,000/2,048/2,048 as defined in the test.

The first attempts with the original route defaults rejected the new synthetic
current-turn tool-history control with HTTP 400 for missing `reasoning_content`.
Those failed controls are excluded from the table. The optional extra-body
setting makes this protocol constraint explicit; it neither supplies a forged
reasoning trace nor changes production defaults. The earlier 80-answer baseline
above used the prior implementation and original route defaults. It is separate
historical evidence, not a claim that the full two-repeat matrix was rerun on
these review changes.

The final review tree also passed Linux/WSL `make ci` (including the native-npx
MCP integration tests), focused interpreter/default-middleware tests under
`-race`, and Linux/Windows builds. Full lint reported zero issues. Native
Windows targeted tests and the standalone `.gsh` smoke test passed as well.

## Follow-up review: oversized-history allocation benchmark

`BenchmarkContextLargeHistory` exercises the bounded-failure path with a 1 MiB
synthetic tool result, a 6,000-token estimated window and exactly 32 mocked
summary responses. It checks the call limit while measuring local work, not
model latency. Run it without credentials:

```bash
go test ./internal/script/interpreter -run '^$' \
  -bench '^BenchmarkContextLargeHistory$' -benchmem -benchtime=1x -count=3
```

On Windows/Go 1.25.14 (i5-12400F), the median of three single-iteration runs
changed from about 72.4 ms and 125.5 MB allocated to 8.8 ms and 12.2 MB
allocated. Candidate searches now bound their raw text by the input allowance
and avoid serializing a whole message that cannot possibly fit. These are
cumulative allocations in this synthetic failure path, not peak RAM or an
end-to-end speed guarantee. Lossless fragment reconstruction and request-budget
tests still apply.

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

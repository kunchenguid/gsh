# Search and fetch directly through MCP, without a model or API key.
mcp parallel {
    url: "https://search.parallel.ai/mcp",
    headers: {
        "User-Agent": "gsh (https://github.com/kunchenguid/gsh)",
    },
}

# Generate one random 32-character hex ID for this run and reuse it in both calls.
hex = "0123456789abcdef"
sessionID = ""
while (sessionID.length < 32) {
    digit = Math.floor(Math.random() * 16)
    sessionID = sessionID + hex.substring(digit, digit + 1)
}

searchResult = parallel.web_search({
    objective: "Find the official Go documentation for getting started with modules.",
    search_queries: ["Go modules getting started", "Go official modules tutorial"],
    session_id: sessionID,
})
print(searchResult)

# Fetch a known documentation page. Edit the URL and objective for your task.
fetchResult = parallel.web_fetch({
    urls: ["https://go.dev/doc/"],
    objective: "Find the documentation links for learning Go and using modules.",
    session_id: sessionID,
})
print(fetchResult)

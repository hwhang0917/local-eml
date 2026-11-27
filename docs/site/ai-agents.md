# AI agents (MCP)

Local Eml can let an AI agent you already use, such as Claude Desktop or Claude Code, search and read your archive. It does this through [MCP](https://modelcontextprotocol.io) (Model Context Protocol), the standard way agents connect to tools. Once it's set up, you can ask your agent things like:

- "Find the lease renewal email from my landlord and summarize what changed."
- "What did the March invoices from Northwind add up to? Read the PDFs."
- "When did I last hear from the moving company, and what did they say about the deposit?"

It's **off by default**. Turn it on under **Settings → AI agents**.

::: danger What an agent reads leaves your computer
Local Eml itself still sends nothing anywhere. But every email and attachment an agent reads is sent to that agent's AI provider (for example Anthropic for Claude, or OpenAI for ChatGPT) and handled under that provider's terms. Only connect agents you'd trust with this mail. An agent that runs a local model, for example through [Ollama](https://ollama.com), keeps it on your computer.
:::

## What agents can and can't do

Agents get read-only tools:

| Tool | What it does |
|---|---|
| `search_emails` | Searches with the same [operators as the search box](./search) (`from:`, `subject:`, `after:`, `has:attachment` …) |
| `read_email` | Reads one message: headers, text and its list of attachments |
| `get_thread` | Lists the rest of a message's conversation |
| `list_attachments` | Finds attachments across the archive by name, type, sender or date |
| `read_attachment` | Reads the text of a PDF, Word (`.docx`), Excel (`.xlsx`), PowerPoint (`.pptx`), Hangul (`.hwp`, `.hwpx`), OpenDocument, HTML or text file |

Agents **can't** delete, change, star, move or send anything. Mail you flagged as [spam or phishing](./spam-phishing) stays hidden from search, and its attachments can't be read.

Images, scanned PDFs and the old binary Office formats (`.doc`, `.xls`, `.ppt`) have no text an agent can read. Password-protected files can't be read either.

::: tip Emails can contain instructions aimed at AI
Anyone can send you an email that says "ignore your instructions and …". Local Eml tells agents to treat mail as data, and the tools can't change anything, but an agent that also has other tools (a browser, your files) could still be misled. Keep an eye on what it does after reading unfamiliar mail.
:::

## Connecting an agent

Turn on **Settings → AI agents**. The page then shows the setup for each kind of agent, with a **Copy** button.

### Claude Desktop

1. In Claude Desktop, open **Settings → Developer → Edit Config**.
2. Paste the config from Local Eml into `claude_desktop_config.json`. If the file already has an `"mcpServers"` block, add the `"local-eml"` entry inside it.
3. Quit Claude Desktop completely and open it again.

The config runs `local-eml mcp`, a small bridge that reads the agent token from your data folder and finds Local Eml on whatever port it's using. So the config file holds no secret, and it keeps working after you make a new token. Local Eml has to be running (the background service, or the app open).

If Local Eml runs inside WSL and Claude Desktop runs on Windows, the page gives you a config that starts the bridge through `wsl.exe`. Paste it as is.

### Claude Code

Run the command from the page once in a terminal:

```sh
claude mcp add --transport http local-eml http://127.0.0.1:7878/mcp --header "Authorization: Bearer <token>"
```

### Other agents

- Agents that connect over HTTP (the "streamable HTTP" transport) use the URL `http://127.0.0.1:7878/mcp` and send the token in an `Authorization: Bearer <token>` header.
- Agents that start MCP servers as a command (stdio) run `local-eml mcp`. Add `--port N` if Local Eml isn't on its usual port and the bridge can't find it.

The URL uses the port Local Eml is on. If it moved to another port because 7878 was busy, copy the URL again from Settings. The `local-eml mcp` command finds the port by itself.

## The agent token

The agent token is a separate password from your [session token](./settings-cli#session-token). It only opens the read-only agent tools, not the rest of Local Eml, and your browser's sign-in doesn't work for agents.

- **Show** reveals it. The setup commands copy it for you.
- **Make a new token** replaces it at once. Agents that connect over HTTP need the new one pasted in. The Claude Desktop setup picks it up by itself.
- Turning agent access off refuses every agent from the next request on. Turning it back on keeps the same token.

The switch and token are kept in `mcp.json` in your data folder, readable only by you. They aren't part of backups, so restoring a backup never turns agent access on.

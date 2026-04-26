# jira-thing — The Idiot's Guide 🧸

Imagine Jira is a big noticeboard at school where teachers pin up jobs that need doing. **jira-thing** is a little helper robot that can read the noticeboard, pin up new jobs, leave sticky-note comments, and tell you which jobs are yours — all from your terminal.

---

## What's in the box?

The project is written in **Go** (a programming language). Here's what lives where:

```
jira-thing/
│
├── main.go                        ← The boss. Decides what to do.
│
├── internal/
│   ├── api/
│   │   └── api.go                 ← The messenger. Talks to Jira.
│   │
│   ├── auth/
│   │   └── auth.go                ← The bouncer. Handles your password.
│   │
│   └── template/
│       └── template.go            ← The photocopier. Saves and loads templates.
│
├── Makefile                       ← The "build it" button.
├── go.mod                         ← The shopping list of libraries we need.
└── README.md                      ← The instruction manual.
```

---

## How it all fits together

```
 ┌──────────────────────────────────────────────────────────┐
 │                         YOU                              │
 │                  (typing commands)                       │
 └────────────────────────┬─────────────────────────────────┘
                          │
                          ▼
 ┌──────────────────────────────────────────────────────────┐
 │                      main.go                             │
 │                                                          │
 │  "What did you ask me to do?"                            │
 │                                                          │
 │   template?  → runTemplate()                             │
 │   create?    → runCreate()                               │
 │   update?    → runUpdate()                               │
 │   my-tasks?  → runMyTasks()                              │
 │   clear-auth?→ auth.ClearCredentials()                   │
 └──────┬──────────────┬──────────────────┬─────────────────┘
        │              │                  │
        ▼              ▼                  ▼
 ┌────────────┐ ┌────────────┐   ┌──────────────┐
 │  auth.go   │ │  api.go    │   │ template.go  │
 │  "Who are  │ │  "I'll     │   │ "I'll save   │
 │   you?"    │ │   talk to  │   │  and load    │
 │            │ │   Jira"    │   │  JSON files" │
 └─────┬──────┘ └─────┬──────┘   └──────────────┘
       │               │
       ▼               ▼
 ┌───────────┐  ┌─────────────┐
 │ OS        │  │   Jira      │
 │ Keychain  │  │   (cloud)   │
 │ 🔐        │  │   📋        │
 └───────────┘  └─────────────┘
```

---

## File-by-file: what does each one actually do?

### `main.go` — The Boss

This is where the programme starts. Think of it as the receptionist at a hotel.

You walk in and say a word. The receptionist hears that word and sends you to the right room:

| You say | The receptionist sends you to |
|---|---|
| `template` | `runTemplate()` — go photocopy a ticket |
| `create` | `runCreate()` — go make a new ticket |
| `update` | `runUpdate()` — go leave a sticky-note comment |
| `my-tasks` | `runMyTasks()` — go check your to-do list |
| `clear-auth` | `auth.ClearCredentials()` — forget who you are |

If you say something it doesn't recognise, it prints a help message and closes the door.

#### Key functions in main.go

- **`runTemplate()`** — Asks Jira "show me ticket PROJ-42", then picks out the useful bits (project, type, priority, labels, etc.) and saves them to a JSON file. Like taking a photo of a well-made sandwich so you can make more later.

- **`runCreate()`** — Loads that saved JSON file, asks you "what's the title?" and "what's the description?", then sends it all to Jira to make a brand new ticket. Like using a cookie cutter — same shape, different cookie.

- **`runUpdate()`** — Opens your text editor (or reads from a pipe) so you can write a comment, then posts it to Jira. Like sticking a Post-it note on someone's job card.

- **`runMyTasks()`** — Asks Jira "what's assigned to me that isn't done yet?" and prints a nice table. The `-notupdated` flag filters to only show stale ones (no activity in 3+ business days).

- **`buildDescription()`** — Jira doesn't accept plain text. It wants a special format called "Atlassian Document Format" (ADF). This function wraps your text in that format. Think of it as putting your letter in an envelope before posting it.

- **`threeBusinessDaysAgo()`** — Counts backwards 3 weekdays from today, skipping Saturday and Sunday. Used to find stale tickets.

---

### `internal/api/api.go` — The Messenger

This file is the **only** file that talks to Jira over the internet. Nobody else is allowed to. It knows four tricks:

```
 api.go knows how to:
 ┌─────────────────────────────────────────────────────────┐
 │                                                         │
 │  FetchIssue()    GET  /rest/api/3/issue/PROJ-42         │
 │  "Go read this ticket and bring it back"                │
 │                                                         │
 │  CreateIssue()   POST /rest/api/3/issue                 │
 │  "Go pin this new ticket on the board"                  │
 │                                                         │
 │  AddComment()    POST /rest/api/3/issue/PROJ-42/comment │
 │  "Go stick this note on that ticket"                    │
 │                                                         │
 │  SearchIssues()  POST /rest/api/3/search/jql            │
 │  "Go find all tickets matching this question"           │
 │                                                         │
 └─────────────────────────────────────────────────────────┘
```

Under the hood, every request goes through two helper functions:

1. **`newAuthRequest()`** — Builds an HTTP request and stamps your email + API token on it (Basic Auth). Like writing your name on the back of every letter.
2. **`executeRequest()`** — Actually sends the request, checks the response isn't an error, and reads the JSON that comes back.

The `JiraConnection` struct is just a little bag holding your Jira URL, email, and token — passed around so every function knows where to send things.

---

### `internal/auth/auth.go` — The Bouncer

This file handles your Jira credentials (URL, email, API token). It stores them in your **operating system's keychain** — the same secure vault that stores your Wi-Fi passwords.

```
 First time you run jira-thing:
 ┌──────────┐     "Who are you?"     ┌──────────┐
 │ auth.go  │ ──────────────────────▶│   You    │
 │          │ ◀──────────────────────│          │
 │          │   "Here's my details"  └──────────┘
 │          │
 │          │──── stores in keychain 🔐
 └──────────┘

 Every time after that:
 ┌──────────┐     checks keychain    ┌──────────┐
 │ auth.go  │ ──────────────────────▶│ Keychain │
 │          │ ◀──────────────────────│   🔐     │
 │          │   "Found them!"        └──────────┘
 └──────────┘
```

- **`GetCredentials()`** — Tries to read your details from the keychain. If they're not there, it asks you to type them in, then saves them.
- **`ClearCredentials()`** — Deletes everything from the keychain. Like shredding your ID badge.
- **`readPassword()`** — Reads the API token without showing it on screen (those `••••••` dots).

It uses a `Keyring` interface so that tests can swap in a fake keychain instead of touching the real one.

---

### `internal/template/template.go` — The Photocopier

This file deals with JSON template files — the "cookie cutters" for new tickets.

- **`Build()`** — Takes a full Jira issue (which has loads of fields) and picks out just the reusable ones: project, issue type, priority, labels, components, and assignee. Everything else gets thrown away.

- **`Save()`** — Writes the template to a `.json` file on disk. Defaults to `ticket_template.json` if you don't specify a name.

- **`Load()`** — Reads a `.json` file back into memory so `runCreate()` can use it.

```
 Full Jira ticket (50+ fields)
 ┌──────────────────────────────┐
 │ project, issuetype, priority │──▶ Build() picks these
 │ labels, components, assignee │
 │ created, updated, reporter,  │
 │ description, comments, ...   │──▶ These get thrown away
 └──────────────────────────────┘
```

---

### `Makefile` — The "Build It" Button

Four commands, dead simple:

| Command | What it does |
|---|---|
| `make build` | Compiles the Go code into a binary called `jira-thing` |
| `make test` | Runs all the tests |
| `make vet` | Runs Go's built-in code checker |
| `make tidy` | Cleans up the dependency list |

---

### `go.mod` — The Shopping List

Lists the external libraries the project needs:

| Library | Why |
|---|---|
| `github.com/zalando/go-keyring` | Talk to the OS keychain (macOS Keychain, Windows Credential Manager, etc.) |
| `golang.org/x/term` | Read passwords from the terminal without showing them |

That's it. Two dependencies. Nice and small.

---

## How a command flows through the code

Here's what happens when you type `jira-thing update PROJ-42 -stdin`:

```
 1. main() sees "update"
    │
    ▼
 2. runUpdate() is called with ["PROJ-42", "-stdin"]
    │
    ├── Parses flags: -stdin is true, ticket key is "PROJ-42"
    │
    ├── Reads your comment text from stdin (readAllStdin)
    │
    ├── Checks it's not empty
    │
    ├── mustConnect()
    │   └── auth.GetCredentials()
    │       └── Reads URL/email/token from keychain
    │
    ├── api.AddComment(conn, "PROJ-42", commentBody)
    │   └── POST https://yourorg.atlassian.net/rest/api/3/issue/PROJ-42/comment
    │       with {"body": { ADF document }}
    │
    └── Prints "Comment added to PROJ-42"
```

---

## How the tests work

Every file has a matching `_test.go` file. Tests use:

- **`httptest.NewServer`** — A fake Jira server that runs locally. The tests send real HTTP requests to it, but nothing goes to the internet.
- **Mock keyring** — A fake keychain stored in a Go map, so tests don't touch your real passwords.
- **`captureStdout` / `captureStderr`** — Temporarily redirect output to a buffer so tests can check what was printed.
- **`captureExit`** — Catches `os.Exit` calls (via panic/recover) so a test can verify the programme would have exited without actually killing the test process.

No real Jira connection or OS keychain is ever touched during tests. Everything is faked.

---

## Glossary for the truly lost

| Term | Plain English |
|---|---|
| **Go** | The programming language this is written in. Made by Google. |
| **Binary** | The compiled programme you can run. Like an `.exe` on Windows. |
| **JSON** | A text format for structured data. Looks like `{"key": "value"}`. |
| **API** | A way for programmes to talk to each other over the internet. |
| **REST API** | A specific style of API that uses URLs and HTTP methods (GET, POST, PUT). |
| **JQL** | Jira Query Language. Like SQL but for finding Jira tickets. |
| **ADF** | Atlassian Document Format. Jira's way of storing rich text. |
| **Keychain** | Your OS's built-in password vault. |
| **Basic Auth** | Sending username:password with every request (base64 encoded). |
| **`httptest`** | Go's built-in library for creating fake HTTP servers in tests. |
| **Struct** | A Go data type that groups related values together. Like a form with fields. |
| **Interface** | A Go contract that says "anything that has these methods counts". Used to swap real things for fakes in tests. |

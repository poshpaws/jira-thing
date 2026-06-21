# Data Flow Diagrams

Yourdon–DeMarco notation (adapted for Mermaid):

| Symbol | Mermaid shape | Meaning |
|--------|---------------|---------|
| Rectangle | `[Name]` | External entity |
| Circle | `((P#: Name))` | Process |
| Open rectangle | `[(D#: Name)]` | Data store |
| Arrow + label | `-->|"label"|` | Data flow |

---

## Level 0 — Context Diagram

Shows the entire system as a single process and all external actors.

```mermaid
flowchart LR
    classDef ext fill:#dae8fc,stroke:#6c8ebf,color:#000
    classDef proc fill:#d5e8d4,stroke:#82b366,color:#000

    U([User / Operator]):::ext
    JC([Jira Cloud API]):::ext
    CF([Confluence API]):::ext
    GH([GitHub Releases]):::ext
    ED(["\$EDITOR"]):::ext
    KR([OS Keyring]):::ext
    FS([Local Filesystem]):::ext

    SYS(("jira-thing\nCLI")):::proc

    U     -->|"subcommand + flags + text"| SYS
    SYS   -->|"ticket data, status, rendered output"| U
    SYS  <-->|"REST v3 · Basic Auth · JSON"| JC
    SYS  <-->|"Confluence REST · storage-format HTML"| CF
    SYS   -->|"GET /releases/latest"| GH
    GH    -->|"version JSON"| SYS
    SYS  <-->|"get/set/delete credentials"| KR
    SYS  <-->|"read/write template + config JSON"| FS
    SYS   -->|"open temp file"| ED
    ED    -->|"edited text (saved temp file)"| SYS
```

---

## Level 1 — System Decomposition

Decomposes the system into its seven internal subsystems and four data stores.

```mermaid
flowchart TD
    classDef ext   fill:#dae8fc,stroke:#6c8ebf,color:#000
    classDef proc  fill:#d5e8d4,stroke:#82b366,color:#000
    classDef store fill:#fff2cc,stroke:#d6b656,color:#000

    %% External entities
    U([User]):::ext
    JC([Jira Cloud API]):::ext
    CF([Confluence API]):::ext
    GH([GitHub Releases]):::ext
    ED(["\$EDITOR"]):::ext

    %% Data stores
    D1[(D1: OS Keyring\nURL · email · token)]:::store
    D2[(D2: Template JSON\nticket_template.json)]:::store
    D3[(D3: Config JSON\n~/.config/jira-thing)]:::store
    D4[(D4: Temp Edit File\n/tmp/jira-thing-*.txt)]:::store

    %% Processes
    P1(("P1\nCLI Dispatch\nmain.go")):::proc
    P2(("P2\nCredential\nManager\ninternal/auth")):::proc
    P3(("P3\nJira API\nClient\ninternal/api")):::proc
    P4(("P4\nTemplate\nEngine\ninternal/template")):::proc
    P5(("P5\nConfluence\nClient\ninternal/api\n/confluence")):::proc
    P6(("P6\nConfig\nLoader\ninternal/config")):::proc
    P7(("P7\nTUI Selector\ninternal/tui")):::proc
    P8(("P8\nComment\nRenderer\nrender.go")):::proc

    %% User ↔ CLI
    U       -->|"subcommand + args + text input"| P1
    P1      -->|"status messages + formatted output"| U

    %% P1 ↔ Credential Manager
    P1      -->|"credential request"| P2
    P2     <-->|"Get/Set/Delete keys"| D1
    P2      -->|"prompt: enter URL / email / token"| U
    U       -->|"URL, email, API token"| P2
    P2      -->|"JiraConnection{BaseURL, Email, Token}"| P1

    %% P1 ↔ Config Loader
    P1      -->|"Load()"| P6
    P6     <-->|"ReadFile"| D3
    P6      -->|"Config{Project, ToilMarker, Editor, …}"| P1

    %% P1 ↔ Jira API Client
    P1      -->|"FetchIssue / CreateIssue / SearchIssues\nAddComment / FetchLastComment / FetchMyself"| P3
    P3     <-->|"HTTPS REST v3 · Basic Auth · JSON body"| JC
    P3      -->|"issue map / SearchResult / Comment"| P1

    %% P1 ↔ Template Engine
    P1      -->|"Build(issueData)"| P4
    P1      -->|"Save(tmpl, path)"| P4
    P1      -->|"Load(path)"| P4
    P4     <-->|"ReadFile / WriteFile JSON"| D2
    P4      -->|"template map[string]any"| P1

    %% P1 ↔ Confluence Client
    P1      -->|"FetchConfluencePage / ListChildPages\nCreateConfluencePage / UpdateConfluencePage"| P5
    P5     <-->|"HTTPS REST · Basic Auth · storage-format HTML"| CF
    P5      -->|"ConfluencePage / ConfluencePageWithBody"| P1

    %% P1 ↔ TUI Selector
    P1      -->|"[]Ticket (key, summary, status, priority, updated)"| P7
    P7      -->|"keyboard events display"| U
    U       -->|"row selection + confirm"| P7
    P7      -->|"[]Ticket (selected)"| P1

    %% P1 ↔ Comment Renderer
    P1      -->|"Comment{Body ADF JSON}"| P8
    P8      -->|"styled terminal markdown"| U

    %% P1 ↔ Editor
    P1     <-->|"WriteFile / ReadFile"| D4
    P1      -->|"exec($EDITOR, tempfile)"| ED
    ED      -->|"saved file (via OS)"| D4

    %% Version check
    P1      -->|"GET /repos/…/releases/latest"| GH
    GH      -->|"tag_name, html_url"| P1
```

---

## Level 2 — Command Flows

Each diagram traces data movement for one top-level command.

### 2.1 `template <KEY> [-o file]`

Fetches a ticket and extracts a reusable JSON template.

```mermaid
sequenceDiagram
    actor U as User
    participant P1 as CLI Dispatch
    participant P2 as Credential Manager
    participant D1 as OS Keyring
    participant P3 as Jira API Client
    participant JC as Jira Cloud
    participant P4 as Template Engine
    participant D2 as Template JSON

    U->>P1: jira-thing template PROJ-123 [-o path]
    P1->>P2: GetCredentials()
    P2->>D1: Get(jira_url, jira_email, jira_api_token)
    D1-->>P2: stored values (or ErrNotFound)
    alt missing credentials
        P2->>U: prompt URL / email / token
        U->>P2: credential values
        P2->>D1: Set(key, value) × 3
    end
    P2-->>P1: JiraConnection
    P1->>P3: FetchIssue(conn, "PROJ-123")
    P3->>JC: GET /rest/api/3/issue/PROJ-123?fields=*all
    JC-->>P3: issue JSON (fields: project, issuetype, priority, labels, components, assignee, …)
    P3-->>P1: map[string]any
    P1->>P4: Build(issueData)
    P4-->>P1: template map (stripped to reusable fields)
    P1->>U: prompt: assignee self or original?
    U->>P1: choice (1/2)
    P1->>P4: Save(tmpl, outputPath)
    P4->>D2: WriteFile ticket_template.json
    P4-->>P1: saved path
    P1->>U: "Template saved to <path>" + JSON preview
```

---

### 2.2 `create [-t template.json]`

Creates a new Jira ticket from a template.

```mermaid
sequenceDiagram
    actor U as User
    participant P1 as CLI Dispatch
    participant P4 as Template Engine
    participant D2 as Template JSON
    participant P2 as Credential Manager
    participant P3 as Jira API Client
    participant JC as Jira Cloud

    U->>P1: jira-thing create [-t path]
    P1->>P4: Load(path)
    P4->>D2: ReadFile → strip excluded custom fields
    D2-->>P4: raw JSON
    P4-->>P1: template map[string]any
    P1->>U: prompt: summary?
    U->>P1: summary text
    P1->>U: prompt: description?
    U->>P1: description text
    P1->>P1: inject summary + buildDescription(ADF doc)
    P1->>P2: GetCredentials()
    P2-->>P1: JiraConnection
    alt assignee == __SELF__
        P1->>P3: FetchMyself(conn)
        P3->>JC: GET /rest/api/3/myself
        JC-->>P3: {accountId, …}
        P3-->>P1: accountId
        P1->>P1: ResolveAssignee(tmpl, accountId)
    end
    P1->>P3: CreateIssue(conn, fields)
    P3->>JC: POST /rest/api/3/issue  {fields: …}
    JC-->>P3: {key: "PROJ-456", id: …}
    P3-->>P1: result map
    P1->>U: "Created ticket: PROJ-456\nURL: …"
```

---

### 2.3 `update <KEY> [-stdin]`

Adds a comment to an existing ticket via `$EDITOR` or stdin.

```mermaid
flowchart TD
    classDef ext   fill:#dae8fc,stroke:#6c8ebf,color:#000
    classDef proc  fill:#d5e8d4,stroke:#82b366,color:#000
    classDef store fill:#fff2cc,stroke:#d6b656,color:#000

    U([User]):::ext
    ED(["\$EDITOR"]):::ext
    JC([Jira Cloud]):::ext
    D4[(D4: Temp File)]:::store
    P1(("P1\nCLI Dispatch")):::proc
    P3(("P3\nJira API Client")):::proc

    U -->|"update PROJ-123 [-stdin]"| P1
    P1 -->|"stdin mode?"| DEC{"-stdin flag?"}
    DEC -->|"yes"| P1b["read all of stdin"]
    DEC -->|"no"| P1c["WriteFile empty temp"]
    P1c -->|"exec editor + tempfile path"| ED
    ED -->|"user saves and closes"| D4
    D4 -->|"ReadFile"| P1
    P1b -->|"comment text"| P1
    P1 -->|"buildDescription(text) → ADF doc"| P1
    P1 -->|"AddComment(conn, key, adfBody)"| P3
    P3 -->|"POST /rest/api/3/issue/PROJ-123/comment"| JC
    JC -->|"201 Created"| P3
    P3 -->|"nil error"| P1
    P1 -->|"Comment added to PROJ-123\nURL: …"| U
```

---

### 2.4 `last-comment <KEY>`

Fetches and renders the most recent comment as terminal markdown.

```mermaid
flowchart LR
    classDef ext   fill:#dae8fc,stroke:#6c8ebf,color:#000
    classDef proc  fill:#d5e8d4,stroke:#82b366,color:#000

    U([User]):::ext
    JC([Jira Cloud]):::ext
    P1(("P1\nCLI Dispatch")):::proc
    P3(("P3\nJira API Client")):::proc
    P8(("P8\nComment Renderer\nrender.go")):::proc

    U       -->|"last-comment PROJ-123"| P1
    P1      -->|"FetchLastComment(conn, key)"| P3
    P3      -->|"GET /comment?maxResults=1 → read total"| JC
    JC      -->|"CommentResult{Total: N}"| P3
    P3      -->|"GET /comment?startAt=N-1&maxResults=1"| JC
    JC      -->|"CommentResult{Comments:[Comment]}"| P3
    P3      -->|"Comment{Author, Body ADF, Created}"| P1
    P1      -->|"Comment"| P8
    P8      -->|"adfToMarkdown() → glamour.Render()"| P8
    P8      -->|"styled terminal markdown"| U
```

---

### 2.5 `my-tasks [-notupdated]`

Lists open tickets assigned to the current user.

```mermaid
flowchart LR
    classDef ext   fill:#dae8fc,stroke:#6c8ebf,color:#000
    classDef proc  fill:#d5e8d4,stroke:#82b366,color:#000

    U([User]):::ext
    JC([Jira Cloud]):::ext
    P1(("P1\nCLI Dispatch")):::proc
    P3(("P3\nJira API Client")):::proc

    U  -->|"my-tasks [-notupdated]"| P1
    P1 -->|"buildMyTasksJQL(notUpdated)"| P1
    P1 -->|"SearchIssues(conn, SearchQuery{JQL, fields, maxResults:100})"| P3
    P3 -->|"POST /rest/api/3/search/jql  {jql, fields, maxResults}"| JC
    JC -->|"SearchResult{Issues[], Total}"| P3
    P3 -->|"SearchResult"| P1
    P1 -->|"printTaskRow × N  (key, status, priority, updated, summary)"| U
```

---

### 2.6 `toil-sync`

Queries TOIL tickets, lets user select via TUI, then syncs each to Confluence.

```mermaid
flowchart TD
    classDef ext   fill:#dae8fc,stroke:#6c8ebf,color:#000
    classDef proc  fill:#d5e8d4,stroke:#82b366,color:#000
    classDef store fill:#fff2cc,stroke:#d6b656,color:#000

    U([User]):::ext
    JC([Jira Cloud]):::ext
    CF([Confluence API]):::ext
    D3[(D3: Config JSON)]:::store

    P1(("P1\nCLI Dispatch")):::proc
    P3(("P3\nJira API Client")):::proc
    P5(("P5\nConfluence Client")):::proc
    P6(("P6\nConfig Loader")):::proc
    P7(("P7\nTUI Selector")):::proc

    U  -->|"toil-sync"| P1
    P1 -->|"Load()"| P6
    P6 -->|"ReadFile"| D3
    P6 -->|"Config{Project, ToilMarker, ToilTeam,\nConfluenceSpace, TicketHanger}"| P1
    P1 -->|"SearchIssues(conn, JQL: project+labels+unresolved)"| P3
    P3 -->|"POST /rest/api/3/search/jql"| JC
    JC -->|"SearchResult{Issues[]}"| P3
    P3 -->|"SearchResult"| P1
    P1 -->|"issuesToTickets() → []Ticket"| P7
    P7 -->|"bubble-tea TUI table"| U
    U  -->|"space=toggle, enter=confirm"| P7
    P7 -->|"[]Ticket (selected)"| P1
    P1 -->|"FetchConfluencePage(space, ticketHanger)"| P5
    P5 -->|"GET /wiki/rest/api/content?spaceKey=…&title=…"| CF
    CF -->|"ConfluencePage{ID, Title, Version}"| P5
    P5 -->|"ConfluencePage (hanger)"| P1
    P1 -->|"ListChildPages(hanger.ID)"| P5
    P5 -->|"GET /wiki/rest/api/content/{id}/child/page"| CF
    CF -->|"[]ConfluencePageWithBody"| P5
    P5 -->|"childMap[title]→ConfluencePageWithBody"| P1
    subgraph LOOP["for each selected ticket"]
        P1 -->|"renderTicketPage(issue, notes)"| P1
        P1 -->|"CreateConfluencePage or UpdateConfluencePage"| P5
        P5 -->|"POST / PUT /wiki/rest/api/content"| CF
        CF -->|"200/201"| P5
        P5 -->|"ConfluencePage"| P1
    end
    P1 -->|"UpdateConfluencePage(hanger, renderHangerPage(issues, manualPages))"| P5
    P5 -->|"PUT /wiki/rest/api/content/{hangerID}"| CF
    P1 -->|"Synced N ticket(s) to 'Hanger Title'"| U
```

---

### 2.7 `diagnose`

Tests API connectivity and credential resolution end-to-end.

```mermaid
flowchart LR
    classDef ext   fill:#dae8fc,stroke:#6c8ebf,color:#000
    classDef proc  fill:#d5e8d4,stroke:#82b366,color:#000

    U([User]):::ext
    JC([Jira Cloud]):::ext
    P1(("P1\nCLI Dispatch")):::proc
    P2(("P2\nCredential Manager")):::proc
    P3(("P3\nJira API Client")):::proc

    U  -->|"diagnose [--url URL] [--email E] [--token T]"| P1
    P1 -->|"flag overrides or GetCredentials()"| P2
    P2 -->|"JiraConnection"| P1
    P1 -->|"FetchMyself(conn)"| P3
    P3 -->|"GET /rest/api/3/myself"| JC
    JC -->|"200 {displayName, accountId, emailAddress}"| P3
    P3 -->|"user map"| P1
    P1 -->|"connectivity OK + user details"| U
```

---

### 2.8 Credential Resolution Flow (cross-command)

Shared sub-process invoked by every command that calls `mustConnect()`.

```mermaid
flowchart TD
    classDef ext   fill:#dae8fc,stroke:#6c8ebf,color:#000
    classDef proc  fill:#d5e8d4,stroke:#82b366,color:#000
    classDef store fill:#fff2cc,stroke:#d6b656,color:#000

    U([User]):::ext
    D1[(D1: OS Keyring\njira-thing-poc service)]:::store
    P2(("P2\nCredential Manager\ninternal/auth")):::proc

    P2  -->|"Get(jira_url)"| D1
    P2  -->|"Get(jira_email)"| D1
    P2  -->|"Get(jira_api_token)"| D1
    D1  -->|"value or ErrNotFound"| P2
    P2  -->|"all present?"| CHK{all\npresent?}
    CHK -->|"yes"| OUT["Credentials{URL, Email, Token}"]
    CHK -->|"no"| PROMPT["prompt user for missing values"]
    PROMPT -->|"request: URL / email / API token"| U
    U   -->|"credential values"| PROMPT
    PROMPT -->|"Set(jira_url, value)"| D1
    PROMPT -->|"Set(jira_email, value)"| D1
    PROMPT -->|"Set(jira_api_token, value)"| D1
    PROMPT --> OUT
    OUT -->|"JiraConnection{BaseURL, Email, APIToken}"| P2
```

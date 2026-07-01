# SST — Command-line reference

## NAME

**sst** — Semantic STEP Technology core command-line interface for repositories, datasets, and RDF stages

## SYNOPSIS

**sst** [**-h** | **--help**]

**sst** **interactive**

**sst** **version**

Inside interactive mode (after the `sst >` prompt), commands are free-form lines; see **INTERACTIVE SYNTAX** below.

## DESCRIPTION

**sst** is a low-level CLI used to debug and exercise the SST Core API against local or remote SST repositories, SuperRepositories, datasets, mutable **stages**, **named graphs**, and **IBNodes**. Most work happens in **interactive mode**: a terminal session that prints a `sst >` prompt, reads one line of input, runs that command, prints any output, and repeats (using the readline library when available for editing, history, and Tab completion). Resources get short aliases (`r1`, `d1`, …) and operations often use the `alias.command` pattern.

The default invocation (**sst** with no subcommand) starts interactive mode, equivalent to **sst** **interactive**.

For a narrative guide, examples, and background, see **CLI_INTRODUCTION.md**.

## OPTIONS

The top-level **sst** command is implemented with Cobra. Only the following generic options apply at the root; subcommands do not currently add their own flags beyond Cobra’s defaults.

**-h**, **--help**  
Print help for **sst** (and, when shown for the root command, a short footer pointing to interactive mode).

Cobra may also accept **--help** on subcommands (e.g. `sst interactive --help`).

There is **no** top-level **-v** / **--version** flag; use **sst** **version** instead.

## COMMANDS

**interactive**  
Start **interactive mode**: print the same startup line as a bare **sst** run(read lines after `sst >`, dispatch commands, until `q`). Behavior matches **sst** with **no** subcommand; **sst interactive** is only an explicit way to enter that mode from the shell. No arguments.

**version**  
Print one line of version information and exit. Implementation: `cli/cmd/version.go`. No arguments.

## INTERACTIVE SYNTAX

**Prompt**  
The prompt is `sst > `.

**Quitting**  
Line `q` exits interactive mode and ends the process (after closing the readline session).

**Tokens**  
Standalone commands use the first whitespace-delimited token as the command name; remaining tokens are arguments. For `alias.command` lines, the first token must contain a `.` (e.g. `r1.info`); the part before the first `.` is the **alias**, the part after is the subcommand and its arguments (parsed with quote-aware splitting).

**Case**  
Standalone keywords such as `help` are matched case-insensitively. The **subcommand** name after the dot is normalized with `strings.ToLower` (so `r1.commitInfo` and `r1.commitinfo` both resolve to the same handler).

**Optional resource alias**  
Many opening and checkout commands accept **-a** *alias* to choose the alias for a new repository, stage, etc. If **-a** is omitted, the CLI picks the next free default (`r1`, `r2`, … / `s1`, …).

**Grammar (summary)**  

| Form | Meaning |
|------|---------|
| *word* *args…* | Standalone command (e.g. `help`, `status`, `openlocalrepository`). |
| *alias*.*subcommand* *args…* | Operation on an opened resource (e.g. `r1.info`, `d1.history`). |

**Readline**  
When the readline library initializes successfully: Tab completion for top-level commands, aliases, and `alias.` subcommands; history file (see **FILES**). **Ctrl+D** sends EOF and exits. **Ctrl+C** aborts the current line and reminds you to use `q` to quit. If readline fails to start, the CLI falls back to a simple reader (no history/completion).

## INTERACTIVE COMMANDS

The following tables mirror the built-in `help` output and the dispatch logic in `cli/cmd/interactive`. Arguments in angle brackets are required unless noted.

### Session

| Command | Description |
|---------|-------------|
| **q** | Exit interactive mode. |
| **help** | Print the full command list (same categories as below). |
| **status** | List opened repositories, datasets, stages, named graphs, IBNodes and aliases. |

### Opening and imports (no leading alias)

| Command | Description |
|---------|-------------|
| **openlocalrepository** *path* [**-a** *alias*] | Open a local SST repository. |
| **openlocalflatrepository** *path* [**-a** *alias*] | Open a local “flat” repository (directory of `.sst` files). |
| **openremoterepository** *URL* [**-a** *alias*] | Open a remote repository (TLS; auth via provider / credentials; see **ENVIRONMENT**). |
| **openlocalsuperrepository** *path* [**-a** *alias*] | Open a local SuperRepository. |
| **openremotesuperrepository** *URL* [**-a** *alias*] | Open a remote SuperRepository. |
| **rdfread** *file* | Load Turtle or TriG into a **new** stage (new alias). |
| **sstread** *file* | Load an SST binary file into a **new** stage (new alias). |
| **importap242xml** *file* | Import AP242 XML into a new stage. |
| **importp21** *file* | Import STEP P21 into a new stage. |

### *repo* — repository alias

| Command | Description |
|---------|-------------|
| **info** | Repository metadata (sizes, counts, remote flags, index, …). |
| **close** | Close this repository in the session. |
| **superrepository** | If this repo belongs to a SuperRepository, show that linkage/info. |
| **datasets** | List datasets. |
| **dataset** *iri* | Open dataset by IRI (new dataset alias). |
| **query** *bleve-query* [**--limit** *n*] | Bleve full-text query over the repository index. |
| **listfield** | List indexed Bleve field names. |
| **log** [**-v** \| **--verbose**] | Repository commit log; verbose for details. |
| **commitInfo** *commit-hash* | Details for one commit (camel or lower case accepted). |
| **commitdiff** *commit-hash* | NamedGraph-level diff vs parent(s): added / modified (triple diff) / deleted graphs. |
| **checkoutcommit** *hash* [**-a** *stage-alias*] | Materialize a stage at a repository commit (all NamedGraphs affected by the commit). |
| **extractsstfile** *hash* | Extract raw SST bytes for a NamedGraphRevision hash. |
| **dump** *bucket-key*[**/***sub-key*] | Dump internal Bolt buckets (**ngr**, **dsr**, **c**, **ds**, **dl**); dangerous on production data. |
| **openstage** | Create an empty stage on this repo. |
| **syncfrom** *source-repo-alias* [*branch*] [*dataset* …] | Copy/sync from another open repository (see in-app usage text for branch and dataset selection). |
| **clone** *target-directory* | Clone repository to a local directory. |
| **documentinfo** *hash* | Document metadata. |
| **documents** | List documents. |
| **documentdelete** *hash* | Delete document by hash. |
| **documentset** *file* | Upload a document file. |
| **documentget** *hash* *output-path* | Download document by hash. |

**dump** bucket keys: **ngr** (NamedGraphRevisions), **dsr** (DatasetRevisions), **c** (Commits), **ds** (Datasets), **dl** (DatasetLog).

### *superrepo* — SuperRepository alias

| Command | Description |
|---------|-------------|
| **info** | SuperRepository info. |
| **close** | Close SuperRepository session. |
| **list** | List contained repositories. |
| **get** *repo-name* | Open a member repository (gets a normal *r** alias). |
| **create** *repo-name* | Create member repository. |
| **delete** *repo-name* | Delete member repository. |

### *dataset* — dataset alias

| Command | Description |
|---------|-------------|
| **listcommits** [**--details**] | Walk history from leaf commits; optional detailed rows. |
| **commitdetailsbyhash** *hash* | Show commit details by hash. |
| **commitdetailsbybranch** *branch* | Show commit details by branch. |
| **branches** | List branches and commit hashes. |
| **leafcommits** | List leaf commit hashes only. |
| **checkoutcommit** *hash* [**-a** *stage-alias*] | Materialize a stage at a commit (single dataset scope). |
| **checkoutrevision** *hash* [**-a** *stage-alias*] | Materialize a stage at a dataset revision. |
| **checkoutbranch** *branch* [**-a** *stage-alias*] | Materialize a stage at a branch tip. |
| **setbranchcommit** *commit-hash* *branch* | Set a branch to point to a commit. |
| **setbranchrevision** *dataset-revision-hash* *branch* | Set a branch to point to a dataset revision. |
| **removebranch** *branch* | Remove a branch from the dataset. |
| **diff** *NGR-hash1* *NGR-hash2* | Triple-level diff between two NamedGraphRevision hashes. |
| **history** | Print/visualize dataset commit history graph. |

### *stage* — stage alias

| Command | Description |
|---------|-------------|
| **info** | Stage statistics (graphs, triples, …). |
| **namedgraphs** | List named graphs in the stage. |
| **referencednamedgraphs** | List referenced named graphs. |
| **namedgraph** *iri* | Resolve graph by IRI (new graph alias). |
| **moveandmerge** *source-stage-alias* | Merge graphs from another open stage into this stage. |
| **alignhistory** *from-stage-alias* | Copy repository pointer and checkout metadata from *from-stage-alias* onto this stage (e.g. after external RDF edit and `rdfread`). Does not merge triples or unregister the source stage. |
| **commit** *message* [*branch*] | Commit staged changes. |
| **validate** | RDF / domain-range validation; detailed report with NamedGraph sections and triples per finding. |
| **rdfwrite** *file* | Write stage as TriG. |
| **trig** | Print stage TriG to stdout. |
| **writesstfilesdirectory** *directory* | Write modified NamedGraphs as SST binary files into *directory*. |

### *namedgraph* — named graph alias

| Command | Description |
|---------|-------------|
| **info** | Graph metadata and revision pointers. |
| **foririnodes** | List IRI nodes. |
| **forallibnodes** | List IRI and blank nodes. |
| **forblanknodes** | List blank nodes only. |
| **getirinodebyfragment** *id* | Resolve IRI node by fragment id. |
| **getblanknodebyfragment** *id* | Resolve blank node by fragment id. |
| **rdfwrite** *file* | Write Turtle for this graph. |
| **sstwrite** *file* | Write SST binary for this graph. |
| **exportap242xml** *file.xml* | Export AP242 XML. |
| **ttl** | Print Turtle to stdout. |

### *ibnode* — IBNode alias

| Command | Description |
|---------|-------------|
| **forall** | List triples in which this IBNode appears. |

### *info* on multiple kinds

The **info** subcommand is dispatched by alias type: SuperRepository, repository, stage, or named graph (not dataset in the current code path).

## ENVIRONMENT

**`.env.sst-cli`** (gitignored, local only)  
Loaded at startup from `./.env.sst-cli` or `$HOME/.env.sst-cli`. Shell variables already set are not overwritten.

**File format**

- One variable per line: `KEY=VALUE`
- Empty lines and lines starting with `#` are ignored
- Values may be double- or single-quoted: `KEY="value"`

**Required** (remote repository authentication)

| Variable | Format |
|----------|--------|
| **SST_OIDC_REALM_URL** | Keycloak realm base URL, no trailing slash (e.g. `https://host/auth/realms/users`) |
| **SST_OIDC_CLIENT_ID** | OAuth client id (non-empty string) |
| **SST_OIDC_CLIENT_SECRET** | OAuth client secret (non-empty string) |

**Optional**

| Variable | Format |
|----------|--------|
| **SST_USERNAME** | Keycloak username; used only if **SST_PASSWORD** is also set |
| **SST_PASSWORD** | Keycloak password; used only if **SST_USERNAME** is also set |

If either **SST_USERNAME** or **SST_PASSWORD** is missing, the CLI prompts interactively. **Storing passwords in the environment is a security risk** on shared machines; prefer interactive entry when possible.

Other variables depend on the host OS and gRPC / TLS stack; there are no additional SST-specific variables required for local-only use.

## FILES

*$HOME/.sst_cli_history* (or the user profile home directory on Windows)  
Stores up to the last 1000 interactive input lines when readline is active.

## DIAGNOSTICS

Exit status is **0** on normal completion of the **sst** process. Non-zero exit codes are propagated from Go’s **os.Exit** paths only where explicitly set; many interactive errors print a message and return to the prompt without exiting the program.

## BUGS

Interactive mode is intended for development and operations support, not as a stable scripting API: command spelling and available subcommands evolve with the Core API.

## SEE ALSO

**CLI_INTRODUCTION.md** — tutorial-style guide, workflows, and merged design notes from the legacy `index`.

Project layout: build the binary with  
`go build -o cli/sst ./cli/main.go`  
from the repository root unless your packaging uses another output path.

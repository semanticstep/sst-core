---
title: "SST-CLI: Introduction to the Command Line Interface"
layout: "simple"
---

The SST Core "Command Line Interface" tool CLI is a low level tool for debugging and testing the SST Core and Repositories.

CLI is working within a terminal window (e.g. a bash terminal). If the tool is not yet build you can do so by invoking the following command that is creating the sst executable.

```
$ go build -o cli/sst ./cli/main.go
```

The CLI tool is started by invoking the ``sst`` executable.

```
$ ./sst 
Entering SST CLI tool in interactive mode. Type 'q' to quit, 'help' to see available commands.
sst > q
Exiting SST CLI tool
```


# CLI

Command line Interface (CLI) to SST

The (planned) command line application sst - we name it SST-CLI - is similar to the "git" command line application as defined in https://git-scm.com/docs. This means that SST functionality, that is equivalent to GIT functionality will has the same or at lease similar commands, options and interface.SST-CLI is implemented directly on top of the SST-Core API. 

Main use cases for SST-CLI is to:
* to play with and test out SST functionality
* being able to easily provide SST test cases without developing software
* being able to identify and report issues that might be in the core SST API, but without the need of programming
* being able to execute specific functionality that an SST application might not (yet) support
* But unlike the GIT-CLI, the SST-LCI is not intended to be used on a daily basis to work with SST data. 

All checked out SST data will be presented to and accepted from the user in the form of readable Turtle (*.ttl) files. These Turtle (*.ttl) files will be named by their corresponding NamedGraph UUID. Question is on how to show the user also the corresponding URL if provided? The Turtle files will be stored in a flat **working directory**  (instead of the **working tree**  in GIT).
Note that SST stores internally all data in the form of SST binary files that a user can not easily inspect.

Like GIT, the SST-CLI is supporting a local SST repository (storage type with history information); but his should be optional. Users should also be able to directly work to/from a remote SST repository without a local one. Configuration data and the optionally local SST Repository will be be stored in a (hidden) sub-directory `.sst`, similar to the `.git` directory for GIT. 

Checkout of SST data is done on Datasets, and so consequently the resulting working directories will be identified by the UUID of the datasets followed by "_" and the branch name. By default the working directories are stored in parallel to the `.sst` directory.

## SST commands

Big question: In git, the current path defines the "git repository" to use. Do we want to have something similar for SST CLI?

### sst-version

```
$ sst --version
```

### sst-statistics

```
$ sst --statistics
```

### ... many more commands to follow

[[CLI comparison]] with GIT-CLI

Naming of parameters:
- r1, r2, r3 ... Repositories
- d1, d2, d3 ... Datasets
- s1, s2, s3 ... Stages
- g1, g2, g3 ... NamedGraphs
- n1, n2, n3 ... IBNodes
- c1, c2, c3 ... collections
the numer 1, 2, 3 will automatically be assigned by sst-cli, next free one
Example:
  sst> OpenRemoteRepository ... # any command
  repository <r1> opened # tell the user about the result
  sst> r1.Close
  repository <r1> is closed
  sst> Statistics
  error: <r1> not opened

---

## Basic Usage

### Getting Help

At any time, type `help` to see all available commands:

```
sst > help
```

This displays a comprehensive list of interactive commands organized by category.

### Status and Information

Check the current state of opened resources:

```
sst > status
```

This shows all currently opened repositories, datasets, stages, named graphs, and IBNodes with their aliases.

## Command Syntax

Commands in the CLI follow a pattern of `<alias>.<command>` for operations on specific resources. Resources are automatically assigned aliases when opened (e.g., `r1`, `r2` for repositories; `d1`, `d2` for datasets; `s1`, `s2` for stages).

- **Repository commands:** `<repo-alias>.<command>`
  - Example: `r1.info`, `r1.datasets`, `r1.close`
- **Dataset commands:** `<dataset-alias>.<command>`
  - Example: `d1.commitdetailsbyhash <hash>`, `d1.history`, `d1.checkoutbranch <branch>`, `d1.setbranchcommit <hash> <branch>`
- **Stage commands:** `<stage-alias>.<command>`
  - Example: `s1.validate`, `s1.commit "message"`, `s1.namedgraphs`
- **NamedGraph commands:** `<namedgraph-alias>.<command>`
  - Example: `g1.info`, `g1.foririnodes`, `g1.ttl`
- **IBNode commands:** `<ibnode-alias>.<command>`
  - Example: `n1.forall`
- **SuperRepository commands:** `<superrepo-alias>.<command>` (aliases are often `sr1`, `sr2`, …)
  - Example: `sr1.list`, `sr1.get my-repo`
- **Standalone commands:** Direct commands without alias
  - Example: `help`, `q`, `status`, `rdfread`

## Repository Commands

### Opening Repositories

Open a local repository:

```
sst > openlocalrepository /path/to/repository
```

This opens a local SST repository and assigns it an alias (e.g., `r1`).

Open a **flat** local repository (a directory of `.sst` files):

```
sst > openlocalflatrepository /path/to/flat-repo
```

Open a remote repository:

```
sst > openremoterepository https://example.com/repo
```

This opens a remote SST repository via URL and assigns it an alias.

Open a **SuperRepository** (local path or remote URL); see [SuperRepository Commands](#superrepository-commands):

```
sst > openlocalsuperrepository /path/to/superrepo
sst > openremotesuperrepository https://example.com/superrepo
```

### Repository Information

Show repository information:

```
sst > <repo-alias>.info
```

Displays detailed information about the repository including:
- EndPoint
- MasterDBSize and DerivedDBSize
- Number of Datasets, Dataset Revisions, Named Graph Revisions, Commits
- RepositoryLogs count
- Whether it's remote
- Whether it supports revision history
- Bleve index information

Example:
```
sst > r1.info
```

### List Datasets

List all datasets in the repository:

```
sst > <repo-alias>.datasets
```

Example:
```
sst > r1.datasets
```

### Get Dataset by IRI

Get a dataset by IRI:

```
sst > <repo-alias>.dataset <iri>
```

Example:
```
sst > r1.dataset urn:uuid:fcfe1293-3045-4717-8065-7dc3659e1faf
```

The dataset is assigned an alias (e.g., `d1`) for subsequent operations.

### Query Repository Index

Run a Bleve text query in the repository index:

```
sst > <repo-alias>.query <bleve-query> [--limit <number>]
```

Example:
```
sst > r1.query *
```

> **Note:** The query command displays all indexed fields for each result. The `--limit` parameter (default: 10) controls how many results are returned.

### List Indexed Fields

List all indexed fields in the repository:

```
sst > <repo-alias>.listfield
```

This shows all available searchable fields in the Bleve index.

### Commit History

List commit history:

```
sst > <repo-alias>.log [-v|--verbose]
```

Use `-v` or `--verbose` for detailed commit information.

Example:
```
sst > r1.log
sst > r1.log -v
```

Show commit details by hash:

```
sst > <repo-alias>.commitinfo <commit-hash>
```

Example:
```
sst > r1.commitinfo abc123def456...
```

Show commit diff (all changes in a commit):

```
sst > <repo-alias>.commitdiff <commit-hash>
```

This shows all added/modified/deleted NamedGraphs in the given commit.

Checkout a repository commit into a stage (all NamedGraphs affected by that commit):

```
sst > <repo-alias>.checkoutcommit <commit-hash> [-a <stage-alias>]
```

Example:
```
sst > r1.checkoutcommit abc123def456... -a s2
```

Unlike `<dataset>.checkoutcommit`, this loads every NamedGraph recorded in the commit without requiring a dataset alias first.

### SuperRepository (of a repository)

Show SuperRepository information for this repository:

```
sst > <repo-alias>.superrepository
```

### Stage Operations

Create an empty stage:

```
sst > <repo-alias>.openstage
```

This creates a new empty stage and assigns it an alias (e.g., `s1`).

### Document Operations

List all documents in the repository:

```
sst > <repo-alias>.documents
```

Show document metadata:

```
sst > <repo-alias>.documentinfo <hash>
```

Upload a document file:

```
sst > <repo-alias>.documentset <file>
```

Example:
```
sst > r1.documentset /path/to/document.pdf
```

Download a document by hash:

```
sst > <repo-alias>.documentget <hash> <output-file>
```

Example:
```
sst > r1.documentget abc123... output.pdf
```

Delete a document:

```
sst > <repo-alias>.documentdelete <hash>
```

### Internal Operations (Advanced)

Dump internal BoltDB data (use with caution):

```
sst > <repo-alias>.dump <bucket-key>[/<sub-key>]
```

Bucket keys:

| Key | Contents |
|-----|----------|
| `ngr` | NamedGraphRevisions |
| `dsr` | DatasetRevisions |
| `c` | Commits (metadata: author, message, timestamp) |
| `ds` | Datasets (IRI, UUID, etc.) |
| `dl` | DatasetLog (chronological commit log entries) |

Examples:

```
sst > r1.dump ds
sst > r1.dump "c/<commit-hash>"
sst > r1.dump c
```

Clone a repository to a local directory:

```
sst > <repo-alias>.clone <target-directory>
```

Sync data from another repository:

```
sst > <repo-alias>.syncfrom <source-repo-alias> [branch] [dataset1] [dataset2] ...
```

Extract raw SST file:

```
sst > <repo-alias>.extractsstfile <hash>
```

Extract the raw SST file of a NamedGraphRevision by its hash.

### Close Repository

Close a repository:

```
sst > <repo-alias>.close
```

This closes the repository and removes it from the active session.

## Dataset Commands

### List Commits

List commit history starting from each **leaf** commit (same leaf set as `leafcommits`, with history traversal):

```
sst > <dataset-alias>.listcommits [--details]
```

Without flags, hashes along each chain are shown. Pass `--details` for full commit information at each step.

List all branches and their commit hashes:

```
sst > <dataset-alias>.branches
```

Example output:
```
Branches:
  master: abc123def456...
  branch1: def456ghi789...
```

List all leaf commits (commits not identified by a branch):

```
sst > <dataset-alias>.leafcommits
```

### Get Commit Details

Get commit details by hash:

```
sst > <dataset-alias>.commitdetailsbyhash <hash>
```

Example:
```
sst > d1.commitdetailsbyhash abc123def456...
```

Get commit details by branch:

```
sst > <dataset-alias>.commitdetailsbybranch <branch-name>
```

Example:
```
sst > d1.commitdetailsbybranch master
```

Both commands display:
- Commit Hash
- Author
- Date
- Message
- Dataset Revisions
- NamedGraph Revisions
- Parent Commits

### Checkout Operations

Checkout a repository commit (all NamedGraphs in the commit):

```
sst > <repo-alias>.checkoutcommit <hash> [-a <stage-alias>]
```

This creates a new stage from the specified repository commit. Use this when you have a commit hash from `repo.log` or `repo.commitinfo` and need a stage containing all datasets/NamedGraphs affected by that commit.

Example:
```
sst > r1.checkoutcommit abc123def456... -a s2
```

Checkout a specific commit (single dataset):

```
sst > <dataset-alias>.checkoutcommit <hash>
```

This creates a new stage from the specified commit. If you do not pass `-a`, a stage alias is auto-generated (`s1`, `s2`, …).

Example:
```
sst > d1.checkoutcommit abc123def456... -a s2
```

Checkout a specific dataset revision:

```
sst > <dataset-alias>.checkoutrevision <hash>
```

This creates a new stage from the specified dataset revision hash. Use this when a dataset revision exists that is not associated with a branch name. If you do not pass `-a`, a stage alias is auto-generated (`s1`, `s2`, …).

Example:
```
sst > d1.checkoutrevision abc123def456... -a s2
```

Checkout a branch:

```
sst > <dataset-alias>.checkoutbranch <branch-name>
```

This creates a new stage from the specified branch. Optional stage alias: `-a <alias>`.

Example:
```
sst > d1.checkoutbranch master
sst > d1.checkoutbranch master -a s2
```

### Branch Management

Set a branch to point to a commit:

```
sst > <dataset-alias>.setbranchcommit <commit-hash> <branch-name>
```

Example:
```
sst > d1.setbranchcommit abc123def456... feature
```

Set a branch to point to a dataset revision (when the revision is not tied to a commit lookup):

```
sst > <dataset-alias>.setbranchrevision <dataset-revision-hash> <branch-name>
```

Example:
```
sst > d1.setbranchrevision def456ghi789... feature
```

Remove a branch:

```
sst > <dataset-alias>.removebranch <branch-name>
```

Example:
```
sst > d1.removebranch feature
```

### History and Diff

Show commit history graph:

```
sst > <dataset-alias>.history
```

This displays an graph visualization of the commit history.

Compare two NamedGraphRevision hashes:

```
sst > <dataset-alias>.diff <NGR-hash1> <NGR-hash2>
```

This shows the differences between two NamedGraphRevision hashes, including added, modified, and deleted triples.

Example:
```
sst > d1.diff abc123... def456...
```

## Stage Commands

### Stage Information

Show stage information:

```
sst > <stage-alias>.info
```

Displays:
- Number of local graphs
- Number of referenced graphs
- Total number of triples
- IBNodes count for each graph

### Named Graph Operations

List named graphs in the stage:

```
sst > <stage-alias>.namedgraphs
```

List referenced named graphs:

```
sst > <stage-alias>.referencednamedgraphs
```

Get named graph by IRI:

```
sst > <stage-alias>.namedgraph <iri>
```

Examples:
```
sst > s1.namedgraph urn:uuid:fcfe1293-3045-4717-8065-7dc3659e1faf#
```

Merge content from another open stage into this stage (target stage receives graphs from the source):

```
sst > <target-stage-alias>.moveandmerge <source-stage-alias>
```

### AlignHistory (replace data, keep checkout history)

Copy the repository link and per–named-graph checkout metadata (commit, NG revision, DS revision) from an existing checkout stage onto another stage—typically a stage produced by `rdfread` after you edited TriG/Turtle outside SST. Unlike `moveandmerge`, this does **not** merge graph contents or remove the source stage from the session; it only aligns history so a subsequent `commit` from the target stage can record changes against the same repository lineage.

```
sst > <to-stage-alias>.alignhistory <from-stage-alias>
```

- **to-stage**: Stage with the new RDF content (often from `rdfread`; it may not be linked to a repository before this command).
- **from-stage**: Original checkout stage; must already be linked to a repository.

Typical flow: checkout → `rdfwrite` → edit file externally → `rdfread` (new stage) → `<new>.alignhistory <original>` → `commit` on the new stage.

### Validation

Validate stage content:

```
sst > <stage-alias>.validate
```

This validates the stage content for RDF syntax and domain-range constraints. The output is a detailed report organized by NamedGraph, with each finding showing the message and the related triple (Subject, Predicate, Object).

### Commit Changes

Commit current changes in the stage:

```
sst > <stage-alias>.commit <message> [branch]
```

Example:
```
sst > s1.commit "Initial commit"
sst > s1.commit "Updated configuration" feature-branch
```

### RDF output (stage)

Write the stage RDF to a file (TriG):

```
sst > <stage-alias>.rdfwrite <file>
```

Print the stage RDF to the console (TriG):

```
sst > <stage-alias>.trig
```

### SST file output (stage)

Write modified NamedGraphs from a stage into a directory as SST binary files

```
sst > <stage-alias>.writesstfilesdirectory <directory>
```

Example:

```
sst > s1.writesstfilesdirectory ./out-sst
```

### Reading RDF Files

Read an RDF file in Turtle or TriG format into a new stage:

```
sst > rdfread <file>
```

Example:
```
sst > rdfread /path/to/data.ttl
```

This creates a new stage from the RDF file.

### Reading SST Files

Read an SST binary file into a new stage:

```
sst > sstread <file>
```

Example:
```
sst > sstread /path/to/data.sst
```

This creates a new stage from the SST file. Use `<stage>.namedgraph <iri>` to obtain a NamedGraph alias, then `<namedgraph>.sstwrite <file>` to write it back to SST binary.

### Import/Export (Converters)

Import AP242 XML into a new stage:

```
sst > importap242xml <file>
```

Export a NamedGraph to AP242 XML:

```
sst > <namedgraph-alias>.exportap242xml <output-file.xml>
```

Import a STEP P21 file into a new stage:

```
sst > importp21 <file>
```

## SuperRepository Commands

SuperRepositories are opened with `openlocalsuperrepository` / `openremotesuperrepository` and get an alias (e.g., `sr1`).

Commands:

```
sst > <superrepo-alias>.info
sst > <superrepo-alias>.close
sst > <superrepo-alias>.list
sst > <superrepo-alias>.get <repo-name>
sst > <superrepo-alias>.create <repo-name>
sst > <superrepo-alias>.delete <repo-name>
```

## NamedGraph Commands

### NamedGraph Information

Show named graph information:

```
sst > <namedgraph-alias>.info
```

Displays detailed information including:
- IRI and ID
- Whether it's referenced or empty
- Whether it's modified
- Number of IRI Nodes, Blank Nodes, Term Collections
- Number of Direct/All Imported Graphs
- Number of Subject/Predicate/Object/TermCollection Triples
- Commit Hash, NamedGraph Revision Hash, Dataset Revision Hash

### List Nodes

List all IRI nodes:

```
sst > <namedgraph-alias>.foririnodes
```

List all IBNodes (IRI nodes and blank nodes):

```
sst > <namedgraph-alias>.forallibnodes
```

List all blank nodes:

```
sst > <namedgraph-alias>.forblanknodes
```

### Get Node by Fragment

Get IRINode by fragment:

```
sst > <namedgraph-alias>.getirinodebyfragment <fragment-id>
```

Get blank node by fragment:

```
sst > <namedgraph-alias>.getblanknodebyfragment <fragment-id>
```

### RDF Output

Write RDF to a file (Turtle format):

```
sst > <namedgraph-alias>.rdfwrite <file>
```

Example:
```
sst > ng1.rdfwrite output.ttl
```

Write SST binary to a file:

```
sst > <namedgraph-alias>.sstwrite <file>
```

Example:
```
sst > ng1.sstwrite output.sst
```

Print RDF to console (Turtle format):

```
sst > <namedgraph-alias>.ttl
```

This outputs the entire NamedGraph in Turtle format to the console.

## IBNode Commands

### List Triples

List all triples in an IBNode:

```
sst > <ibnode-alias>.forall
```

This displays all triples where the IBNode appears as subject, predicate, or object.

## Workflow Examples

### Example 1: Basic Repository and Dataset Workflow

```
# Open a local repository
sst > openlocalrepository /path/to/repo

# Check status
sst > status

# List datasets
sst > r1.datasets

# Get a dataset
sst > r1.dataset urn:uuid:fcfe1293-3045-4717-8065-7dc3659e1faf

# View branches
sst > d1.branches

# Checkout a branch
sst > d1.checkoutbranch master

# View stage info
sst > s1.info

# List named graphs
sst > s1.namedgraphs

# Get a named graph
sst > s1.namedgraph urn:uuid:fcfe1293-3045-4717-8065-7dc3659e1faf#

# View named graph info
sst > g1.info
```

### Example 2: Reading and Validating RDF

```
# Read an RDF file
sst > rdfread data.ttl

# Validate the stage
sst > s1.validate

# If valid, commit it
sst > s1.commit "Initial data import"
```

### Example 3: Working with Commits

```
# View commit history
sst > r1.log -v

# Get commit details
sst > d1.commitdetailsbyhash abc123...

# View history graph
sst > d1.history

# Compare revisions
sst > d1.diff hash1 hash2
```

### Example 4: Querying and Searching

```
# List indexed fields
sst > r1.listfield

# Run a text query
sst > r1.query "field:value"
sst > r1.query connector --limit 20
```

## See also

- **[sst.1.md](sst.1.md)** — Git-style command reference (NAME, SYNOPSIS, OPTIONS, full interactive command tables).

## Exiting

To exit the interactive mode, simply type:

```
sst > q
```

This will close all opened resources and exit the CLI.

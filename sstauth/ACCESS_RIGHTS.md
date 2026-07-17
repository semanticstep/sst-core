# SST Access Control

This document describes how SST authorizes remote Repository access. It covers
the access modes, role mapping, the two supported authorization models, and the
gRPC method-level permission table.

The authorization logic lives in the `sstauth` package together with the gRPC
interceptors used by `sst.NewServer` and `sst.NewSuperServer`.

In this document, the Repositories managed by a SuperRepository are referred to
as **Repository Entries**.

## Access modes

`AccessMode` is an ordered integer. Higher values imply all permissions of lower
values:

| Mode | String | Description |
|------|--------|-------------|
| `AccessMode_None` | `none` | No access. |
| `AccessMode_ReadOnly` | `read-only` | Read Datasets, commits, refs, Bleve search, etc. |
| `AccessMode_ReadWrite` | `read-write` | Read + write Datasets, commits, branch updates, sync. |
| `AccessMode_Admin` | `admin` | Read/write + administrative operations such as branch management. |
| `AccessMode_SuperAdmin` | `super-admin` | SuperRepository operations such as creating or deleting Repositories. |

`AccessModeFromRoles` derives the effective mode from a list of role names by
looking for the substrings `super-admin`, `admin`, `write`/`rw`, or
`read-only`/`readonly`/`read_only`. If no recognized role is found, it returns
`AccessMode_None`.

## Role source: Keycloak `resource_access`

SST reads roles from the OIDC ID token claim:

```json
{
  "email": "user@example.com",
  "resource_access": {
    "<client-id>": {
      "roles": ["ReadOnly", "ReadWrite"]
    }
  }
}
```

`AccessModeForClientID(claims, clientID)` returns the highest access mode found
under `resource_access.<clientID>.roles`. Tokens must also contain the expected
client ID in their `aud` claim; the OIDC verifier is configured with
`SkipClientIDCheck: true` so that the interceptors can validate the audience
themselves.

## Authentication flow

1. **Obtain a token.** The client authenticates to the configured Keycloak realm
   and requests an access token for the SST audience. The token must contain the
   expected `aud` value and the required client roles under
   `resource_access.<clientID>.roles`.

2. **Attach the token to the context.** The client wraps the token in an
   `sstauth.Provider` and attaches it to the gRPC context:

   ```go
   ctx := sstauth.ContextWithAuthProvider(ctx, &myProvider{token: token})
   ```

3. **Send the token with every RPC.** The remote SST client
   (`sst.OpenRemoteRepository` / `sst.OpenRemoteSuperRepository`) automatically
   includes the token as an OAuth2 bearer token in the gRPC metadata of each
   call.

4. **Verify the token.** On the server, `UnaryRBACInterceptor` or
   `StreamRBACInterceptor` extracts the bearer token from the incoming metadata,
   verifies its signature and expiry against the configured OIDC issuer, and
   parses the claims.

5. **Check the audience.** The interceptor compares the token's `aud` claim
   against the expected client ID:

   - SuperRepository management methods (`RepoManagerService`) expect the super
     `ClientID`.
   - Other methods with `PerRepoAuth = true` expect
     `<superClientID>#<repoName>`.
   - Single Repository servers (`PerRepoAuth = false`) expect the server's
     `ClientID`.

6. **Check the roles.** For the matched client ID, the interceptor looks up
   `resource_access.<clientID>.roles`, derives an `AccessMode`, and compares it
   with the minimum required mode for the gRPC method.

7. **StreamRBACInterceptor and the streaming follow-up check.** Streaming RPCs
   go through `StreamRBACInterceptor`, which authenticates the token and checks
   that its audience belongs to this server (super client or any per-repository
   client). Because the target repository name arrives inside the first stream
   message, the interceptor cannot verify the exact per-repository audience and
   roles before the stream starts. The handler therefore calls
   `sstauth.CheckRepoAccess` after reading the first message to enforce the
   per-repository audience and required roles.

8. **Execute or reject.** If all checks pass, the handler runs; otherwise the
   server returns `Unauthenticated` (token missing or invalid) or
   `PermissionDenied` (audience or roles insufficient).

### Unary RPCs vs. streaming RPCs

gRPC supports two basic call shapes, so SST uses two interceptors:

| Aspect | Unary RPC | Streaming RPC |
|--------|-----------|---------------|
| Messages | One request, one response | Multiple messages in one or both directions |
| Interceptor | `UnaryRBACInterceptor` | `StreamRBACInterceptor` |
| When the target repo is known | Before the handler runs (`req.GetRepoName()`) | Only after the handler reads the first stream message |
| Audience and role check | Fully inside the interceptor | Preliminary check in the interceptor; exact per-repo check in the handler via `sstauth.CheckRepoAccess` |

**Unary RPCs** are simple request/response calls such as `ListDatasets` or
`SetBranch`. Because the whole request is available to the interceptor, it can
resolve `RepoName`, build the exact expected audience (for example
`<superClientID>#<repoName>`), and reject the call before the handler is ever
invoked.

**Streaming RPCs** keep the connection open for multiple messages. SST uses them
for operations such as `bleveproto.IndexService/Search` and
`bboltproto.DatasetService/SyncFrom`. When `StreamRBACInterceptor` runs, no
message has been received yet, so the interceptor cannot know which repository
the stream targets. It therefore only verifies that the token is valid and that
its `aud` belongs to this server (super client or any per-repository client).
The handler then reads the first message, extracts the repo name, and calls
`sstauth.CheckRepoAccess` to perform the same audience and role validation that
`UnaryRBACInterceptor` does for unary calls.

## Authorization models

### `PerRepoAuth = false` — single Repository server

`sst.NewServer` creates a single `RepositoryServer` with `PerRepoAuth` set to
`false`. This is the original SST authorization model and is kept for backward
compatibility.

- The server expects the token audience to be exactly the configured
  `ClientID`.
- All method roles are resolved from `resource_access.<ClientID>.roles`.
- There is no per-Repository isolation; every caller with a valid token for the
  server's client ID is authorized according to their roles.

Example token audience:

```json
{ "aud": ["grpc://repo.example.com"] }
```

### `PerRepoAuth = true` — SuperRepository with per-Repository clients

`sst.NewSuperServer` creates a `SuperRepositoryServer` with `PerRepoAuth` set to
`true`. In this model the super client ID owns Repository management, and each
sub-Repository has its own audience.

- `RepoManagerService` methods always use the super client ID audience.
- All other methods use a per-Repository client ID:
  `<superClientID>#<repoName>`.
- Unary RPCs resolve the target repo name from the request's `RepoName` field
  and validate both audience and roles before invoking the handler.
- Streaming RPCs go through `StreamRBACInterceptor`. Because the repo name
  arrives in the first message, the interceptor can only authenticate the token
  and verify that its audience belongs to this server. The handler then calls
  `sstauth.CheckRepoAccess` after reading the first message to enforce the exact
  per-Repository audience and roles.

Example token for Repository `repoA`:

```json
{ "aud": ["grpc://super.example.com#repoA"] }
```

## Keycloak configuration for SST servers

The server's `Issuer` must point to the Keycloak realm
(`https://keycloak.example.com/realms/<realm>`) and its `ClientID` must match a
Keycloak client ID exactly. The client ID is also the default value of the
`aud` claim in issued tokens.

### Single Repository server (`PerRepoAuth = false`)

1. Create one Keycloak client whose **Client ID** equals the server's `ClientID`
   (for example `grpc://repo.example.com`).
2. Under that client, create client roles: `ReadOnly`, `ReadWrite`, `Admin`,
   `SuperAdmin`.
3. Assign those client roles to users or groups through **Role mappings**.
4. Ensure the token contains a `Client roles` mapper so that
   `resource_access.<ClientID>.roles` is populated.

The resulting token audience and roles look like:

```json
{
  "aud": ["grpc://repo.example.com"],
  "resource_access": {
    "grpc://repo.example.com": {
      "roles": ["ReadWrite"]
    }
  }
}
```

### SuperRepository server (`PerRepoAuth = true`)

1. Create the **super client** with **Client ID** equal to the server's
   `ClientID` (for example `grpc://super.example.com`). Add the
   `SuperAdmin`, `Admin`, `ReadWrite` and `ReadOnly` client roles to it.
2. For every Repository managed by the SuperRepository, create a Keycloak client
   whose **Client ID** is `<superClientID>#<repoName>` (for example
   `grpc://super.example.com#repoA`). Add `ReadOnly`, `ReadWrite` and `Admin`
   roles to each per-Repository client as needed.
3. Assign roles to users or groups under the appropriate client.
4. Ensure each client has a `Client roles` mapper so the token includes
   `resource_access.<clientID>.roles`.

A token used to access Repository `repoA` contains:

```json
{
  "aud": ["grpc://super.example.com#repoA"],
  "resource_access": {
    "grpc://super.example.com#repoA": {
      "roles": ["ReadWrite"]
    }
  }
}
```

If a client needs to access multiple Repositories, add an **Audience**
protocol mapper to that client for each per-Repository audience
(`<superClientID>#<repoName>`) and enable **Add to access token**.

### Service accounts

For machine-to-machine access, enable **Client authentication** and **Service
accounts roles** on the relevant Keycloak client. Assign the service account the
required client roles, then use the **Client Credentials** grant to obtain a
token.

## gRPC method permissions

The `repositoryMethodRoles` table maps gRPC full method names to the minimum
required `AccessMode`. A caller whose effective mode is greater than or equal to
the required mode is allowed.

### `RepoManagerService` (SuperRepository only)

| Method | Required mode |
|--------|---------------|
| `ListRepos` | `SuperAdmin` |
| `CreateRepo` | `SuperAdmin` |
| `DeleteRepo` | `SuperAdmin` |
| `GetRepoQuota` | `SuperAdmin` |
| `SetRepoQuota` | `SuperAdmin` |
| `GetSuperQuota` | `SuperAdmin` |
| `SetSuperQuota` | `SuperAdmin` |
| `GetMaxRepoCount` | `SuperAdmin` |
| `SetMaxRepoCount` | `SuperAdmin` |

### `DatasetService` (read)

| Method | Required mode |
|--------|---------------|
| `GetBranches` | `ReadOnly` |
| `GetLeafCommits` | `ReadOnly` |
| `GetBleveInfo` | `ReadOnly` |
| `ListDatasets` | `ReadOnly` |
| `GetDataset` | `ReadOnly` |
| `FetchDatasets` | `ReadOnly` |
| `GetRepositoryInfo` | `ReadOnly` |
| `GetRepositoryLog` | `ReadOnly` |
| `Document` | `ReadOnly` |
| `Documents` | `ReadOnly` |
| `DownloadNamedGraphRevision` | `ReadOnly` |
| `FindCommonParentRevision` | `ReadOnly` |
| `GetCommitForDatasetRevision` | `ReadOnly` |
| `SyncTo` | `ReadOnly` |

### `DatasetService` (write)

| Method | Required mode |
|--------|---------------|
| `CreateDataset` | `ReadWrite` |
| `SetBranch` | `ReadWrite` |
| `PushDatasets` | `ReadWrite` |
| `DocumentSet` | `ReadWrite` |
| `SyncFrom` | `ReadWrite` |
| `RemoveBranch` | `ReadWrite` |
| `DocumentDelete` | `ReadWrite` |

### `RefService`

| Method | Required mode |
|--------|---------------|
| `ListRefs` | `ReadOnly` |
| `GetRef` | `ReadOnly` |

### `CommitService`

| Method | Required mode |
|--------|---------------|
| `ListCommits` | `ReadOnly` |
| `GetCommit` | `ReadOnly` |
| `CompareCommits` | `ReadOnly` |
| `GetCommitDetailsBatch` | `ReadOnly` |
| `CreateCommit` | `ReadWrite` |

### `ssquery.IndexService`

| Method | Required mode |
|--------|---------------|
| `Search` | `ReadOnly` |

## Server configuration

```go
// Single Repository server — PerRepoAuth is false.
server, err := sst.NewServer(&sst.RepositoryServerConfig{
    RepoDir:  "/data/repo",
    Issuer:   "https://keycloak.example.com/realms/sst",
    ClientID: "grpc://repo.example.com",
    // ServerCert, Verbose, DeriveInfo, ...
})

// SuperRepository server — PerRepoAuth is true.
server, err := sst.NewSuperServer(&sst.RepositoryServerConfig{
    RepoDir:  "/data/super",
    Issuer:   "https://keycloak.example.com/realms/sst",
    ClientID: "grpc://super.example.com",
    // ServerCert, Verbose, DeriveInfo, ...
})
```

Both servers attach the `UnaryRBACInterceptor` and `StreamRBACInterceptor` when
an issuer and client ID are configured.

## Client usage

Attach a bearer token to the request context with `sstauth.ContextWithAuthProvider`:

```go
token := "<OIDC access token>"
ctx := sstauth.ContextWithAuthProvider(context.Background(), &myProvider{token: token})
repo, err := sst.OpenRemoteRepository(ctx, "localhost:50051", transportCreds)
```

The token must contain the correct audience (`aud`) and the required roles under
`resource_access.<clientID>.roles` for the authorization model used by the
server.

## Helper functions

- `AccessModeForClientID(claims, clientID)` — returns the access mode for a
  specific Keycloak client.
- `CheckRepoAccess(ctx, repoURL, fullMethod, methodRoles, superClientID, useSuperClient)` —
  validates audience and roles inside streaming handlers.
- `RepoNameFromRequest(req)` — extracts `RepoName` from a gRPC request, defaulting
  to `"default"`.
- `HasAccess(userMode, requiredMode)` — compares two ordered access modes.

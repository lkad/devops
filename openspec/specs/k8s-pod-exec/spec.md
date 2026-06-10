# k8s-pod-exec

## Purpose

Define the on-demand "shell into a running pod" capability
for operators. Mirrors `kubectl exec -it pod -- <cmd>`:
stream stdout + stderr in real time, propagate the remote
exit code, and tear the session down on cancel or timeout.
This is the foundation for in-platform live debugging
("inspect the in-memory state of the auth service that the
metrics say is wedged") without leaving the operator's
web UI.

This spec is intentionally READ-ONLY-shaped (one command
per session, no persistent PTY upgrade path yet) — the
companion `kubectl debug`-style interactive shell is a
deliberate non-goal for v0.3. Operators who need that today
have `kubectl`.

## Requirements

### Requirement: ExecIntoPod

The system SHALL execute a single command in a running pod
and stream the output to the caller.

#### Scenario: Successful command

- **WHEN** operator calls `POST /api/v1/k8s/clusters/{clusterID}/namespaces/{ns}/pods/{pod}/exec`
  with body `{ "command": ["ls", "-la", "/var/log"], "container": "app" }`
- **AND** the pod is in `Running` phase with the named container present
- **THEN** system opens an SPDY session to the cluster's apiserver
- **AND** streams stdout lines as `{ "stream": "stdout", "line": "..." }` frames
- **AND** streams stderr lines as `{ "stream": "stderr", "line": "..." }` frames
- **AND** on command exit, returns the remote exit code
- **AND** closes the streaming connection

#### Scenario: Empty command rejected

- **WHEN** operator calls exec with `command: []` (empty array) or missing
- **THEN** system returns 400 with code `INVALID_EXEC_REQUEST`
- **AND** does NOT open an apiserver connection

#### Scenario: Pod not found

- **WHEN** operator targets a pod that does not exist in the cluster
- **THEN** system returns 404 with code `POD_NOT_FOUND`
- **AND** does NOT charge a retry budget against the cluster's kubeconfig

#### Scenario: Container not found in pod

- **WHEN** operator specifies a `container` that does not exist in the pod's spec
- **THEN** system returns 400 with code `CONTAINER_NOT_FOUND`
- **AND** the error message lists the available containers in the pod

#### Scenario: Command exits non-zero

- **WHEN** the remote command exits with code != 0
- **THEN** system returns HTTP 200 with body `{ "exit_code": N, "stdout_lines": [...], "stderr_lines": [...] }`
- **AND** does NOT treat the non-zero exit as a request failure (this is normal for `grep`, `test -f`, etc.)

#### Scenario: Cancellation

- **WHEN** the client cancels the request (ctx.Done()) while the command is still running
- **THEN** system closes the SPDY session
- **AND** returns 499 (client closed) or surfaces the cancel as a typed error

#### Scenario: Timeout

- **WHEN** the command has been running longer than `ExecMaxDuration` (default 30s)
- **THEN** system closes the SPDY session
- **AND** returns a `TIMEOUT` error
- **AND** does NOT retry — the remote process may be in any state, retrying could duplicate side effects

### Requirement: No PTY

The system SHALL NOT allocate a TTY for v0.3 exec calls.

- The `command` array is a pure argv (no shell expansion).
  An operator who needs `ls /var/log | grep error` writes
  `["sh", "-c", "ls /var/log | grep error"]` themselves.
  This keeps the spec free of shell-injection footguns
  and matches the kubectl-exec-without-tty default.
- A future spec may add `tty: true` if user feedback
  demands it; until then, the client-side terminal must
  be a raw line-streamer, not a real PTY emulator.

### Requirement: Auth and RBAC

The system SHALL enforce auth + RBAC on every exec call.

- Caller must present a valid JWT (existing dev_bypass
  flow applies to dev).
- The system looks up the caller's role and rejects with
  403 if the role lacks `k8s:pods:exec`.
- The system's k8s.Service decides per-cluster whether
  the stored kubeconfig has exec permission. If the
  cluster's service account lacks the `pods/exec` sub-
  resource, system returns 502 with code
  `CLUSTER_LACKS_EXEC_PERMISSION`. The catalog health
  rollup is unchanged — this is an operator capability,
  not a service-level signal.

### Requirement: Wire contract

- Request: `POST /api/v1/k8s/clusters/{clusterID}/namespaces/{ns}/pods/{pod}/exec`
  - Body: `{ "command": [string...], "container": string, "timeout_seconds"?: number }`
  - Header: `Authorization: Bearer <jwt>` (existing pattern)
- Response (200):
  - Body: `{ "exit_code": int, "stdout_lines": [string...], "stderr_lines": [string...], "duration_ms": number }`
- Response (streaming variant, future work): not in v0.3.
  Operators who need real-time tail use
  `GetLogsBySelector` (Item 3). Exec is for one-shot
  debugging.

## Wire shape

```typescript
// Request
type ExecRequest = {
  command: string[];          // argv, no shell
  container: string;          // required for multi-container pods
  timeout_seconds?: number;   // default 30, max 600
};

// Response (200)
type ExecResponse = {
  exit_code: number;
  stdout_lines: string[];
  stderr_lines: string[];
  duration_ms: number;
};

// Error envelope (existing pattern, codes only)
type ExecErrorCode =
  | "INVALID_EXEC_REQUEST"   // 400, empty command
  | "POD_NOT_FOUND"           // 404
  | "CONTAINER_NOT_FOUND"     // 400
  | "TIMEOUT"                 // 504
  | "CLUSTER_LACKS_EXEC_PERMISSION"  // 502
  | "UNAUTHORIZED"            // 401
  | "FORBIDDEN"               // 403
  | "APISERVER_UNREACHABLE";  // 502
```

## Non-goals (v0.3)

- **Interactive PTY.** The wire shape is request/response
  only. A future spec may add WebSocket-based exec with
  TTY allocation.
- **File transfer.** `kubectl cp`-style copy-in / copy-out
  is out of scope. Operators use `kubectl cp` for that.
- **Port-forward.** `kubectl port-forward` is a different
  subresource and a different spec.
- **Multi-pod fan-out.** The request targets one pod. The
  multi-pod case is a higher-level orchestration that
  this primitive does not need to solve.

## Implementation notes (for the engineering reader)

- The K8s `pods/exec` subresource is a SPDY/websockets
  upgrade, not a regular HTTP request. Use
  `remotecommand.NewSPDYExecutor(config, "POST", url)`
  with `corev1.PodExecOptions` in the request body.
- For v0.3 the in-memory kubeconfig (from
  `k8s.Service.DecryptKubeconfig`) is converted to a
  `*rest.Config` and threaded through
  `kubernetes.Interface.Discovery().RESTClient()`. The
  upgrade path is the existing `KubeClient`'s typed
  iface — ExecInPod is the next method to add there.
- The cancellation contract uses `context.Context`. When
  the request context is cancelled, `Stream(ctx)` on the
  executor returns the cancel error; we close the SPDY
  session and surface `TIMEOUT` to the caller.
- A successful non-zero exit code is NOT a request error.
  Many debugging commands (`grep`, `test`) exit non-zero
  as their normal mode. The wire shape's `exit_code`
  carries the truth; HTTP status 200 means the request
  itself succeeded.

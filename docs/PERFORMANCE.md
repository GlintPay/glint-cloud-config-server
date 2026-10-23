# Performance optimisation notes

Analysis of safe, non-breaking performance improvements for gccs. Based on a read of the request path and an example config repo. Nothing here has been benchmarked or implemented yet; impact notes are estimates.

## Constraints

- **Strict freshness.** After a config change, every request must see it. No TTLs, no background-only refresh, no bounded staleness. Anything that violates this is out of scope.
- **No behaviour changes** for existing clients (response bodies, ordering, headers).

## Request path today

1. `LoadConfigurations` sorts the shared backends slice, then calls each backend's `GetCurrentState`.
2. Git backend: `PlainOpen`, then `Worktree.Pull` against the remote on every request (when `refreshRate` is 0, the documented default), then `commit.Files()`.
3. For each matching file: read the blob, parse YAML (`sigs.k8s.io/yaml`, YAML to JSON to map), optionally flatten.
4. If `resolve=true`: sort sources, merge, resolve `${}` placeholders and Go templates (parsed per string per request).
5. `encoding/json` marshal, gzip (`middleware.Compress(5)`).

## Recommendations

### Priority 1: measure first

`?norefresh` skips the pull, so it gives a built-in A/B:

```bash
for i in $(seq 20); do curl -s -o /dev/null -w '%{time_total}\n' "localhost:PORT/myapp/production"; done
for i in $(seq 20); do curl -s -o /dev/null -w '%{time_total}\n' "localhost:PORT/myapp/production?norefresh"; done
```

If the first set is much slower, the pull (probably the SSH handshake) dominates. If they are close, the processing costs below matter more. Also collect real traffic numbers (RPS, p50/p99).

### Priority 2: SHA-keyed caches (freshness-safe)

Every request still pulls and obtains the current HEAD SHA, so caches keyed on it are correct by construction.

- **Response cache.** Key: commit SHA(s) + path + profiles + query flags (+ injection body for PATCH). A hit skips load, parse, flatten, resolve and marshal. Skip caching responses that resolved `k8s/...` placeholders, because those can change without a commit (not currently in use).
- **Blob-hash YAML cache.** Key: git blob hash; content-addressed so it is never stale. Downstream code mutates the maps (`overrideValue`, `resolvePlaceholders`), so deep-copy on a hit. Makes misses cheaper, e.g. immediately after a push when all response keys change.
- **ETag / If-None-Match** using the same key: purely additive for clients, and lets repeat callers get 304s.

### Priority 3: make the pull cheaper without making it stale

- **Serialize and batch pulls (leader/follower).** Currently concurrent requests each run an unsynchronised `Pull` on the same worktree. Run one pull at a time; requests arriving during a pull wait for it and then share the next single pull. Each request is served by a pull that started after it arrived, so freshness is exact. Under load N pulls become at most 2 per window, and the overlapping-pull hazard disappears. No benefit for a single sequential caller.
- **Cheaper connection setup.** Over SSH each pull pays TCP, key exchange and auth; go-git cannot reuse SSH connections. HTTPS with a token reuses keep-alive connections, so the check can approach one round trip. This is a deploy-key to token config change with no behaviour change. Decide after the Priority 1 numbers.
- **Keep the `*Repository` open** rather than `PlainOpen` on every request, so go-git's object cache survives. Reopen only if needed.

### Priority 4: trivial fixes

- **Sort backends once at startup.** `LoadConfigurations` calls `sort.SliceStable` on the shared slice on every request: wasted work and a data race under concurrency.
- **Downgrade the per-source `Info` log** in `newDiscoveryHandler` to `Debug`. It fires for every property source on every request. Changes log volume only.

### Low value for this workload (skip unless profiling says otherwise)

- Replacing `commit.Files()` with root-tree lookups: the tree is one flat directory of about 295 entries, so the saving is negligible. (Safe, if ever wanted: only root-level files can match, because names are compared with the path included.)
- JSON library swap (`goccy/go-json`, `sonic`): would need byte-identical output checks.
- Compression level or threshold tuning.
- Go template parse caching: only helps `admin2` requests and is awkward because of the per-call `prop` closure.
- Shallow clone (`disableLabels: true`): disables the `/{label}` route and the repo is already small.

## Rejected: background refresh / `refreshRate > 0`

Rejected because it introduces staleness, which conflicts with the strict-freshness requirement. Separately, it looks like a latent bug: in `backend/git/backend.go`, `connect` skips the pull entirely when `RefreshRateMillis > 0` (added in commit `85a208d`), and the scheduled job calls `connect`. So the scheduled "pull" appears to do nothing. This should be confirmed with a test. It does not affect deployments running `refreshRate: 0`, but anyone setting a non-zero value may be serving a stale repo.

## Other findings

- `GetCurrentState` releases `commitsLock` before the files are iterated, and the pull runs outside the lock, so concurrent requests can overlap on the worktree. Serialising pulls (Priority 3) largely resolves this.

## Suggested order of work

1. Measure (Priority 1).
2. Sort-once, log level (Priority 4).
3. Serialised/batched pull and open-repo reuse (Priority 3).
4. SHA-keyed response cache, ETag, then blob YAML cache (Priority 2).
5. Investigate the `refreshRate` bug separately with a failing test first.
6. Revisit SSH vs HTTPS only if the Priority 1 numbers point at connection setup.

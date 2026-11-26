# Spec: Clean up a deleted peer's apps, exposure and metadata (#102)

## Context
`DELETE /api/peers/v1/:name` currently removes only the peer's WireGuard entry and its peer record. The peer's apps stay in `GET /api/apps/v1`, as do their nginx proxy configs, Kubernetes Services and NetworkPolicies, and the peer's stored metadata. All of these point at a peer that no longer exists. After this change, deleting a peer also removes everything the server built for it. Source: the approved intent for issue #102.

## Requirements
- R1: After a successful peer deletion, `GET /api/apps/v1` returns no entry whose `peer` is the deleted peer.
- R2: After a successful peer deletion, every exposure the server created for the peer's apps is withdrawn: the nginx config file of each app (`remote-<peer>-<app>.conf` in the nginx conf.d path), and in Kubernetes mode the Service of each app and, if network policies are enabled, its NetworkPolicy.
- R3: After a successful peer deletion, no metadata is stored for the peer, neither in the persistent metadata storage (BoltDB) nor in the in-memory cache in front of it.
- R4: The server forgets the deleted peer's last synced app list (`AppStateChangeGenerator.peerApps`). If a peer with the same name pairs again, the apps from its first sync are exposed again.
- R5: A sync from the peer that is already in progress when the deletion starts cannot bring the peer's apps or metadata back after the cleanup.
- R6: Cleanup runs asynchronously. Cleanup failures never change the DELETE response. Each failure is logged, and the remaining cleanup continues.
- R7: Apps, exposure and metadata of other peers, and the server's own locally exposed apps, are unchanged by a peer deletion.
- R8: The DELETE endpoint keeps its path, auth and responses: 204 on success, 500 when the peer is not found or when removing the WireGuard entry or the peer record fails.

## Behaviour

### Successful deletion
1. The caller sends `DELETE /api/peers/v1/<name>` with basic auth, the same as today.
2. The server removes the peer's WireGuard entry and its peer record, in the same order as today in `PeerController.deletePeer` (`pkg/api/peers.go`).
3. Once the peer record is deleted, the server:
   - deletes the peer's metadata (R3);
   - forgets the peer's last synced app list (R4);
   - emits a withdraw event for every app the peer had last synced (R1, R2).

   Withdraw events go through the existing path: `AppStateChangeGenerator` changes channel, then `listeners.Registry.Watch`, then `Exposer.Withdraw`. That path is the same one used today when a peer stops reporting an app. It removes the app from the registry behind `GET /api/apps/v1`, deletes the NetworkPolicy (if enabled) and the Service, and removes the nginx config file.
4. DELETE returns 204. The withdrawals may still be running when the response is sent. A `GET /api/apps/v1` immediately afterwards may briefly still list some of the peer's apps. They disappear once the event loop has processed the withdrawals, normally within seconds.

### Failure
- Peer not found: the response is 500, as today, and no cleanup runs.
- The WireGuard config update fails: the response is 500, as today. The peer record is not removed and no cleanup runs.
- Removing the peer record fails: the response is 500. No cleanup runs, because the peer still exists.
- Metadata deletion fails: the error is logged, the remaining cleanup continues, and the response stays 204.
- Withdrawing an app fails (for example, the k8s API refuses to delete a Service): this is handled as today in `Registry.Watch`. The error is logged and the app is removed from the app list anyway. The other apps of the peer are still withdrawn. Within a single app, the existing `k8sResourceExposer.Withdraw` stops at the first failing resource; for example, a failed NetworkPolicy delete leaves that app's Service and nginx file in place. This per-app behaviour is not changed. Leftovers like these have to be removed by hand (the k8s resources carry the label `wormhole.glothriel.github.com/exposed-by=wormhole`).

### Edge cases
- The peer never synced, or synced zero apps: there is nothing to withdraw. Only metadata deletion runs (if no metadata is stored, that is a no-op), and the response is 204.
- The peer has metadata but no apps, or apps but no metadata: each part of the cleanup runs on its own, and a missing part is not an error.
- The same peer is deleted twice: the second call returns 500 (peer not found), as today, and no cleanup runs.
- A sync from the peer is in progress during the deletion: syncs are handled one at a time in `syncing.Server.Start`. Handling a single sync (peer lookup, metadata `Set`, `UpdateForPeer`) and the peer-deletion steps from step 2 onward must not interleave. A sync handled after the deletion fails the peer lookup and is rejected, as for any unknown peer today (R5).
- The deleted peer keeps running: its syncs can no longer reach the server once the WireGuard entry is gone. Any that do arrive are rejected as from an unknown peer. This is unchanged.
- The peer pairs again under the same name: pairing works as today. Because its last synced app list was forgotten, its first sync produces add events and its apps are exposed again (R4).
- Two peers have apps with the same name: withdrawals match on both peer and app name (`Registry.Watch`, the nginx config file name, and the k8s entity name `<peer>-<app>`), so the other peer's apps are not touched (R7).

## Impact
- Existing data: metadata of peers deleted before this change stays in the BoltDB metadata storage. Kubernetes Services and NetworkPolicies left behind by earlier deletions also stay. Neither is cleaned up by this change (see Out of scope). The nginx config files are already wiped on every server start (`nginx.NewNginxExposer` calls `WithdrawAll`). The app list is in memory and rebuilt on every restart.
- `GET /api/peers/v2`: it reads metadata per listed peer, and the deleted peer is no longer listed. No change.
- `GET /api/apps/v1`: stops listing the deleted peer's apps (R1).
- The sync responses the server sends to other peers list only the server's own apps (`appsExposedHere`), not remote peers' apps. No change.
- In-cluster consumers: the Services `<peer>-<app>` in the server namespace disappear, so DNS for them stops resolving. That is the intended result.
- `MetadataStorage` (`pkg/syncing/metadata.go`) has no delete operation today. All three implementations (in-memory, BoltDB, caching) need one.
- Client mode does not expose the peers API and is not affected.
- Existing e2e test `tests/test_kubernetes.py::test_peer_deletion` covers deleting a peer and the auth rules of the endpoint. It stays valid.

## Feasibility
- Blockers: none
- Expensive parts: none. The withdrawal path, the k8s RBAC (`delete` on services and networkpolicies in `kubernetes/helm/templates/server-rbac.yaml`) and the per-peer app list needed to know what to withdraw already exist. Making syncs and deletions mutually exclusive is cheap because syncs are already processed one at a time.

## Constraints
- Compatibility: the DELETE endpoint's path, basic-auth requirement and response codes are unchanged (R8). No API or flag is added.
- No data migration.
- Concurrency: R5 must hold under the race detector (`go test -race`).
- Performance: one k8s API call per managed resource per app, run in the background. DELETE latency is not a requirement (deletions happen about every 6 months in production; more often in testing).

## Acceptance criteria
- [ ] AC1 (R1): A peer has synced apps `a` and `b`. After the DELETE returns 204 and the asynchronous cleanup has finished, `GET /api/apps/v1` contains no entry with that peer.
- [ ] AC2 (R2): In Kubernetes mode with network policies enabled, after deleting a peer that exposed apps, polling the server namespace shows no Service and no NetworkPolicy named `<peer>-<app>` for any of its apps, and no `remote-<peer>-*.conf` file exists in the nginx conf.d path.
- [ ] AC3 (R3): After deleting a peer that has synced metadata, `Get(<peer>)` on the server's metadata storage returns not-found, for both the BoltDB and the caching storage. A new `MetadataStorage` instance opened on the same BoltDB file also returns not-found.
- [ ] AC4 (R4): Delete a peer that had synced app `a`, then let a peer with the same name pair and sync `a` again without a server restart. `GET /api/apps/v1` then lists `a` for that peer, and its Service exists in Kubernetes mode.
- [ ] AC5 (R5): A test where a sync for peer P and a deletion of P run concurrently. Afterwards, the app list contains no app of P and the metadata storage has no entry for P, under `go test -race`.
- [ ] AC6 (R6): With the exposer's `Withdraw` made to fail for one of the peer's apps, the DELETE still returns 204, the failure is logged, the peer's other apps are withdrawn, and the failed app is not in `GET /api/apps/v1`.
- [ ] AC7 (R6): With metadata deletion made to fail, the DELETE still returns 204, the failure is logged, and the peer's apps are still withdrawn.
- [ ] AC8 (R7): Peers P1 and P2 both have an app named `x`. After deleting P1, `GET /api/apps/v1` still lists P2's `x` and the server's own apps unchanged, P2's metadata is still returned by `GET /api/peers/v2`, and P2's Service and nginx file for `x` still exist.
- [ ] AC9 (R8): DELETE without credentials is rejected as today. DELETE of an unknown peer returns 500. DELETE of a known peer returns 204. `test_peer_deletion` passes unchanged.

## Out of scope
- Removing metadata, Services or NetworkPolicies left over from deletions made before this change.
- Periodic or background reconciliation of orphaned apps or k8s resources.
- Changing how a deleted peer that keeps running behaves, or whether it can pair again.
- Changing how a single app's withdrawal handles a failure part-way through its resources (`k8sResourceExposer.Withdraw` stops at the first error).
- Making DELETE wait for the cleanup or report cleanup failures.

## Decisions
| Question | Decision | Decided by |
|---|---|---|
| How often are peers deleted in practice? | Rarely: about once every 6 months in production, more often during testing or prototyping. It affects priority only. | @glothriel |
| If part of the cleanup fails, does the deletion still succeed? | Yes. Log each failure and continue with the rest of the cleanup. The peer is removed and DELETE returns 204. | @glothriel |
| Must cleanup be finished before DELETE returns? | No. Cleanup is asynchronous, and `GET /api/apps/v1` may list the peer's apps for a moment after the 204. | @glothriel |
| Clear the peer's last synced app list (`AppStateChangeGenerator.peerApps`) on deletion? | Yes. Otherwise a peer that pairs again under the same name never gets its apps exposed until the server restarts. | agent decision |
| Prevent an in-flight sync from re-adding apps or metadata after the cleanup? | Yes. Handling one sync and deleting a peer must not interleave, because the race reproduces exactly the bug this issue fixes, and syncs are already serialized. | agent decision |
| Must DELETE return without waiting for withdrawal events to be queued? | No requirement. Whatever the implementation gives is acceptable, as long as cleanup failures never change the response, because deletions are rare and latency was not asked for. | agent decision |
| When does cleanup start, given that the WireGuard update or the peer record removal can fail? | Only after the peer record has been deleted. On earlier failures the peer still exists, so its apps and metadata are still valid. | agent decision |
| Should a failing resource inside one app's withdrawal stop the rest of that app's withdrawal? | Keep today's behaviour (stop at the first error, log it, drop the app from the list). This code path is shared with normal sync withdrawals, and changing it is a separate fix. | agent decision |

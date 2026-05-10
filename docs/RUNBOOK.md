# Runbook

Operational playbook for `bi8s-go`. Each section: **symptom → diagnose
→ mitigate → fix.** Pair this with the dashboards under
[OBSERVABILITY.md](OBSERVABILITY.md).

## On-call quick links

- **API health**: `https://<host>/v1/livez`, `/v1/readyz`, `/v1/health`.
- **Grafana**: API Overview dashboard.
- **Logs**: Loki query `{app="bi8s-go"} |= "level=error"`.
- **Traces**: Tempo, search by `request_id`.
- **CI**: <https://github.com/xanderbilla/bi8s-go/actions>.

---

## 1. Sudden 5xx spike

**Symptom**: API Overview shows error rate > 1%; alert fires.

**Diagnose**:

1. Filter Loki: `{app="bi8s-go"} |= "level=error" | json` for the last
   15 min. Look for a dominant `error.type` and `path`.
2. Pivot on a representative `request_id` to its trace in Tempo.
3. Check downstream:
   - DynamoDB throttles (`bi8s_dynamodb_calls_total{status="throttled"}`).
   - S3 5xx.
   - Encoder failures.

**Mitigate**:

- If it's a single bad route, lower its rate-limit cap or block at NGINX.
- If it's a downstream dependency, see sections 4–6.
- If it correlates with a deploy: **rollback** (section 9).

**Fix**: ship a patch + add a regression test.

---

## 2. High latency (p95 > 1.5 s)

**Diagnose**:

1. Grafana → API Overview → "Latency by route" panel.
2. If a single route is slow, open Tempo and inspect spans — usually
   one DynamoDB call dominates.
3. Check `bi8s_dynamodb_calls_total` histogram for that table.

**Mitigate**:

- Reduce `DYNAMODB_MAX_SCAN_PAGES` if a `Scan` is the culprit.
- Bump capacity (Tofu) or switch table to on-demand temporarily.

**Fix**: convert hot `Scan` to `Query` against a GSI; add caching.

---

## 3. Rate-limit storm

**Symptom**: 429 rate spikes; legitimate users blocked.

**Diagnose**:

- Loki: `{app="bi8s-go"} |= "rate limit"` and group by client IP
  (`json | client_ip != ""`).
- If a single IP dominates → abuse; block at NGINX.
- If broad → genuine traffic surge.

**Mitigate**:

- Raise `RATE_LIMIT_REQUESTS_PER_SECOND` and `RATE_LIMIT_BURST` and
  redeploy (env-var change only — fast).
- Or scale horizontally (more EC2 instances behind ALB).

---

## 4. Redis outage

**Symptom**: rate limiting degrades but **the app stays up**
(`RATE_LIMIT_BACKEND=redis` is **fail-open**).

**Diagnose**: `{app="bi8s-go"} |= "redis"` in Loki — look for
`circuit-open` or connection refused.

**Mitigate**: nothing required — service continues without rate limits.
Optionally set `RATE_LIMIT_BACKEND=memory` to remove the noisy logs.

**Fix**: restore Redis; the client auto-reconnects.

---

## 5. DynamoDB throttling

**Symptom**: 5xx on writes; metrics show `status="throttled"`.

**Mitigate**:

- Switch the affected table to **on-demand** via Tofu (instant via
  console for a true emergency).
- Reduce write batch sizes in code if obvious.

**Fix**: tune autoscaling target (default 70% utilization) or stay on
on-demand for prod.

---

## 6. S3 access errors

**Symptom**: image/video loads fail; encoder uploads error.

**Diagnose**: Loki for `AccessDenied` / `NoSuchKey` / `SlowDown`.

**Mitigate**:

- `AccessDenied`: instance role drift — re-apply Tofu.
- `SlowDown`: introduce client retries with jitter (already enabled in
  the SDK; verify no custom retry overrides).
- `NoSuchKey`: data inconsistency — find the orphan in DynamoDB and
  reconcile.

---

## 7. Encoder backlog

**Symptom**: `bi8s_encoder_jobs_total{status="queued"}` rises;
`/v1/a/encoder` jobs stay `queued` for minutes.

**Diagnose**:

- CPU exhaustion on the EC2 host (encoder is in-process by default).
- ffmpeg failure loop — check `error` column in `bi8s-video-table-*`.

**Mitigate**:

- Externalize: set `ENCODER_QUEUE_URL` to an SQS queue and run a
  dedicated worker pool on a larger instance.
- Pause new submissions by gating `/v1/a/encoder` at NGINX.

**Fix**: scale workers; investigate ffmpeg errors per content id.

---

## 8. Telemetry pipeline outage

**Symptom**: Grafana panels show "No data".

**Diagnose order**:

1. `docker ps` on the host — is `otel-collector` up? `tempo`? `loki`?
2. `docker logs otel-collector` for export errors (S3 perms, DNS).
3. Check `STORAGE_BASE_URL`/S3 permissions for Loki/Tempo.

**Mitigate**: app traffic is unaffected by telemetry outages; the OTel
SDK drops on backpressure rather than blocking requests.

**Fix**: restart the collector; if persistent, redeploy the
observability stack (`docker compose -f docker-compose.local.yml up -d`).

---

## 9. Rollback

**Pre-conditions**: bad release identified, last-good tag known.

```bash
# Update the prod var to the previous tag.
cd infra/tofu/envs/prod
tofu plan  -var="app_image_tag=v0.1.7"
tofu apply -var="app_image_tag=v0.1.7"
```

The EC2 user-data script pulls the requested tag and rolls the
container behind NGINX. Roll forward (instead of rollback) for
data-shape regressions that already wrote new-shape items.

---

## 10. Restore from PITR (DynamoDB)

```bash
aws dynamodb restore-table-to-point-in-time \
  --source-table-name bi8s-content-table-prod \
  --target-table-name bi8s-content-table-prod-restore \
  --restore-date-time 2025-05-04T03:00:00Z
```

After verification, swap the env var (`DYNAMODB_CONTENT_TABLE`) to the
restored table and redeploy. Drop the old table once stable.

---

## 11. Restore an S3 object

Versioning is enabled. Restore the previous version:

```bash
aws s3api list-object-versions \
  --bucket bi8s-storage-prod --prefix content/<id>/poster.jpg
aws s3api copy-object \
  --bucket bi8s-storage-prod \
  --copy-source "bi8s-storage-prod/content/<id>/poster.jpg?versionId=<v>" \
  --key content/<id>/poster.jpg
```

---

## 12. Routine ops

| Task                 | Cadence     | How                                                    |
| -------------------- | ----------- | ------------------------------------------------------ |
| Renew TLS            | auto / 60 d | `infra/scripts/setup-ssl-letsencrypt.sh` (cron).       |
| Rotate Grafana admin | first boot  | UI.                                                    |
| Review CVE alerts    | weekly      | Dependabot PRs + `govulncheck` CI.                     |
| Verify backups       | monthly     | Spot-restore a DynamoDB PITR snapshot to a temp table. |
| Capacity review      | monthly     | Check Grafana "API Overview" + DynamoDB metrics.       |

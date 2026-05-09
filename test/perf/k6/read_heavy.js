// k6 read-heavy load profile for the bi8s API.
//
// Mirrors a steady-state browse workload: livez probe + content discovery +
// catalog reads with ~80 % read traffic. Run with:
//
//   BASE_URL=https://api.example.com k6 run test/perf/k6/read_heavy.js
//
// SLO targets enforced as thresholds:
//   - p95 latency < 500 ms (excluding /livez which we treat as a probe)
//   - error rate < 1 %
//
// All requests use idempotent GETs only; safe to run against any environment
// whose data is non-production-sensitive.

import http from "k6/http";
import { check, sleep, group } from "k6";
import { Rate, Trend } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

const errorRate = new Rate("errors");
const apiLatency = new Trend("api_latency_ms", true);

export const options = {
  scenarios: {
    steady_browse: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "30s", target: 10 },
        { duration: "2m", target: 20 },
        { duration: "30s", target: 0 },
      ],
      gracefulRampDown: "15s",
    },
  },
  thresholds: {
    "http_req_duration{kind:api}": ["p(95)<500"],
    errors: ["rate<0.01"],
    http_req_failed: ["rate<0.01"],
  },
};

function getJSON(path, name) {
  const res = http.get(`${BASE_URL}${path}`, {
    tags: { kind: "api", name: name || path },
    headers: { Accept: "application/json" },
  });
  apiLatency.add(res.timings.duration);
  const ok = check(res, {
    "status is 2xx or 4xx": (r) => r.status >= 200 && r.status < 500,
    "has body": (r) => r.body && r.body.length > 0,
  });
  errorRate.add(!ok);
  return res;
}

export default function () {
  group("health", () => {
    http.get(`${BASE_URL}/v1/livez`, { tags: { kind: "probe" } });
  });

  group("discover", () => {
    getJSON("/v1/c/discover", "discover");
    getJSON("/v1/c/banner", "banner");
    getJSON("/v1/c/attributes", "attributes");
  });

  group("search", () => {
    getJSON("/v1/c/search?q=action&pageSize=20", "search");
  });

  sleep(Math.random() * 2 + 0.5);
}

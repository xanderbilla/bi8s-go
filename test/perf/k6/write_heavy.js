// k6 write-heavy load profile for the bi8s API.
//
// Exercises the admin write surface (validation/error envelope only — no
// real S3/DynamoDB mutations): empty multipart POSTs and missing-body POSTs
// are expected to return 4xx, which is the contract under test.
//
//   BASE_URL=https://api.example.com k6 run test/perf/k6/write_heavy.js
//
// SLO targets:
//   - p95 latency < 800 ms (writes traverse multipart parser + validators)
//   - error-envelope-shape failures < 1 % (we validate JSON envelope, not 2xx)
//
// Safe to run against dev/preview only — DO NOT point at prod admin paths.

import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

const envelopeFailRate = new Rate("envelope_failures");
const writeLatency = new Trend("write_latency_ms", true);

export const options = {
  scenarios: {
    write_burst: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "30s", target: 5 },
        { duration: "1m", target: 15 },
        { duration: "30s", target: 0 },
      ],
      gracefulRampDown: "15s",
    },
  },
  thresholds: {
    "http_req_duration{kind:write}": ["p(95)<800"],
    envelope_failures: ["rate<0.01"],
  },
};

function postEmptyMultipart(path, name) {
  const params = {
    headers: { "Content-Type": "multipart/form-data; boundary=----bi8s-perf" },
    tags: { kind: "write", name: name || path },
  };
  const res = http.post(`${BASE_URL}${path}`, "", params);
  writeLatency.add(res.timings.duration);

  const ok = check(res, {
    "status 4xx (validation rejected)": (r) =>
      r.status >= 400 && r.status < 500,
    "json error envelope": (r) => {
      try {
        const body = r.json();
        return body && body.success === false && body.error && body.error.code;
      } catch (_) {
        return false;
      }
    },
  });
  envelopeFailRate.add(!ok);
  return res;
}

export default function () {
  postEmptyMultipart("/v1/a/content/", "create_content");
  postEmptyMultipart("/v1/a/people/", "create_people");
  postEmptyMultipart("/v1/a/attributes/", "create_attribute");
  sleep(Math.random() * 1 + 0.25);
}

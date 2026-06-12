// k6-physicalhost.js — load test for the physical-hosts
// HTTP surface. Mirrors tests/load/loadgen_test.go's
// SLO contract: 50 VUs / 30s default, p95 < 500ms, error
// rate < 1%. The two implementations MUST stay in sync;
// loadgen_test.go is the executable copy, this file is
// the contract for environments where k6 is the canonical
// load-tester.
//
// Targets:
//   - GET /api/v1/physical-hosts (List, the page-level read)
//   - GET /api/v1/physical-hosts/:id (Get)
//   - GET /api/v1/physical-hosts/:id/metrics (Metrics)
//
// Required env:
//   BASE_URL — server root, e.g. http://localhost:3000
//   TOKEN    — bearer token (dev bypass is fine in dev)
//
// Run:
//   k6 run --vus 50 --duration 30s tests/load/k6-physicalhost.js
//
// The script is intentionally minimal — k6's default
// options.http_req_duration and http_req_failed cover the
// two SLO thresholds.

import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';
const TOKEN = __ENV.TOKEN || 'dev-bypass';

export const options = {
  thresholds: {
    // SLO contract. A regression fails the run.
    http_req_duration: ['p(95)<500'],
    http_req_failed:   ['rate<0.01'],
  },
};

function authHeader() {
  return { headers: { Authorization: `Bearer ${TOKEN}` } };
}

// List the hosts once at the start of the test so the
// per-VU iteration can pick a host :id at random. The
// per-host Get + Metrics endpoints are the latency
// surface; the List call is the "page" surface.
const listResp = http.get(`${BASE_URL}/api/v1/physical-hosts`, authHeader());
check(listResp, { 'list 200': (r) => r.status === 200 });
const hostIDs = (listResp.json('data') || []).map((h) => h.id).filter(Boolean);

export default function () {
  // List (page-level read). Every VU does this.
  const list = http.get(`${BASE_URL}/api/v1/physical-hosts`, authHeader());
  check(list, { 'list status 200': (r) => r.status === 200 });

  if (hostIDs.length === 0) {
    return; // no hosts seeded; the List check above is the only thing
  }
  // Get + Metrics on a random host. 2 endpoint hits
  // per VU iteration; List is a 3rd.
  const id = hostIDs[Math.floor(Math.random() * hostIDs.length)];
  const get = http.get(`${BASE_URL}/api/v1/physical-hosts/${id}`, authHeader());
  check(get, { 'get status 200': (r) => r.status === 200 });

  const met = http.get(
    `${BASE_URL}/api/v1/physical-hosts/${id}/metrics`,
    authHeader(),
  );
  check(met, { 'metrics status 200': (r) => r.status === 200 });
}

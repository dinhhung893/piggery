#!/usr/bin/env python3
"""flaky.py <port> <fail count>: answers 503 to the first N POSTs, then forwards to the real endpoint."""
import http.server, json, sys, urllib.request, urllib.error
PORT, FAILS = int(sys.argv[1]), int(sys.argv[2])
UP = json.load(open("/tmp/pg-omp/upstream.json"))["baseUrl"].rstrip("/")
n = {"posts": 0}

class H(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("content-length", 0)))
        n["posts"] += 1
        if n["posts"] <= FAILS:
            out = json.dumps({"error": {"message": "Service overloaded, please retry", "type": "overloaded_error"}}).encode()
            self.send_response(503); self.send_header("content-type", "application/json")
            self.send_header("content-length", str(len(out))); self.end_headers(); self.wfile.write(out)
            open("/tmp/pg-omp/flaky.log", "a").write(f"post {n['posts']}: 503\n"); return
        path = self.path[len("/v1"):] if self.path.startswith("/v1") else self.path
        req = urllib.request.Request(UP + path, data=body, method="POST",
                                     headers={**{k: v for k, v in self.headers.items() if k.lower() not in ("host", "content-length", "accept-encoding")}, "Accept-Encoding": "identity"})
        try:
            r = urllib.request.urlopen(req, timeout=120)
        except urllib.error.HTTPError as e:
            r = e
        self.send_response(r.status)
        for k, v in r.headers.items():
            if k.lower() not in ("transfer-encoding", "connection", "content-length", "content-encoding"): self.send_header(k, v)
        self.send_header("connection", "close"); self.end_headers()
        while True:
            chunk = r.read(4096)
            if not chunk: break
            self.wfile.write(chunk); self.wfile.flush()
        open("/tmp/pg-omp/flaky.log", "a").write(f"post {n['posts']}: forwarded {r.status}\n")

http.server.ThreadingHTTPServer(("127.0.0.1", PORT), H).serve_forever()

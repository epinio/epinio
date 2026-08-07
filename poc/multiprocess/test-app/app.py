import http.server
import os
import sys
import time


version = os.environ.get("IMAGE_VERSION", "unknown")
process = sys.argv[1] if len(sys.argv) > 1 else "web"

if process == "web":
    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            body = f"web version={version} process={os.environ.get('EPINIO_PROCESS')}\n".encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, message, *args):
            print(f"web version={version}: {message % args}", flush=True)

    print(f"web starting version={version}", flush=True)
    http.server.ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
elif process == "worker":
    print(f"worker starting version={version}", flush=True)
    while True:
        print(f"worker heartbeat version={version}", flush=True)
        time.sleep(5)
elif process == "cron":
    print(f"cron ran version={version}", flush=True)
elif process == "migrate":
    should_fail = len(sys.argv) > 2 and sys.argv[2] == "fail"
    print(f"migration version={version} fail={should_fail}", flush=True)
    sys.exit(42 if should_fail else 0)
elif process == "crash":
    print(f"deliberate process failure version={version}", flush=True)
    sys.exit(23)
else:
    raise SystemExit(f"unknown process {process}")
